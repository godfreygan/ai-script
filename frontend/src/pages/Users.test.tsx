import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import UsersPage from './Users';
import { userApi, deptApi, roleApi } from '@/api/modules';

// Mock API modules
vi.mock('@/api/modules', async () => {
  const actual = await vi.importActual<typeof import('@/api/modules')>('@/api/modules');
  return {
    ...actual,
    userApi: {
      list: vi.fn(),
      get: vi.fn(),
      create: vi.fn(),
      update: vi.fn(),
      delete: vi.fn(),
      resetPassword: vi.fn(),
    },
    deptApi: {
      list: vi.fn(),
    },
    roleApi: {
      list: vi.fn(),
    },
  };
});

// Mock AntApp.useApp so message.error/success don't crash in jsdom
vi.mock('antd', async () => {
  const actual = await vi.importActual<typeof import('antd')>('antd');
  return {
    ...actual,
    App: {
      useApp: () => ({
        message: {
          success: vi.fn(),
          error: vi.fn(),
        },
        notification: {},
        modal: {},
      }),
    },
  };
});

function renderUsersPage() {
  return render(<UsersPage />);
}

const mockUsers = [
  {
    id: 1,
    username: 'admin',
    nickname: '管理员',
    email: 'admin@example.com',
    phone: '13800138000',
    dept_id: 1,
    status: 1,
    last_login_at: '2024-01-01 12:00:00',
    created_at: '2024-01-01 10:00:00',
  },
  {
    id: 2,
    username: 'user01',
    nickname: '用户一',
    email: 'user01@example.com',
    phone: '13900139000',
    dept_id: 2,
    status: 2,
    last_login_at: '',
    created_at: '2024-01-02 10:00:00',
  },
];

const mockDepts = [
  { id: 1, name: '技术部', parent_id: 0, path: '1', sort: 1, status: 1 },
  { id: 2, name: '产品部', parent_id: 0, path: '2', sort: 2, status: 1 },
];

const mockRoles = [
  { id: 1, code: 'admin', name: '管理员', description: '', data_scope: 'all', is_system: 1, status: 1 },
  { id: 2, code: 'editor', name: '编辑', description: '', data_scope: 'dept', is_system: 0, status: 1 },
];

