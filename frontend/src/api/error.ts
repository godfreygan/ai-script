export interface ApiErrorPayload {
  code?: number;
  message?: string;
  error?: string;
  request_id?: string;
  trace_id?: string;
}

export class ApiError extends Error {
  code?: number;
  status?: number;
  requestId?: string;
  traceId?: string;
  handled?: boolean;

  constructor(message: string, options?: ApiErrorPayload & { status?: number; handled?: boolean }) {
    super(message);
    this.name = 'ApiError';
    this.code = options?.code;
    this.status = options?.status;
    this.requestId = options?.request_id;
    this.traceId = options?.trace_id;
    this.handled = options?.handled;
  }
}

const codeMessageMap: Record<number, string> = {
  20010001: '请求参数有误，请检查后重试',
  20010002: '提交内容不符合要求，请检查后重试',
  20010003: '密码强度不足，请按提示修改',
  20020001: '登录状态已失效，请重新登录',
  20020002: '登录凭证无效，请重新登录',
  20030001: '当前账号没有权限执行该操作',
  20040001: '请求的资源不存在或已被删除',
  20050001: '当前资源状态冲突，请刷新后重试',
  20050002: '当前状态不允许执行该操作',
  20060001: '当前额度不足，请调整后重试',
  20070001: '账户已被锁定，请稍后再试',
  20080001: '请求过于频繁，请稍后再试',
  20100001: '服务开小差了，请稍后重试',
  20100002: '依赖服务暂时不可用，请稍后重试',
  20100003: '请求处理超时，请稍后重试',
};

const statusMessageMap: Record<number, string> = {
  400: '请求参数有误，请检查后重试',
  401: '登录状态已失效，请重新登录',
  403: '当前账号没有权限执行该操作',
  404: '请求的资源不存在或已被删除',
  409: '数据状态冲突，请刷新后重试',
  423: '账户或资源已被锁定，请稍后再试',
  429: '请求过于频繁，请稍后再试',
  500: '服务开小差了，请稍后重试',
  502: '网关异常，请稍后重试',
  503: '服务暂时不可用，请稍后重试',
  504: '服务响应超时，请稍后重试',
};

export function mapApiErrorMessage(payload?: ApiErrorPayload, status?: number): string {
  if (payload?.code && codeMessageMap[payload.code]) {
    return codeMessageMap[payload.code];
  }
  if (status && statusMessageMap[status]) {
    return statusMessageMap[status];
  }
  if (payload?.message) {
    return payload.message;
  }
  if (payload?.error) {
    return payload.error;
  }
  return '请求失败，请稍后重试';
}

export function appendTraceHint(message: string, payload?: ApiErrorPayload): string {
  const traceId = payload?.trace_id || payload?.request_id;
  if (!traceId) {
    return message;
  }
  return `${message}，追踪ID：${traceId}`;
}
