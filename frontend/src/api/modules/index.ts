import { apiDelete, apiGet, apiPost, apiPut, Page } from '../client';
import { ApiError, appendTraceHint, mapApiErrorMessage } from '../error';
import { createTraceId } from '../trace';
import { useAuthStore } from '@/stores/auth';

// =================== 通用 ===================

/** 通用状态 (0=禁用, 1=启用) */
export type CommonStatus = 0 | 1;

/** 模型类型 */
export type ModelType = 'text' | 'image' | 'video' | 'audio';

/** 资源状态 (短/完整视频、审核等) */
export type ResourceStatus = 'draft' | 'queued' | 'running' | 'succeeded' | 'done' | 'failed' | 'pending' | 'approved' | 'rejected' | 'cancelled';

export interface BaseModel {
  id: number;
  created_at: string;
  updated_at: string;
}

// =================== Auth ===================

export interface User {
  id: number;
  username: string;
  nickname: string;
  email: string;
  phone: string;
  dept_id: number;
  status: number;
  last_login_at?: string;
  created_at?: string;
}

export interface LoginResult {
  access_token: string;
  refresh_token: string;
  expires_in: number;
  user: User;
  roles: string[];
}

export const authApi = {
  login: (username: string, password: string) =>
    apiPost<LoginResult>('/auth/login', { username, password }),
  logout: () => apiPost('/auth/logout'),
  refresh: (refresh_token: string) =>
    apiPost<LoginResult>('/auth/refresh', { refresh_token }),
};

// =================== User ===================

export interface UserWithRoles extends User {
  role_ids: number[];
}

export const userApi = {
  me: () => apiGet<User>('/users/me'),
  changePassword: (old_password: string, new_password: string) =>
    apiPost('/users/me/password', { old_password, new_password }),

  list: (params?: { page?: number; page_size?: number; q?: string; dept_id?: number; status?: number }) =>
    apiGet<Page<User>>('/users', params),
  get: (id: number) => apiGet<UserWithRoles>(`/users/${id}`),
  create: (data: {
    username: string;
    password: string;
    nickname?: string;
    email?: string;
    phone?: string;
    dept_id?: number;
    role_ids?: number[];
    status?: number;
  }) => apiPost<User>('/users', data),
  update: (
    id: number,
    data: {
      nickname?: string;
      email?: string;
      phone?: string;
      dept_id?: number;
      status?: number;
      role_ids?: number[];
    },
  ) => apiPut<User>(`/users/${id}`, data),
  delete: (id: number) => apiDelete(`/users/${id}`),
  resetPassword: (id: number, new_password: string) =>
    apiPost(`/users/${id}/reset_password`, { new_password }),
};

// =================== Dept ===================

export interface Department {
  id: number;
  name: string;
  parent_id: number;
  path: string;
  sort: number;
  status: number;
}

export const deptApi = {
  list: () => apiGet<Department[]>('/depts'),
  get: (id: number) => apiGet<Department>(`/depts/${id}`),
  create: (data: { name: string; parent_id?: number; sort?: number }) =>
    apiPost<Department>('/depts', data),
  update: (id: number, data: { name?: string; sort?: number; status?: number }) =>
    apiPut<Department>(`/depts/${id}`, data),
  delete: (id: number) => apiDelete(`/depts/${id}`),
};

// =================== Role ===================

export interface Role {
  id: number;
  code: string;
  name: string;
  description: string;
  data_scope: string;
  is_system: number;
  status: number;
}

export interface Permission {
  id: number;
  code: string;
  name: string;
  resource: string;
  action: string;
}

export interface RoleWithPermissions extends Role {
  permissions: string[];
}

export const roleApi = {
  list: () => apiGet<Role[]>('/roles'),
  get: (id: number) => apiGet<RoleWithPermissions>(`/roles/${id}`),
  listPermissions: () => apiGet<Permission[]>('/permissions'),
  create: (data: {
    code: string;
    name: string;
    description?: string;
    data_scope?: string;
    permissions?: string[];
  }) => apiPost<Role>('/roles', data),
  update: (
    id: number,
    data: {
      name?: string;
      description?: string;
      data_scope?: string;
      status?: number;
      permissions?: string[];
    },
  ) => apiPut<Role>(`/roles/${id}`, data),
  delete: (id: number) => apiDelete(`/roles/${id}`),
};

// =================== Project ===================

