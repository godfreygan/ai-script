const TRACE_STORAGE_KEY = 'ai-script-trace-id';

function randomHex(size: number): string {
  const bytes = new Uint8Array(size);
  window.crypto.getRandomValues(bytes);
  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('');
}

function createTraceId(): string {
  try {
    return randomHex(12);
  } catch {
    return `${Date.now().toString(16)}${Math.random().toString(16).slice(2, 10)}`;
  }
}

export function getTraceId(): string {
  const cached = window.localStorage.getItem(TRACE_STORAGE_KEY);
  if (cached) {
    return cached;
  }
  const traceId = createTraceId();
  window.localStorage.setItem(TRACE_STORAGE_KEY, traceId);
  return traceId;
}
