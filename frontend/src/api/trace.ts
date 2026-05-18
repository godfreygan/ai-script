function randomHex(size: number): string {
  const cryptoObj = globalThis.crypto;
  if (!cryptoObj?.getRandomValues) {
    throw new Error('crypto unavailable');
  }
  const bytes = new Uint8Array(size);
  cryptoObj.getRandomValues(bytes);
  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('');
}

/** 为单次 HTTP 请求生成新的 trace_id（不缓存、不复用） */
export function createTraceId(): string {
  try {
    return randomHex(12);
  } catch {
    return `${Date.now().toString(16)}${Math.random().toString(16).slice(2, 10)}`;
  }
}