describe('UsersPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders page with title', async () => {
    const mockedUserList = vi.mocked(userApi.list);
    const mockedDeptList = vi.mocked(deptApi.list);
    const mockedRoleList = vi.mocked(roleApi.list);

    mockedUserList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });
    mockedDeptList.mockResolvedValue(mockDepts);
    mockedRoleList.mockResolvedValue(mockRoles);

    renderUsersPage();

    await waitFor(() => {
      expect(screen.getByText('用户管理')).toBeInTheDocument();
    });
  });

  it('shows loading state initially', async () => {
    const mockedUserList = vi.mocked(userApi.list);
    const mockedDeptList = vi.mocked(deptApi.list);
    const mockedRoleList = vi.mocked(roleApi.list);

    mockedUserList.mockReturnValue(new Promise(() => {}));
    mockedDeptList.mockReturnValue(new Promise(() => {}));
    mockedRoleList.mockReturnValue(new Promise(() => {}));

    renderUsersPage();

    await waitFor(() => {
      expect(document.querySelector('[aria-busy="true"]')).toBeInTheDocument();
    });
  });

  it('displays user list in table', async () => {
    const mockedUserList = vi.mocked(userApi.list);
    const mockedDeptList = vi.mocked(deptApi.list);
    const mockedRoleList = vi.mocked(roleApi.list);

    mockedUserList.mockResolvedValue({ total: 2, list: mockUsers, page: 1, page_size: 20 });
    mockedDeptList.mockResolvedValue(mockDepts);
    mockedRoleList.mockResolvedValue(mockRoles);

    renderUsersPage();

    await waitFor(() => {
      expect(screen.getByText('admin')).toBeInTheDocument();
    });

    expect(screen.getByText('user01')).toBeInTheDocument();
    expect(screen.getByText('管理员')).toBeInTheDocument();
    expect(screen.getByText('用户一')).toBeInTheDocument();
    expect(screen.getByText('admin@example.com')).toBeInTheDocument();
    expect(screen.getByText('user01@example.com')).toBeInTheDocument();
    expect(screen.getByText('技术部')).toBeInTheDocument();
    expect(screen.getByText('产品部')).toBeInTheDocument();
    expect(screen.getByText('启用')).toBeInTheDocument();
    expect(screen.getByText('禁用')).toBeInTheDocument();
  });

  it('displays empty state when no users', async () => {
    const mockedUserList = vi.mocked(userApi.list);
    const mockedDeptList = vi.mocked(deptApi.list);
    const mockedRoleList = vi.mocked(roleApi.list);

    mockedUserList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });
    mockedDeptList.mockResolvedValue(mockDepts);
    mockedRoleList.mockResolvedValue(mockRoles);

    renderUsersPage();

    await waitFor(() => {
      expect(screen.getAllByText('No data').length).toBeGreaterThan(0);
    });
  });

  it('handles API error gracefully', async () => {
    const mockedUserList = vi.mocked(userApi.list);
    const mockedDeptList = vi.mocked(deptApi.list);
    const mockedRoleList = vi.mocked(roleApi.list);

    mockedUserList.mockRejectedValue(new Error('Network error'));
    mockedDeptList.mockResolvedValue(mockDepts);
    mockedRoleList.mockResolvedValue(mockRoles);

    renderUsersPage();

    await waitFor(() => {
      expect(screen.getByText('用户管理')).toBeInTheDocument();
    });

    // Table should show empty state after error
    await waitFor(() => {
      expect(screen.getAllByText('No data').length).toBeGreaterThan(0);
    });
  });

  it('opens create user modal and submits', async () => {
    const user = userEvent.setup();
    const mockedUserList = vi.mocked(userApi.list);
    const mockedDeptList = vi.mocked(deptApi.list);
    const mockedRoleList = vi.mocked(roleApi.list);
    const mockedUserCreate = vi.mocked(userApi.create);

    mockedUserList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });
    mockedDeptList.mockResolvedValue(mockDepts);
    mockedRoleList.mockResolvedValue(mockRoles);
    mockedUserCreate.mockResolvedValue({ id: 3, username: 'newuser', nickname: '新用户', email: '', phone: '', dept_id: 0, status: 1 });

    renderUsersPage();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /新建用户/ })).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: /新建用户/ }));

    // Modal title
    await waitFor(() => {
      expect(screen.getByRole('dialog', { name: '新建用户' })).toBeInTheDocument();
    });

    // Fill form
    const usernameInput = screen.getByLabelText('用户名');
    const passwordInput = screen.getByLabelText('密码');
    const nicknameInput = screen.getByLabelText('昵称');

    await user.type(usernameInput, 'newuser');
    await user.type(passwordInput, '123456');
    await user.type(nicknameInput, '新用户');

    await user.click(screen.getByRole('button', { name: 'OK' }));

    await waitFor(() => {
      expect(mockedUserCreate).toHaveBeenCalledWith(
        expect.objectContaining({
          username: 'newuser',
          password: '123456',
          nickname: '新用户',
        })
      );
    });
  });

  it('opens edit user modal and submits', async () => {
    const user = userEvent.setup();
    const mockedUserList = vi.mocked(userApi.list);
    const mockedDeptList = vi.mocked(deptApi.list);
    const mockedRoleList = vi.mocked(roleApi.list);
    const mockedUserGet = vi.mocked(userApi.get);
    const mockedUserUpdate = vi.mocked(userApi.update);

    mockedUserList.mockResolvedValue({ total: 1, list: [mockUsers[0]], page: 1, page_size: 20 });
    mockedDeptList.mockResolvedValue(mockDepts);
    mockedRoleList.mockResolvedValue(mockRoles);
    mockedUserGet.mockResolvedValue({
      ...mockUsers[0],
      role_ids: [1],
    });
    mockedUserUpdate.mockResolvedValue(mockUsers[0]);

    renderUsersPage();

    await waitFor(() => {
      expect(screen.getByText('admin')).toBeInTheDocument();
    });

    await user.click(screen.getAllByText('编辑')[0]);

    await waitFor(() => {
      expect(screen.getByRole('dialog', { name: '编辑用户' })).toBeInTheDocument();
    });

    // Verify form is pre-filled
    expect(screen.getByDisplayValue('admin')).toBeInTheDocument();
    expect(screen.getByDisplayValue('管理员')).toBeInTheDocument();

    // Update nickname
    const nicknameInput = screen.getByLabelText('昵称');
    await user.clear(nicknameInput);
    await user.type(nicknameInput, '超级管理员');

    await user.click(screen.getByRole('button', { name: 'OK' }));

    await waitFor(() => {
      expect(mockedUserUpdate).toHaveBeenCalledWith(
        1,
        expect.objectContaining({
          nickname: '超级管理员',
        })
      );
    });
  });

  it('deletes a user', async () => {
    const user = userEvent.setup();
    const mockedUserList = vi.mocked(userApi.list);
    const mockedDeptList = vi.mocked(deptApi.list);
    const mockedRoleList = vi.mocked(roleApi.list);
    const mockedUserDelete = vi.mocked(userApi.delete);

    mockedUserList.mockResolvedValue({ total: 1, list: [mockUsers[1]], page: 1, page_size: 20 });
    mockedDeptList.mockResolvedValue(mockDepts);
    mockedRoleList.mockResolvedValue(mockRoles);
    mockedUserDelete.mockResolvedValue(undefined);

    renderUsersPage();

    await waitFor(() => {
      expect(screen.getByText('user01')).toBeInTheDocument();
    });

    await user.click(screen.getAllByText('删除')[0]);

    // Confirm in Popconfirm
    await waitFor(() => {
      expect(screen.getByText('确认删除 user01?')).toBeInTheDocument();
    });

    // Click the confirm button in the popconfirm (OK button)
    await user.click(screen.getByRole('button', { name: 'OK' }));

    await waitFor(() => {
      expect(mockedUserDelete).toHaveBeenCalledWith(2);
    });
  });

  it('resets password for a user', async () => {
    const user = userEvent.setup();
    const mockedUserList = vi.mocked(userApi.list);
    const mockedDeptList = vi.mocked(deptApi.list);
    const mockedRoleList = vi.mocked(roleApi.list);
    const mockedResetPassword = vi.mocked(userApi.resetPassword);

    mockedUserList.mockResolvedValue({ total: 1, list: [mockUsers[0]], page: 1, page_size: 20 });
    mockedDeptList.mockResolvedValue(mockDepts);
    mockedRoleList.mockResolvedValue(mockRoles);
    mockedResetPassword.mockResolvedValue(undefined);

    renderUsersPage();

    await waitFor(() => {
      expect(screen.getByText('admin')).toBeInTheDocument();
    });

    await user.click(screen.getAllByText('重置密码')[0]);

    await waitFor(() => {
      expect(screen.getByRole('dialog', { name: '重置密码 - admin' })).toBeInTheDocument();
    });

    const pwInput = screen.getByLabelText('新密码');
    await user.type(pwInput, 'newpass123');

    await user.click(screen.getByRole('button', { name: 'OK' }));

    await waitFor(() => {
      expect(mockedResetPassword).toHaveBeenCalledWith(1, 'newpass123');
    });
  });

  it('disables delete button for admin user (id=1)', async () => {
    const mockedUserList = vi.mocked(userApi.list);
    const mockedDeptList = vi.mocked(deptApi.list);
    const mockedRoleList = vi.mocked(roleApi.list);

    mockedUserList.mockResolvedValue({ total: 1, list: [mockUsers[0]], page: 1, page_size: 20 });
    mockedDeptList.mockResolvedValue(mockDepts);
    mockedRoleList.mockResolvedValue(mockRoles);

    renderUsersPage();

    await waitFor(() => {
      expect(screen.getByText('admin')).toBeInTheDocument();
    });

    // Find delete button in the row for admin (id=1) - it should be disabled
    const deleteBtn = screen.getAllByText('删除')[0].closest('button');
    expect(deleteBtn).toBeDisabled();
  });

  it('searches users by keyword', async () => {
    const user = userEvent.setup();
    const mockedUserList = vi.mocked(userApi.list);
    const mockedDeptList = vi.mocked(deptApi.list);
    const mockedRoleList = vi.mocked(roleApi.list);

    mockedUserList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });
    mockedDeptList.mockResolvedValue(mockDepts);
    mockedRoleList.mockResolvedValue(mockRoles);

    renderUsersPage();

    await waitFor(() => {
      expect(screen.getByPlaceholderText('搜索用户名/邮箱')).toBeInTheDocument();
    });

    const searchInput = screen.getByPlaceholderText('搜索用户名/邮箱');
    await user.type(searchInput, 'admin');
    await user.click(screen.getByRole('button', { name: 'search' }));

    await waitFor(() => {
      expect(mockedUserList).toHaveBeenCalledWith(
        expect.objectContaining({ q: 'admin', page: 1, page_size: 20 })
      );
    });
  });

  it('calls list API with correct pagination parameters', async () => {
    const mockedUserList = vi.mocked(userApi.list);
    const mockedDeptList = vi.mocked(deptApi.list);
    const mockedRoleList = vi.mocked(roleApi.list);

    mockedUserList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });
    mockedDeptList.mockResolvedValue(mockDepts);
    mockedRoleList.mockResolvedValue(mockRoles);

    renderUsersPage();

    await waitFor(() => {
      expect(mockedUserList).toHaveBeenCalledWith({ page: 1, page_size: 20, q: '' });
    });
  });

  it('handles partial API failures (dept/role fail but user list succeeds)', async () => {
    const mockedUserList = vi.mocked(userApi.list);
    const mockedDeptList = vi.mocked(deptApi.list);
    const mockedRoleList = vi.mocked(roleApi.list);

    mockedUserList.mockResolvedValue({ total: 1, list: [mockUsers[0]], page: 1, page_size: 20 });
    mockedDeptList.mockRejectedValue(new Error('Dept error'));
    mockedRoleList.mockRejectedValue(new Error('Role error'));

    renderUsersPage();

    await waitFor(() => {
      expect(screen.getByText('admin')).toBeInTheDocument();
    });

    // Dept name should fallback to '-' when depts fail to load
    expect(screen.getAllByText('-').length).toBeGreaterThan(0);
  });
});
