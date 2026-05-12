import axios, { AxiosInstance, AxiosResponse } from 'axios';
import { message } from 'antd';
import { useAuthStore } from '@/stores/auth';

export interface Envelope<T = unknown> {
  code: number;
  message: string;
  data: T;
  request_id?: string;
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
  if (token) c.headers.set('Authorization', `Bearer ${token}`);
  return c;
});

client.interceptors.response.use(
  (resp: AxiosResponse<Envelope<unknown>>) => {
    if (resp.data?.code !== 0) {
      message.error(resp.data?.message || '请求失败');
      return Promise.reject(resp.data);
    }
    return resp;
  },
  (err) => {
    if (err.response?.status === 401) {
      useAuthStore.getState().logout();
      window.location.href = '/login';
    } else {
      message.error(err.response?.data?.message || err.message);
    }
    return Promise.reject(err);
  },
);

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