export interface Project {
  id: number;
  code: string;
  name: string;
  description: string;
  status: number;
  owner_id: number;
  dept_id: number;
  default_pipeline_id: number;
  cover_url: string;
  created_at: string;
  updated_at: string;
}

export interface ProjectMember {
  id: number;
  project_id: number;
  user_id: number;
  role_in_project: string;
}

export const projectApi = {
  list: (params?: { page?: number; page_size?: number; status?: number; q?: string }) =>
    apiGet<Page<Project>>('/projects', params),
  get: (id: number) => apiGet<Project>(`/projects/${id}`),
  create: (data: {
    code: string;
    name: string;
    description?: string;
    dept_id?: number;
    default_pipeline_id?: number;
    cover_url?: string;
  }) => apiPost<Project>('/projects', data),
  update: (
    id: number,
    data: {
      name?: string;
      description?: string;
      status?: number;
      default_pipeline_id?: number;
      cover_url?: string;
    },
  ) => apiPut<Project>(`/projects/${id}`, data),
  delete: (id: number) => apiDelete(`/projects/${id}`),

  listMembers: (id: number) => apiGet<ProjectMember[]>(`/projects/${id}/members`),
  addMember: (id: number, user_id: number, role_in_project = 'editor') =>
    apiPost(`/projects/${id}/members`, { user_id, role_in_project }),
  removeMember: (id: number, uid: number) => apiDelete(`/projects/${id}/members/${uid}`),
};

// =================== Model ===================

export interface Model {
  id: number;
  code: string;
  name: string;
  type: string;
  provider: string;
  endpoint: string;
  default_params: Record<string, unknown>;
  capability_tags: string[];
  enabled: number;
  priority: number;
  max_qps: number;
  health_check_url: string;
  last_health_at?: string;
  last_health_status: number;
}

export const modelApi = {
  list: (params?: {
    page?: number;
    page_size?: number;
    q?: string;
    type?: string;
    provider?: string;
    enabled?: number;
  }) => apiGet<Page<Model>>('/models', params),
  get: (id: number) => apiGet<Model>(`/models/${id}`),
  create: (data: {
    code: string;
    name: string;
    type: string;
    provider: string;
    endpoint: string;
    api_key?: string;
    model_name?: string;
    default_params?: Record<string, unknown>;
    capability_tags?: string[];
    priority?: number;
    max_qps?: number;
    health_check_url?: string;
  }) => apiPost<Model>('/models', data),
  update: (
    id: number,
    data: {
      name?: string;
      endpoint?: string;
      api_key?: string;
      default_params?: Record<string, unknown>;
      capability_tags?: string[];
      enabled?: number;
      priority?: number;
      max_qps?: number;
      health_check_url?: string;
    },
  ) => apiPut<Model>(`/models/${id}`, data),
  delete: (id: number) => apiDelete(`/models/${id}`),
  healthcheck: (id: number) =>
    apiPost<{ healthy: boolean; error?: string }>(
      `/models/${id}/healthcheck`,
      {},
      { timeout: 90_000, skipErrorToast: true },
    ),
};

// =================== Script / Episode / Prompt ===================

export interface Script {
  id: number;
  project_id: number;
  name: string;
  source_url: string;
  raw_text: string;
  current_version: number;
  status: number; // 1=uploaded, 2=parsed, 3=episode_split
  created_by: number;
  updated_by: number;
  created_at: string;
  updated_at: string;
}

export interface Episode {
  id: number;
  script_id: number;
  ep_no: number;
  title: string;
  summary: string;
  raw_segment: string;
  status: number;
  created_at: string;
  updated_at: string;
}

export interface ScriptVersion {
  id: number;
  script_id: number;
  version_no: number;
  content: string;
  commit_msg: string;
  created_by: number;
  created_at: string;
}

export interface EpisodePrompt {
  id: number;
  episode_id: number;
  content: Record<string, unknown> | string;
  model_id: number;
  generation_params: Record<string, unknown> | string;
  status: number;
  is_current?: number;
  generated_by: number;
  created_at: string;
}

export interface EnqueueResult {
  task_id: string;
  topic: string;
}

