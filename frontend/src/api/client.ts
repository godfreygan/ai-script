import axios, { AxiosInstance, AxiosResponse } from 'axios';
import { message } from 'antd';
import { useAuthStore } from '@/stores/auth';
import { ApiError, appendTraceHint, mapApiErrorMessage } from './error';
import { getTraceId } from './trace';

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

const client: AxiosInstance = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
});

client.interceptors.request.use((c) => {
  const token = useAuthStore.getState().accessToken;
  const traceId = getTraceId();
  c.headers.set('X-Trace-Id', traceId);
  c.headers.set('trace_id', traceId);
  if (token) {
    c.headers.set('Authorization', `Bearer ${token}`);
  }
  return c;
});

client.interceptors.response.use(
  (resp: AxiosResponse<Envelope<unknown>>) => {
    if (resp.data?.code !== 0) {
      const displayMessage = appendTraceHint(mapApiErrorMessage(resp.data, resp.status), resp.data);
      message.error(displayMessage);
      return Promise.reject(
        new ApiError(displayMessage, {
          ...resp.data,
          status: resp.status,
          handled: true,
        }),
      );
    }
    return resp;
  },
  (err) => {
    const payload = err.response?.data as Envelope<unknown> | undefined;
    const status = err.response?.status as number | undefined;
    const displayMessage = appendTraceHint(mapApiErrorMessage(payload, status), payload);

    if (status === 401) {
      useAuthStore.getState().logout();
      window.location.href = '/login';
      return Promise.reject(
        new ApiError(displayMessage, {
          ...payload,
          status,
          handled: true,
        }),
      );
    }

    message.error(displayMessage);
    return Promise.reject(
      new ApiError(displayMessage, {
        ...payload,
        status,
        handled: true,
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

export async function apiGet<T>(url: string, params?: Record<string, unknown>) {
  const r = await client.get<Envelope<T>>(url, { params });
  return r.data.data;
}

export async function apiPost<T>(url: string, body?: unknown) {
  const r = await client.post<Envelope<T>>(url, body);
  return r.data.data;
}

export async function apiPut<T>(url: string, body?: unknown) {
  const r = await client.put<Envelope<T>>(url, body);
  return r.data.data;
}

export async function apiDelete<T>(url: string) {
  const r = await client.delete<Envelope<T>>(url);
  return r.data.data;
}

export default client;
