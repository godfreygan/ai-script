import axios, {
  AxiosHeaders,
  type AxiosResponse,
  type InternalAxiosRequestConfig,
} from 'axios';
import { useAuthStore } from '@/stores/auth';
import { ApiError, appendTraceHint, mapApiErrorMessage } from './error';
import { createTraceId } from './trace';

export interface Envelope<T = unknown> {
  code: number;
  message: string;
  data: T;
  error?: string;
  request_id?: string;
  trace_id?: string;
}

export interface Page<T> {
  list: T[];
  page: number;
  page_size: number;
  total: number;
}

declare module 'axios' {
  interface AxiosRequestConfig {
    /** 为 true 时不弹出全局错误 toast（由调用方自行处理） */
    skipErrorToast?: boolean;
  }
}

const client = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
});

// === 消息注入: 由 main.tsx 在 <AntApp> 内部注入,解决 antd5 静态 message 上下文丢失 ===
let _messageInstance: any = null;
export function setMessageInstance(msg: any) {
  _messageInstance = msg;
}

function safeToast(type: 'error' | 'success' | 'warning', content: string) {
  if (_messageInstance?.[type]) {
    _messageInstance[type](content);
  } else {
    // eslint-disable-next-line no-console
    console.warn(`[API ${type}]`, content);
  }
}

// === 401 refresh token 自动刷新(队列锁防止并发雪崩) ===
let _refreshPromise: Promise<boolean> | null = null;

async function tryRefreshToken(): Promise<boolean> {
  const refreshToken = useAuthStore.getState().refreshToken;
  if (!refreshToken) return false;
  try {
    const resp = await client.post<Envelope<{ access_token: string; refresh_token: string; user: unknown }>>(
      '/auth/refresh',
      { refresh_token: refreshToken },
      { skipErrorToast: true },
    );
    if (resp.data.code === 0) {
      const { access_token, refresh_token, user } = resp.data.data as any;
      useAuthStore.getState().login({
        accessToken: access_token,
        refreshToken: refresh_token,
        user,
      });
      return true;
    }
  } catch {
    // refresh 失败,继续向下执行 logout
  }
  return false;
}

function applyRequestHeaders(config: InternalAxiosRequestConfig): void {
  const token = useAuthStore.getState().accessToken;
  const headers = AxiosHeaders.from(config.headers ?? {});
  headers.set('X-Trace-Id', createTraceId());
  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }
  config.headers = headers;
}

function isEnvelope(payload: unknown): payload is Envelope<unknown> {
  return (
    typeof payload === 'object' &&
    payload !== null &&
    'code' in payload &&
    typeof (payload as Envelope).code === 'number'
  );
}

client.interceptors.request.use((config) => {
  try {
    applyRequestHeaders(config);
  } catch (e) {
    // eslint-disable-next-line no-console
    console.error('applyRequestHeaders failed', e);
  }
  return config;
});

client.interceptors.response.use(
  (resp: AxiosResponse<Envelope<unknown>>) => {
    const payload = resp.data;
    if (!isEnvelope(payload)) {
      if (!resp.config.skipErrorToast) {
        safeToast('error', mapApiErrorMessage(undefined, resp.status));
      }
      return Promise.reject(
        new ApiError(mapApiErrorMessage(undefined, resp.status), {
          status: resp.status,
          handled: !resp.config.skipErrorToast,
        }),
      );
    }
    if (payload.code !== 0) {
      const displayMessage = appendTraceHint(
        mapApiErrorMessage(payload, resp.status),
        payload,
      );
      if (!resp.config.skipErrorToast) {
        safeToast('error', displayMessage);
      }
      return Promise.reject(
        new ApiError(displayMessage, {
          ...payload,
          status: resp.status,
          handled: !resp.config.skipErrorToast,
        }),
      );
    }
    return resp;
  },
  async (err) => {
    const skipToast = err.config?.skipErrorToast === true;
    const payload = err.response?.data as Envelope<unknown> | undefined;
    const status = err.response?.status as number | undefined;
    const displayMessage = appendTraceHint(
      mapApiErrorMessage(isEnvelope(payload) ? payload : undefined, status),
      isEnvelope(payload) ? payload : undefined,
    );

    if (status === 401) {
      if (!_refreshPromise) {
        _refreshPromise = tryRefreshToken().finally(() => {
          _refreshPromise = null;
        });
      }
      const refreshed = await _refreshPromise;
      if (refreshed && err.config) {
        // token 刷新成功,重试原请求
        return client.request(err.config);
      }
      // refresh 失败:延迟跳转,给并发请求一个排队等待 refresh 的窗口
      safeToast('error', '登录已过期,请重新登录');
      setTimeout(() => {
        useAuthStore.getState().logout();
        window.location.href = '/login';
      }, 50);
      return Promise.reject(
        new ApiError('登录已过期', {
          ...(isEnvelope(payload) ? payload : {}),
          status,
          handled: true,
        }),
      );
    }

    if (!skipToast) {
      safeToast('error', displayMessage);
    }
    return Promise.reject(
      new ApiError(displayMessage, {
        ...(isEnvelope(payload) ? payload : {}),
        status,
        handled: skipToast,
      }),
    );
  },
);

export function escapeHtml(input: string): string {
  return input
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#x27;');
}

export async function apiGet<T>(
  url: string,
  params?: Record<string, unknown>,
  config?: Parameters<typeof client.get>[1],
) {
  const r = await client.get<Envelope<T>>(url, { params, ...(config ?? {}) });
  return r.data.data;
}

export async function apiPost<T>(
  url: string,
  body?: unknown,
  config?: Parameters<typeof client.post>[2],
) {
  const r = await client.post<Envelope<T>>(url, body, config);
  return r.data.data;
}

export async function apiPut<T>(
  url: string,
  body?: unknown,
  config?: Parameters<typeof client.put>[2],
) {
  const r = await client.put<Envelope<T>>(url, body, config);
  return r.data.data;
}

export async function apiDelete<T>(url: string, config?: Parameters<typeof client.delete>[1]) {
  const r = await client.delete<Envelope<T>>(url, config);
  return r.data.data;
}

export default client;