export const scriptApi = {
  list: (params?: { page?: number; page_size?: number; q?: string; project_id?: number; status?: number }) =>
    apiGet<Page<Script>>('/scripts', params),
  get: (id: number) => apiGet<Script>(`/scripts/${id}`),
  create: (data: { project_id: number; name: string; raw_text: string; source_url?: string }) =>
    apiPost<Script>('/scripts', data),
  delete: (id: number) => apiDelete(`/scripts/${id}`),
  episodes: (id: number) => apiGet<Episode[]>(`/scripts/${id}/episodes`),
  split: (
    id: number,
    data: { model_id: number; episode_count?: number; target_chars?: number; params?: Record<string, unknown> },
  ) => apiPost<EnqueueResult>(`/scripts/${id}/split`, data),
};

export const promptApi = {
  listByEpisode: (id: number) => apiGet<EpisodePrompt[]>(`/episodes/${id}/prompts`),
  getCurrent: (id: number) => apiGet<EpisodePrompt | null>(`/episodes/${id}/prompts/current`),
  generate: (id: number, data: { model_id: number; params?: Record<string, unknown> }) =>
    apiPost<EnqueueResult>(`/episodes/${id}/prompts/generate`, data),
  setCurrent: (id: number, episode_id: number) =>
    apiPost(`/prompts/${id}/set_current`, { episode_id }),
};

// =================== Storyboard / Style / Image / Short Video / Upload / Invocation ===================

export interface Storyboard {
  id: number;
  episode_id: number;
  prompt_id: number;
  shot_no: number;
  shot_type: string;
  camera_motion: string;
  scene_desc: string;
  characters: string[] | string | null;
  action: string;
  dialogue: string;
  duration_sec: number;
  notes: string;
  status: number;
  created_at: string;
  updated_at: string;
}

export const storyboardApi = {
  listByEpisode: (id: number) => apiGet<Storyboard[]>(`/episodes/${id}/storyboards`),
  generate: (id: number, data: { model_id: number; params?: Record<string, unknown> }) =>
    apiPost<EnqueueResult>(`/episodes/${id}/storyboards/generate`, data),
  bulkSave: (id: number, shots: Partial<Storyboard>[]) =>
    apiPost<{ saved: number }>(`/episodes/${id}/storyboards/bulk_save`, { shots }),
  get: (id: number) => apiGet<Storyboard>(`/storyboards/${id}`),
  update: (id: number, data: Partial<Storyboard>) => apiPut<Storyboard>(`/storyboards/${id}`, data),
  delete: (id: number) => apiDelete(`/storyboards/${id}`),
  applyStyle: (id: number, style_id: number) =>
    apiPost(`/storyboards/${id}/apply_style`, { style_id }),
};

export interface Style {
  id: number;
  project_id: number;
  name: string;
  art_style: string;
  color_tone: string;
  lighting: string;
  reference_images: string[] | string | null;
  lora_id: string;
  description: string;
  status: number;
  created_by: number;
  created_at: string;
  updated_at: string;
}

export const styleApi = {
  list: (project_id?: number) =>
    apiGet<Style[]>('/styles', project_id ? { project_id } : undefined),
  get: (id: number) => apiGet<Style>(`/styles/${id}`),
  create: (data: {
    project_id?: number;
    name: string;
    art_style?: string;
    color_tone?: string;
    lighting?: string;
    reference_images?: string[];
    lora_id?: string;
    description?: string;
  }) => apiPost<Style>('/styles', data),
  update: (
    id: number,
    data: {
      name?: string;
      art_style?: string;
      color_tone?: string;
      lighting?: string;
      reference_images?: string[];
      lora_id?: string;
      description?: string;
      status?: number;
    },
  ) => apiPut<Style>(`/styles/${id}`, data),
  delete: (id: number) => apiDelete(`/styles/${id}`),
};

export interface ImageItem {
  id: number;
  project_id: number;
  storyboard_id: number;
  src_type: string;
  url: string;
  thumb_url: string;
  width: number;
  height: number;
  prompt: string;
  neg_prompt: string;
  model_id: number;
  params: Record<string, unknown> | string;
  status: number;
  created_by: number;
  created_at: string;
}

export interface ImageGenInput {
  storyboard_id?: number;
  project_id?: number;
  style_id?: number;
  model_id: number;
  prompt: string;
  neg_prompt?: string;
  params?: Record<string, unknown>;
}

export const imageApi = {
  list: (params?: { page?: number; page_size?: number; project_id?: number; storyboard_id?: number; status?: number }) =>
    apiGet<Page<ImageItem>>('/images', params),
  get: (id: number) => apiGet<ImageItem>(`/images/${id}`),
  generate: (data: ImageGenInput) => apiPost<EnqueueResult>('/images/generate', data),
  delete: (id: number) => apiDelete(`/images/${id}`),
};

