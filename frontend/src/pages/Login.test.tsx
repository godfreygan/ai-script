import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { App as AntApp } from 'antd';
import LoginPage from './Login';
import { authApi } from '@/api/modules';
import { useAuthStore } from '@/stores/auth';

// Mock API module
vi.mock('@/api/modules', async () => {
  const actual = await vi.importActual<typeof import('@/api/modules')>('@/api/modules');
  return {
    ...actual,
    authApi: {
      login: vi.fn(),
    },
  };
});

// Mock react-router-dom navigate
const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

function renderLogin() {
  return render(
    <MemoryRouter initialEntries={['/login']}>
      <AntApp>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/" element={<div data-testid="dashboard">Dashboard</div>} />
        </Routes>
      </AntApp>
    </MemoryRouter>
  );
}

describe('LoginPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useAuthStore.setState({ accessToken: null, refreshToken: null, user: null });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders login form with username and password fields', () => {
    renderLogin();

    expect(screen.getByText('AI 短剧平台 · 登录')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('用户名')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('密码')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /登\s*录/ })).toBeInTheDocument();
  });

  it('shows validation error when submitting empty form', async () => {
    renderLogin();
    const user = userEvent.setup();

    const submitBtn = screen.getByRole('button', { name: /登\s*录/ });
    await user.click(submitBtn);

    await waitFor(() => {
      expect(screen.getByText('请输入用户名')).toBeInTheDocument();
      expect(screen.getByText('请输入密码')).toBeInTheDocument();
    });
  });

  it('shows validation error when only username is filled', async () => {
    renderLogin();
    const user = userEvent.setup();

    await user.type(screen.getByPlaceholderText('用户名'), 'admin');
    await user.click(screen.getByRole('button', { name: /登\s*录/ }));

    await waitFor(() => {
      expect(screen.getByText('请输入密码')).toBeInTheDocument();
    });
  });

  it('submits form and navigates to dashboard on success', async () => {
    const mockedLogin = vi.mocked(authApi.login);
    mockedLogin.mockResolvedValueOnce({
      access_token: 'at_123',
      refresh_token: 'rt_456',
      expires_in: 3600,
      user: {
        id: 1,
        username: 'admin',
        nickname: 'Admin',
        email: 'admin@test.com',
        phone: '13800138000',
        dept_id: 1,
        status: 1,
      },
      roles: ['admin'],
    });

    renderLogin();
    const user = userEvent.setup();

    await user.type(screen.getByPlaceholderText('用户名'), 'admin');
    await user.type(screen.getByPlaceholderText('密码'), 'password123');
    await user.click(screen.getByRole('button', { name: /登\s*录/ }));

    await waitFor(() => {
      expect(mockedLogin).toHaveBeenCalledWith('admin', 'password123');
    });

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/');
    });

    // Verify auth store state
    const state = useAuthStore.getState();
    expect(state.accessToken).toBe('at_123');
    expect(state.refreshToken).toBe('rt_456');
    expect(state.user?.username).toBe('admin');
  });

  it('shows error message on login failure', async () => {
    const mockedLogin = vi.mocked(authApi.login);
    mockedLogin.mockRejectedValueOnce(new Error('用户名或密码错误'));

    renderLogin();
    const user = userEvent.setup();

    await user.type(screen.getByPlaceholderText('用户名'), 'admin');
    await user.type(screen.getByPlaceholderText('密码'), 'wrongpassword');
    await user.click(screen.getByRole('button', { name: /登\s*录/ }));

    await waitFor(() => {
      expect(mockedLogin).toHaveBeenCalledWith('admin', 'wrongpassword');
    });

    // Error message is shown via Antd message (we verify login was not successful)
    await waitFor(() => {
      expect(mockNavigate).not.toHaveBeenCalled();
    });

    // Auth store should remain empty
    const state = useAuthStore.getState();
    expect(state.accessToken).toBeNull();
    expect(state.user).toBeNull();
  });

  it('shows loading state during submission', async () => {
    const mockedLogin = vi.mocked(authApi.login);
    let resolveLogin: (value: unknown) => void;
    const loginPromise = new Promise((resolve) => {
      resolveLogin = resolve;
    });
    mockedLogin.mockReturnValueOnce(loginPromise as Promise<never>);

    renderLogin();
    const user = userEvent.setup();

    await user.type(screen.getByPlaceholderText('用户名'), 'admin');
    await user.type(screen.getByPlaceholderText('密码'), 'password123');
    await user.click(screen.getByRole('button', { name: /登\s*录/ }));

    // Button should be in loading state (antd sets ant-btn-loading class, not disabled attr)
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /登\s*录/ })).toHaveClass('ant-btn-loading');
    });

    // Resolve the promise
    resolveLogin!({
      access_token: 'at_123',
      refresh_token: 'rt_456',
      expires_in: 3600,
      user: {
        id: 1,
        username: 'admin',
        nickname: 'Admin',
        email: 'admin@test.com',
        phone: '13800138000',
        dept_id: 1,
        status: 1,
      },
      roles: ['admin'],
    });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /登\s*录/ })).not.toHaveClass('ant-btn-loading');
    });
  });

  it('prevents duplicate submission while loading', async () => {
    const mockedLogin = vi.mocked(authApi.login);
    let resolveLogin: (value: unknown) => void;
    const loginPromise = new Promise((resolve) => {
      resolveLogin = resolve;
    });
    mockedLogin.mockReturnValueOnce(loginPromise as Promise<never>);

    renderLogin();
    const user = userEvent.setup();

    await user.type(screen.getByPlaceholderText('用户名'), 'admin');
    await user.type(screen.getByPlaceholderText('密码'), 'password123');

    // Click multiple times rapidly
    const submitBtn = screen.getByRole('button', { name: /登\s*录/ });
    await user.click(submitBtn);
    await user.click(submitBtn);
    await user.click(submitBtn);

    // Should only be called once because loading prevents duplicate
    await waitFor(() => {
      expect(mockedLogin).toHaveBeenCalledTimes(1);
    });

    resolveLogin!({
      access_token: 'at_123',
      refresh_token: 'rt_456',
      expires_in: 3600,
      user: {
        id: 1,
        username: 'admin',
        nickname: 'Admin',
        email: 'admin@test.com',
        phone: '13800138000',
        dept_id: 1,
        status: 1,
      },
      roles: ['admin'],
    });
  });
});
