import { create } from 'zustand';

interface GlobalState {
  loading: boolean;
  loadingText: string;
  setLoading: (loading: boolean, text?: string) => void;
  showGlobalLoading: () => void;
  hideGlobalLoading: () => void;
}

let loadingTimer: ReturnType<typeof setTimeout> | null = null;
let activeRequests = 0;

/** @internal 仅用于测试重置模块级状态 */
export const __resetGlobalStore = () => {
  if (loadingTimer) {
    clearTimeout(loadingTimer);
    loadingTimer = null;
  }
  activeRequests = 0;
};

export const useGlobalStore = create<GlobalState>((set) => ({
  loading: false,
  loadingText: '',
  setLoading: (loading, text = '') => set({ loading, loadingText: text }),
  showGlobalLoading: () => {
    activeRequests += 1;
    if (loadingTimer) {
      clearTimeout(loadingTimer);
      loadingTimer = null;
    }
    set({ loading: true });
  },
  hideGlobalLoading: () => {
    activeRequests = Math.max(0, activeRequests - 1);
    if (activeRequests === 0) {
      // 延迟 150ms 关闭，避免闪烁
      loadingTimer = setTimeout(() => {
        set({ loading: false, loadingText: '' });
      }, 150);
    }
  },
}));