export interface ShortVideoItem {
  id: number;
  project_id: number;
  storyboard_id: number;
  src_type: string;
  prompt: string;
  source_image_ids: number[] | string | null;
  video_url: string;
  thumb_url: string;
  duration_ms: number;
  width: number;
  height: number;
  audio_url: string;
  subtitle_url: string;
  model_id: number;
  params: Record<string, unknown> | string;
  status: string;
  error_msg: string;
  created_by: number;
  created_at: string;
}

export interface VideoGenInput {
  storyboard_id?: number;
  project_id?: number;
  source_image_ids?: number[];
  prompt?: string;
  model_id: number;
  params?: Record<string, unknown>;
}

export const shortVideoApi = {
  list: (params?: { page?: number; page_size?: number; project_id?: number; storyboard_id?: number; status?: string }) =>
    apiGet<Page<ShortVideoItem>>('/short_videos', params),
  get: (id: number) => apiGet<ShortVideoItem>(`/short_videos/${id}`),
  generate: (data: VideoGenInput) => apiPost<EnqueueResult>('/short_videos/generate', data),
  delete: (id: number) => apiDelete(`/short_videos/${id}`),
};

export interface UploadResult {
  key: string;
  url: string;
  size: number;
  type: string;
}

export const uploadApi = {
  /** 上传文件;namespace 必填,后端只允许 images/videos/audios/styles/covers/scripts/misc */
  upload: async (namespace: string, file: File): Promise<UploadResult> => {
    const fd = new FormData();
    fd.append('namespace', namespace);
    fd.append('file', file);
    const token = useAuthStore.getState().accessToken;
    const resp = await fetch('/api/v1/files/upload', {
      method: 'POST',
      headers: {
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        'X-Trace-Id': createTraceId(),
      },
      body: fd,
    });
    const body = await resp.json();
    if (body.code !== 0) {
      const displayMessage = appendTraceHint(mapApiErrorMessage(body, resp.status), body);
      throw new ApiError(displayMessage, {
        ...body,
        status: resp.status,
      });
    }
    return body.data as UploadResult;
  },
};

export interface InvocationLog {
  id: number;
  model_id: number;
  user_id: number;
  dept_id: number;
  project_id: number;
  biz_type: string;
  biz_ref: string;
  input_tokens: number;
  output_tokens: number;
  units: number;
  duration_ms: number;
  cost: number;
  status: string;
  error_code: string;
  started_at: string;
  ended_at?: string;
}

export interface InvocationStats {
  calls: number;
  input_tokens: number;
  output_tokens: number;
  units: number;
  cost: number;
}

export const invocationApi = {
  list: (params?: {
    page?: number;
    page_size?: number;
    user_id?: number;
    dept_id?: number;
    project_id?: number;
    model_id?: number;
    biz_type?: string;
    status?: string;
    from?: string;
    to?: string;
  }) => apiGet<Page<InvocationLog>>('/invocations', params),
  stats: (params?: {
    user_id?: number;
    dept_id?: number;
    project_id?: number;
    model_id?: number;
    biz_type?: string;
    status?: string;
    from?: string;
    to?: string;
  }) => apiGet<InvocationStats>('/invocations/stats', params),
};

// =================== Pipeline ===================

export interface Pipeline {
  id: number;
  project_id: number;
  name: string;
  description: string;
  dag: Record<string, unknown> | string;
  is_template: number;
  enabled: number;
  created_by: number;
  created_at: string;
  updated_at: string;
}

export interface PipelineRun {
  id: number;
  pipeline_id: number;
  project_id: number;
  triggered_by: number;
  trigger_type: string;
  input: Record<string, unknown> | string;
  output: Record<string, unknown> | string;
  status: string;
  started_at?: string;
  ended_at?: string;
  error_msg: string;
}

export interface StepRun {
  id: number;
  run_id: number;
  node_id: string;
  node_type: string;
  model_id: number;
  input: Record<string, unknown> | string;
  output: Record<string, unknown> | string;
  status: string;
  attempt: number;
  started_at?: string;
  ended_at?: string;
  error_msg: string;
}

