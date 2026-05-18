import axios, {
  AxiosHeaders,
  type AxiosResponse,
  type InternalAxiosRequestConfig,
} from 'axios';
import { message } from 'antd';
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
  } catch {
    // 请求头写入失败不应阻断业务请求
  }
  return config;
});

client.interceptors.response.use(
  (resp: AxiosResponse<Envelope<unknown>>) => {
    const payload = resp.data;
    if (!isEnvelope(payload)) {
      if (!resp.config.skipErrorToast) {
        message.error(mapApiErrorMessage(undefined, resp.status));
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
        message.error(displayMessage);
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
  (err) => {
    const skipToast = err.config?.skipErrorToast === true;
    const payload = err.response?.data as Envelope<unknown> | undefined;
    const status = err.response?.status as number | undefined;
    const displayMessage = appendTraceHint(
      mapApiErrorMessage(isEnvelope(payload) ? payload : undefined, status),
      isEnvelope(payload) ? payload : undefined,
    );

    if (status === 401) {
      useAuthStore.getState().logout();
      window.location.href = '/login';
      return Promise.reject(
        new ApiError(displayMessage, {
          ...(isEnvelope(payload) ? payload : {}),
          status,
          handled: true,
        }),
      );
    }

    if (!skipToast) {
      message.error(displayMessage);
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
  config?: Parameters<typeof client.get>[2],
) {
  const r = await client.get<Envelope<T>>(url, { params, ...config });
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
