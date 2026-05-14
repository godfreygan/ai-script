import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { useAuthStore } from './auth';
import type { User } from '@/api/modules';

describe('auth store', () => {
  const mockUser: User = {
    id: 1,
    username: 'testuser',
    nickname: 'Test User',
    email: 'test@example.com',
    phone: '13800138000',
    dept_id: 1,
    status: 1,
    roles: ['admin'],
  };

  const loginPayload = {
    accessToken: 'access-token-123',
    refreshToken: 'refresh-token-456',
    user: mockUser,
  };

  beforeEach(() => {
    // 清除 localStorage 中可能存在的持久化数据
    localStorage.removeItem('ai-script-auth');
    // 重置 store 到初始状态
    useAuthStore.setState({
      accessToken: null,
      refreshToken: null,
      user: null,
    });
  });

  afterEach(() => {
    localStorage.removeItem('ai-script-auth');
  });

  describe('initial state', () => {
    it('should have null tokens and user by default', () => {
      const state = useAuthStore.getState();
      expect(state.accessToken).toBeNull();
      expect(state.refreshToken).toBeNull();
      expect(state.user).toBeNull();
    });
  });

  describe('login action', () => {
    it('should set accessToken, refreshToken and user', () => {
      useAuthStore.getState().login(loginPayload);

      const state = useAuthStore.getState();
      expect(state.accessToken).toBe('access-token-123');
      expect(state.refreshToken).toBe('refresh-token-456');
      expect(state.user).toEqual(mockUser);
    });

    it('should overwrite existing auth state on re-login', () => {
      const anotherUser: User = { ...mockUser, id: 2, username: 'another' };
      useAuthStore.getState().login(loginPayload);
      useAuthStore.getState().login({
        accessToken: 'new-token',
        refreshToken: 'new-refresh',
        user: anotherUser,
      });

      const state = useAuthStore.getState();
      expect(state.accessToken).toBe('new-token');
      expect(state.user?.username).toBe('another');
    });
  });

  describe('logout action', () => {
    it('should clear all auth state back to null', () => {
      useAuthStore.getState().login(loginPayload);
      useAuthStore.getState().logout();

      const state = useAuthStore.getState();
      expect(state.accessToken).toBeNull();
      expect(state.refreshToken).toBeNull();
      expect(state.user).toBeNull();
    });

    it('should be safe to call logout when already logged out', () => {
      useAuthStore.getState().logout();
      const state = useAuthStore.getState();
      expect(state.accessToken).toBeNull();
      expect(state.refreshToken).toBeNull();
      expect(state.user).toBeNull();
    });
  });

  describe('persistence', () => {
    it('should persist auth data to localStorage after login', () => {
      useAuthStore.getState().login(loginPayload);

      const raw = localStorage.getItem('ai-script-auth');
      expect(raw).toBeTruthy();
      const parsed = JSON.parse(raw!);
      expect(parsed.state.accessToken).toBe('access-token-123');
      expect(parsed.state.refreshToken).toBe('refresh-token-456');
      expect(parsed.state.user.id).toBe(1);
    });

    it('should clear persisted data after logout', () => {
      useAuthStore.getState().login(loginPayload);
      useAuthStore.getState().logout();

      const raw = localStorage.getItem('ai-script-auth');
      expect(raw).toBeTruthy();
      const parsed = JSON.parse(raw!);
      expect(parsed.state.accessToken).toBeNull();
      expect(parsed.state.refreshToken).toBeNull();
      expect(parsed.state.user).toBeNull();
    });
  });
});