export const pipelineApi = {
  list: (params?: { page?: number; page_size?: number; project_id?: number; is_template?: number; enabled?: number }) =>
    apiGet<Page<Pipeline>>('/pipelines', params),
  get: (id: number) => apiGet<Pipeline>(`/pipelines/${id}`),
  create: (data: { project_id?: number; name: string; description?: string; dag: Record<string, unknown>; is_template?: number; enabled?: number }) =>
    apiPost<Pipeline>('/pipelines', data),
  update: (id: number, data: { name?: string; description?: string; dag?: Record<string, unknown>; is_template?: number; enabled?: number }) =>
    apiPut<Pipeline>(`/pipelines/${id}`, data),
  delete: (id: number) => apiDelete(`/pipelines/${id}`),
  run: (id: number, body: { input?: Record<string, unknown>; node_overrides?: Record<string, unknown> }) =>
    apiPost<{ run_id: number; status: string; topic?: string }>(`/pipelines/${id}/run`, body),
  listRuns: (pipelineId: number, params?: { page?: number; page_size?: number }) =>
    apiGet<Page<PipelineRun>>(`/pipelines/${pipelineId}/runs`, params),
  getRun: (runId: number) => apiGet<PipelineRun>(`/pipeline_runs/${runId}`),
  listSteps: (runId: number) => apiGet<StepRun[]>(`/pipeline_runs/${runId}/steps`),
};

// =================== Full Video ===================

export interface TimelineClip {
  short_video_id?: number;
  url?: string;
  duration_ms?: number;
  tts_text?: string;
  speaker?: string;
}

export interface Timeline {
  clips: TimelineClip[];
  background_audio_url?: string;
  tts_model_id?: number;
  burn_subtitles?: boolean;
  subtitle_url?: string;
}

export interface FullVideo {
  id: number;
  project_id: number;
  name: string;
  version: number;
  timeline: Timeline | string;
  output_url: string;
  thumb_url: string;
  cover_url: string;
  duration_ms: number;
  status: string;
  render_progress: number;
  error_msg: string;
  created_by: number;
  created_at: string;
  updated_at: string;
}

export const fullVideoApi = {
  list: (params?: { page?: number; page_size?: number; project_id?: number; status?: string }) =>
    apiGet<Page<FullVideo>>('/full_videos', params),
  get: (id: number) => apiGet<FullVideo>(`/full_videos/${id}`),
  create: (data: { project_id: number; name: string; timeline: Timeline }) =>
    apiPost<FullVideo>('/full_videos', data),
  update: (id: number, data: { name?: string; timeline?: Timeline }) =>
    apiPut<FullVideo>(`/full_videos/${id}`, data),
  delete: (id: number) => apiDelete(`/full_videos/${id}`),
  render: (id: number) => apiPost<EnqueueResult>(`/full_videos/${id}/render`),
};

// =================== Review ===================

export interface ReviewFlow {
  id: number;
  name: string;
  description: string;
  target_type: string;
  enabled: number;
  is_default: number;
}

export interface ReviewNode {
  id: number;
  flow_id: number;
  step_no: number;
  name: string;
  approver_type: string;
  approver_value: string;
  allow_timeout_pass: number;
  timeout_hours: number;
}

export interface ReviewRecord {
  id: number;
  target_type: string;
  target_id: number;
  flow_id: number;
  current_step: number;
  status: string;
  submitted_by: number;
  finished_at?: string;
  created_at: string;
  updated_at: string;
}

export interface ReviewNodeRecord {
  id: number;
  review_record_id: number;
  step_no: number;
  approver_id: number;
  action: string;
  comment: string;
  acted_at: string;
}

export const reviewApi = {
  listFlows: () => apiGet<ReviewFlow[]>('/review/flows'),
  getFlow: (id: number) => apiGet<ReviewFlow>(`/review/flows/${id}`),
  listNodes: (flowId: number) => apiGet<ReviewNode[]>(`/review/flows/${flowId}/nodes`),

  submit: (data: { target_type: string; target_id: number; flow_id?: number; note?: string }) =>
    apiPost<ReviewRecord>('/review/records', data),
  listRecords: (params?: { page?: number; page_size?: number; status?: string }) =>
    apiGet<Page<ReviewRecord>>('/review/records', params),
  getRecord: (id: number) => apiGet<ReviewRecord>(`/review/records/${id}`),
  listActions: (id: number) => apiGet<ReviewNodeRecord[]>(`/review/records/${id}/actions`),
  act: (id: number, data: { action: string; comment?: string }) =>
    apiPost<ReviewRecord>(`/review/records/${id}/act`, data),
  cancel: (id: number) => apiPost(`/review/records/${id}/cancel`),
};

