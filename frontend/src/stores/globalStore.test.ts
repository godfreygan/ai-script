import { describe, it, expect, beforeEach } from 'vitest';
import { useGlobalStore, __resetGlobalStore } from './globalStore';

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

describe('globalStore', () => {
  beforeEach(() => {
    __resetGlobalStore();
    useGlobalStore.setState({ loading: false, loadingText: '' });
  });

  describe('initial state', () => {
    it('should have loading false and empty loadingText by default', () => {
      const state = useGlobalStore.getState();
      expect(state.loading).toBe(false);
      expect(state.loadingText).toBe('');
    });
  });

  describe('setLoading action', () => {
    it('should set loading to true with text', () => {
      useGlobalStore.getState().setLoading(true, 'Loading data...');
      const state = useGlobalStore.getState();
      expect(state.loading).toBe(true);
      expect(state.loadingText).toBe('Loading data...');
    });

    it('should set loading to false and clear text', () => {
      useGlobalStore.getState().setLoading(true, 'Loading...');
      useGlobalStore.getState().setLoading(false);
      const state = useGlobalStore.getState();
      expect(state.loading).toBe(false);
      expect(state.loadingText).toBe('');
    });

    it('should default loadingText to empty string when not provided', () => {
      useGlobalStore.getState().setLoading(true);
      const state = useGlobalStore.getState();
      expect(state.loadingText).toBe('');
    });
  });

  describe('showGlobalLoading / hideGlobalLoading', () => {
    it('should set loading true on showGlobalLoading', () => {
      useGlobalStore.getState().showGlobalLoading();
      expect(useGlobalStore.getState().loading).toBe(true);
    });

    it('should keep loading true when showGlobalLoading is called multiple times', () => {
      useGlobalStore.getState().showGlobalLoading();
      useGlobalStore.getState().showGlobalLoading();
      useGlobalStore.getState().showGlobalLoading();
      expect(useGlobalStore.getState().loading).toBe(true);
    });

    it('should delay hiding loading by ~150ms after single hideGlobalLoading', async () => {
      useGlobalStore.getState().showGlobalLoading();
      useGlobalStore.getState().hideGlobalLoading();

      // 立即检查，仍然为 true
      expect(useGlobalStore.getState().loading).toBe(true);

      // 等待 100ms，仍然为 true
      await sleep(100);
      expect(useGlobalStore.getState().loading).toBe(true);

      // 等待足够时间（150ms + 缓冲），变为 false
      await sleep(100);
      expect(useGlobalStore.getState().loading).toBe(false);
      expect(useGlobalStore.getState().loadingText).toBe('');
    });

    it('should not hide loading if a new request starts before timer fires', async () => {
      useGlobalStore.getState().showGlobalLoading();
      useGlobalStore.getState().hideGlobalLoading();

      // 在 150ms 延迟内又发起了一个请求
      await sleep(100);
      useGlobalStore.getState().showGlobalLoading();

      // 即使超过原来的 150ms，loading 仍为 true
      await sleep(100);
      expect(useGlobalStore.getState().loading).toBe(true);
    });

    it('should only hide loading when all requests are done', async () => {
      // 3 个并发请求
      useGlobalStore.getState().showGlobalLoading();
      useGlobalStore.getState().showGlobalLoading();
      useGlobalStore.getState().showGlobalLoading();

      // 结束 1 个
      useGlobalStore.getState().hideGlobalLoading();
      await sleep(200);
      expect(useGlobalStore.getState().loading).toBe(true);

      // 结束第 2 个
      useGlobalStore.getState().hideGlobalLoading();
      await sleep(200);
      expect(useGlobalStore.getState().loading).toBe(true);

      // 结束最后一个
      useGlobalStore.getState().hideGlobalLoading();
      await sleep(200);
      expect(useGlobalStore.getState().loading).toBe(false);
    });

    it('should not go below zero when hideGlobalLoading is called excessively', async () => {
      useGlobalStore.getState().showGlobalLoading();
      useGlobalStore.getState().hideGlobalLoading();
      await sleep(200);
      expect(useGlobalStore.getState().loading).toBe(false);

      // 再次调用 hide，不应导致负数计数问题
      useGlobalStore.getState().hideGlobalLoading();
      await sleep(200);
      expect(useGlobalStore.getState().loading).toBe(false);
    });

    it('should clear previous timer when showGlobalLoading is called during pending hide', async () => {
      useGlobalStore.getState().showGlobalLoading();
      useGlobalStore.getState().hideGlobalLoading();

      // 在延迟期间重新 show
      await sleep(100);
      useGlobalStore.getState().showGlobalLoading();

      // 原来的 timer 应该被清除了，即使时间超过 150ms 也不会自动隐藏
      await sleep(200);
      expect(useGlobalStore.getState().loading).toBe(true);
    });
  });
});
