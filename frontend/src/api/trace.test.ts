import { describe, it, expect, beforeEach } from 'vitest';
import { createTraceId } from './trace';

describe('createTraceId', () => {
  beforeEach(() => {
    window.localStorage.removeItem('ai-script-trace-id');
  });

  it('returns a new id on each call', () => {
    const a = createTraceId();
    const b = createTraceId();
    expect(a).toMatch(/^[0-9a-f]+$/);
    expect(b).toMatch(/^[0-9a-f]+$/);
    expect(a).not.toBe(b);
  });

  it('does not persist to localStorage', () => {
    createTraceId();
    expect(window.localStorage.getItem('ai-script-trace-id')).toBeNull();
  });
});