// =================== Publish ===================

export interface PublishItem {
  id: number;
  full_video_id: number;
  published_by: number;
  published_at: string;
  status: string;
  watermark_config: Record<string, unknown> | string;
  download_count: number;
  play_count: number;
  updated_at: string;
}

export const publishApi = {
  publish: (data: { full_video_id: number; watermark_config?: Record<string, unknown> }) =>
    apiPost<PublishItem>('/publishes', data),
  unpublish: (videoId: number) => apiPost(`/publishes/${videoId}/unpublish`),
  get: (videoId: number) => apiGet<PublishItem>(`/publishes/${videoId}`),
  list: (params?: { page?: number; page_size?: number; status?: string }) =>
    apiGet<Page<PublishItem>>('/publishes', params),
  incPlay: (videoId: number) => apiPost(`/publishes/${videoId}/play`),
  incDownload: (videoId: number) => apiPost(`/publishes/${videoId}/download`),
  updateWatermark: (videoId: number, watermark_config: Record<string, unknown>) =>
    apiPut<PublishItem>(`/publishes/${videoId}/watermark`, { watermark_config }),
};

// =================== Billing ===================

export interface BillingQuota {
  id: number;
  scope_type: string;
  scope_id: number;
  model_id: number;
  period: string;
  metric: string;
  quota_value: number;
  used_value: number;
  reset_at?: string;
  enabled: number;
}

export interface BillingDaily {
  id: number;
  stat_date: string;
  model_id: number;
  dept_id: number;
  user_id: number;
  calls: number;
  input_tokens: number;
  output_tokens: number;
  units: number;
  cost: number;
}

export const billingApi = {
  listQuotas: (params?: { scope_type?: string; scope_id?: number }) =>
    apiGet<BillingQuota[]>('/billing/quotas', params),
  getQuota: (id: number) => apiGet<BillingQuota>(`/billing/quotas/${id}`),
  createQuota: (data: { scope_type: string; scope_id: number; model_id?: number; period?: string; metric: string; quota_value: number; enabled?: number }) =>
    apiPost<BillingQuota>('/billing/quotas', data),
  updateQuota: (id: number, data: { quota_value?: number; enabled?: number; period?: string }) =>
    apiPut<BillingQuota>(`/billing/quotas/${id}`, data),
  deleteQuota: (id: number) => apiDelete(`/billing/quotas/${id}`),
  listDaily: (params?: { from?: string; to?: string; user_id?: number; dept_id?: number; model_id?: number }) =>
    apiGet<BillingDaily[]>('/billing/daily', params),
};

// =================== Audit ===================

export interface AuditEntry {
  id: number;
  user_id: number;
  action: string;
  resource_type: string;
  resource_id: string;
  before: Record<string, unknown> | string;
  after: Record<string, unknown> | string;
  ip: string;
  ua: string;
  request_id: string;
  created_at: string;
}

export const auditApi = {
  list: (params?: { page?: number; page_size?: number; user_id?: number; resource_type?: string; action?: string }) =>
    apiGet<Page<AuditEntry>>('/audit_logs', params),
};

// =================== Feature Flag ===================

export interface FeatureFlag {
  id: number;
  key: string;
  description: string;
  enabled: number;
  rollout: number;
  rules: { users?: number[]; depts?: number[]; projects?: number[] } | string;
  created_at: string;
  updated_at: string;
}

export const featureFlagApi = {
  list: () => apiGet<FeatureFlag[]>('/feature_flags'),
  get: (id: number) => apiGet<FeatureFlag>(`/feature_flags/${id}`),
  create: (data: { key: string; description?: string; enabled?: number; rollout?: number; rules?: Record<string, unknown> }) =>
    apiPost<FeatureFlag>('/feature_flags', data),
  update: (id: number, data: { description?: string; enabled?: number; rollout?: number; rules?: Record<string, unknown> }) =>
    apiPut<FeatureFlag>(`/feature_flags/${id}`, data),
  delete: (id: number) => apiDelete(`/feature_flags/${id}`),
  evaluate: (key: string) => apiGet<{ key: string; enabled: boolean }>(`/feature_flags/evaluate`, { key }),
};
