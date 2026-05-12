import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { User } from '@/api/modules';

interface AuthState {
  accessToken: string | null;
  refreshToken: string | null;
  user: User | null;
  login(payload: { accessToken: string; refreshToken: string; user: User }): void;
  logout(): void;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      accessToken: null,
      refreshToken: null,
      user: null,
      login: ({ accessToken, refreshToken, user }) => set({ accessToken, refreshToken, user }),
      logout: () => set({ accessToken: null, refreshToken: null, user: null }),
    }),
    { name: 'ai-script-auth' },
  ),
);
