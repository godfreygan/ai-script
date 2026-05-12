import { apiDelete, apiGet, apiPost, apiPut } from '../client';
export const authApi = {
    login: (username, password) => apiPost('/auth/login', { username, password }),
    logout: () => apiPost('/auth/logout'),
    refresh: (refresh_token) => apiPost('/auth/refresh', { refresh_token }),
};
export const userApi = {
    me: () => apiGet('/users/me'),
    changePassword: (old_password, new_password) => apiPost('/users/me/password', { old_password, new_password }),
    list: (params) => apiGet('/users', params),
    get: (id) => apiGet(`/users/${id}`),
    create: (data) => apiPost('/users', data),
    update: (id, data) => apiPut(`/users/${id}`, data),
    delete: (id) => apiDelete(`/users/${id}`),
    resetPassword: (id, new_password) => apiPost(`/users/${id}/reset_password`, { new_password }),
};
export const deptApi = {
    list: () => apiGet('/depts'),
    get: (id) => apiGet(`/depts/${id}`),
    create: (data) => apiPost('/depts', data),
    update: (id, data) => apiPut(`/depts/${id}`, data),
    delete: (id) => apiDelete(`/depts/${id}`),
};
export const roleApi = {
    list: () => apiGet('/roles'),
    get: (id) => apiGet(`/roles/${id}`),
    listPermissions: () => apiGet('/permissions'),
    create: (data) => apiPost('/roles', data),
    update: (id, data) => apiPut(`/roles/${id}`, data),
    delete: (id) => apiDelete(`/roles/${id}`),
};
export const projectApi = {
    list: (params) => apiGet('/projects', params),
    get: (id) => apiGet(`/projects/${id}`),
    create: (data) => apiPost('/projects', data),
    update: (id, data) => apiPut(`/projects/${id}`, data),
    delete: (id) => apiDelete(`/projects/${id}`),
    listMembers: (id) => apiGet(`/projects/${id}/members`),
    addMember: (id, user_id, role_in_project = 'editor') => apiPost(`/projects/${id}/members`, { user_id, role_in_project }),
    removeMember: (id, uid) => apiDelete(`/projects/${id}/members/${uid}`),
};
export const modelApi = {
    list: (params) => apiGet('/models', params),
    get: (id) => apiGet(`/models/${id}`),
    create: (data) => apiPost('/models', data),
    update: (id, data) => apiPut(`/models/${id}`, data),
    delete: (id) => apiDelete(`/models/${id}`),
    healthcheck: (id) => apiPost(`/models/${id}/healthcheck`),
};
export const scriptApi = {
    list: (params) => apiGet('/scripts', params),
    get: (id) => apiGet(`/scripts/${id}`),
    create: (data) => apiPost('/scripts', data),
    delete: (id) => apiDelete(`/scripts/${id}`),
    episodes: (id) => apiGet(`/scripts/${id}/episodes`),
    split: (id, data) => apiPost(`/scripts/${id}/split`, data),
};
export const promptApi = {
    listByEpisode: (id) => apiGet(`/episodes/${id}/prompts`),
    getCurrent: (id) => apiGet(`/episodes/${id}/prompts/current`),
    generate: (id, data) => apiPost(`/episodes/${id}/prompts/generate`, data),
    setCurrent: (id, episode_id) => apiPost(`/prompts/${id}/set_current`, { episode_id }),
};
export const storyboardApi = {
    listByEpisode: (id) => apiGet(`/episodes/${id}/storyboards`),
    generate: (id, data) => apiPost(`/episodes/${id}/storyboards/generate`, data),
    bulkSave: (id, shots) => apiPost(`/episodes/${id}/storyboards/bulk_save`, { shots }),
    get: (id) => apiGet(`/storyboards/${id}`),
    update: (id, data) => apiPut(`/storyboards/${id}`, data),
    delete: (id) => apiDelete(`/storyboards/${id}`),
    applyStyle: (id, style_id) => apiPost(`/storyboards/${id}/apply_style`, { style_id }),
};
export const styleApi = {
    list: (project_id) => apiGet('/styles', project_id ? { project_id } : undefined),
    get: (id) => apiGet(`/styles/${id}`),
    create: (data) => apiPost('/styles', data),
    update: (id, data) => apiPut(`/styles/${id}`, data),
    delete: (id) => apiDelete(`/styles/${id}`),
};
export const imageApi = {
    list: (params) => apiGet('/images', params),
    get: (id) => apiGet(`/images/${id}`),
    generate: (data) => apiPost('/images/generate', data),
    delete: (id) => apiDelete(`/images/${id}`),
};
export const shortVideoApi = {
    list: (params) => apiGet('/short_videos', params),
    get: (id) => apiGet(`/short_videos/${id}`),
    generate: (data) => apiPost('/short_videos/generate', data),
    delete: (id) => apiDelete(`/short_videos/${id}`),
};
export const uploadApi = {
    /** 上传文件;namespace 必填,后端只允许 images/videos/audios/styles/covers/scripts/misc */
    upload: async (namespace, file) => {
        const fd = new FormData();
        fd.append('namespace', namespace);
        fd.append('file', file);
        const resp = await fetch('/api/v1/files/upload', {
            method: 'POST',
            headers: { Authorization: `Bearer ${localStorage.getItem('token') ?? ''}` },
            body: fd,
        });
        const body = await resp.json();
        if (body.code !== 0) {
            throw new Error(body.msg || 'upload failed');
        }
        return body.data;
    },
};
export const invocationApi = {
    list: (params) => apiGet('/invocations', params),
    stats: (params) => apiGet('/invocations/stats', params),
};
export const pipelineApi = {
    list: () => apiGet('/pipelines'),
    run: (id, body) => apiPost(`/pipelines/${id}/run`, body),
};
