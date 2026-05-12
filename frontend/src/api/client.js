import axios from 'axios';
import { message } from 'antd';
import { useAuthStore } from '@/stores/auth';
const client = axios.create({
    baseURL: '/api/v1',
    timeout: 30000,
});
client.interceptors.request.use((c) => {
    const token = useAuthStore.getState().accessToken;
    if (token)
        c.headers.set('Authorization', `Bearer ${token}`);
    return c;
});
client.interceptors.response.use((resp) => {
    if (resp.data?.code !== 0) {
        message.error(resp.data?.message || '请求失败');
        return Promise.reject(resp.data);
    }
    return resp;
}, (err) => {
    if (err.response?.status === 401) {
        useAuthStore.getState().logout();
        window.location.href = '/login';
    }
    else {
        message.error(err.response?.data?.message || err.message);
    }
    return Promise.reject(err);
});
export async function apiGet(url, params) {
    const r = await client.get(url, { params });
    return r.data.data;
}
export async function apiPost(url, body) {
    const r = await client.post(url, body);
    return r.data.data;
}
export async function apiPut(url, body) {
    const r = await client.put(url, body);
    return r.data.data;
}
export async function apiDelete(url) {
    const r = await client.delete(url);
    return r.data.data;
}
export default client;
