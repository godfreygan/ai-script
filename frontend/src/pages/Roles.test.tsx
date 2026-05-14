import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import RolesPage from './Roles';
import { roleApi } from '@/api/modules';

const messageMock = {
  success: vi.fn(),
  error: vi.fn(),
  warning: vi.fn(),
};

vi.mock('antd', async () => {
  const actual = await vi.importActual<typeof import('antd')>('antd');
  return {
    ...actual,
    App: {
      ...actual.App,
      useApp: () => ({ message: messageMock, modal: actual.App.useApp?.().modal || {} }),
    },
  };
});

vi.mock('@/api/modules', async () => {
  const actual = await vi.importActual<typeof import('@/api/modules')>('@/api/modules');
  return {
    ...actual,
    roleApi: {
      list: vi.fn(),
      get: vi.fn(),
      listPermissions: vi.fn(),
      create: vi.fn(),
      update: vi.fn(),
      delete: vi.fn(),
    },
  };
});

function renderRoles() {
  return render(<RolesPage />);
}

describe('RolesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders with title', async () => {
    const mockedList = vi.mocked(roleApi.list);
    const mockedListPerms = vi.mocked(roleApi.listPermissions);

    mockedList.mockResolvedValue([]);
    mockedListPerms.mockResolvedValue([]);

    renderRoles();

    await waitFor(() => {
      expect(screen.getByText('角色权限')).toBeInTheDocument();
    });
  });

  it('shows loading state initially', async () => {
    const mockedList = vi.mocked(roleApi.list);
    const mockedListPerms = vi.mocked(roleApi.listPermissions);

    mockedList.mockReturnValue(new Promise(() => {}));
    mockedListPerms.mockReturnValue(new Promise(() => {}));

    renderRoles();

    await waitFor(() => {
      expect(document.querySelector('[aria-busy="true"]')).toBeInTheDocument();
    });
  });

  it('displays empty state when no roles', async () => {
    const mockedList = vi.mocked(roleApi.list);
    const mockedListPerms = vi.mocked(roleApi.listPermissions);

    mockedList.mockResolvedValue([]);
    mockedListPerms.mockResolvedValue([]);

    renderRoles();

    await waitFor(() => {
      expect(screen.getByText('暂无数据')).toBeInTheDocument();
    });
  });

  it('displays role list with correct tags', async () => {
    const mockedList = vi.mocked(roleApi.list);
    const mockedListPerms = vi.mocked(roleApi.listPermissions);

    mockedList.mockResolvedValue([
      { id: 1, code: 'admin', name: '管理员', description: '系统管理员', data_scope: 'all', is_system: 1, status: 1 },
      { id: 2, code: 'editor', name: '编辑', description: '内容编辑', data_scope: 'dept', is_system: 0, status: 1 },
      { id: 3, code: 'viewer', name: '访客', description: '只读访客', data_scope: 'self', is_system: 0, status: 2 },
    ]);
    mockedListPerms.mockResolvedValue([]);

    renderRoles();

    await waitFor(() => {
      expect(screen.getByText('管理员')).toBeInTheDocument();
    });

    expect(screen.getByText('admin')).toBeInTheDocument();
    expect(screen.getByText('editor')).toBeInTheDocument();
    expect(screen.getByText('viewer')).toBeInTheDocument();
    expect(screen.getByText('系统')).toBeInTheDocument();
    expect(screen.getByText('自定义')).toBeInTheDocument();
    expect(screen.getByText('全部')).toBeInTheDocument();
    expect(screen.getByText('本部门')).toBeInTheDocument();
    expect(screen.getByText('自己')).toBeInTheDocument();
  });

  it('opens create modal when clicking 新建角色', async () => {
    const user = userEvent.setup();
    const mockedList = vi.mocked(roleApi.list);
    const mockedListPerms = vi.mocked(roleApi.listPermissions);

    mockedList.mockResolvedValue([]);
    mockedListPerms.mockResolvedValue([]);

    renderRoles();

    await waitFor(() => {
      expect(screen.getByText('新建角色')).toBeInTheDocument();
    });

    await user.click(screen.getByText('新建角色'));

    await waitFor(() => {
      expect(screen.getByText('新建角色')).toBeInTheDocument();
    });

    expect(screen.getByLabelText('编码')).toBeInTheDocument();
    expect(screen.getByLabelText('名称')).toBeInTheDocument();
    expect(screen.getByLabelText('描述')).toBeInTheDocument();
    expect(screen.getByLabelText('数据范围')).toBeInTheDocument();
  });

  it('creates a new role successfully', async () => {
    const user = userEvent.setup();
    const mockedList = vi.mocked(roleApi.list);
    const mockedListPerms = vi.mocked(roleApi.listPermissions);
    const mockedCreate = vi.mocked(roleApi.create);

    mockedList.mockResolvedValue([]);
    mockedListPerms.mockResolvedValue([]);
    mockedCreate.mockResolvedValue({
      id: 10, code: 'producer', name: '制片人', description: '项目制片人', data_scope: 'all', is_system: 0, status: 1,
    });

    renderRoles();

    await waitFor(() => {
      expect(screen.getByText('新建角色')).toBeInTheDocument();
    });

    await user.click(screen.getByText('新建角色'));

    await waitFor(() => {
      expect(screen.getByLabelText('编码')).toBeInTheDocument();
    });

    await user.type(screen.getByLabelText('编码'), 'producer');
    await user.type(screen.getByLabelText('名称'), '制片人');
    await user.type(screen.getByLabelText('描述'), '项目制片人');

    await user.click(screen.getByRole('button', { name: /确定/i }));

    await waitFor(() => {
      expect(mockedCreate).toHaveBeenCalledWith(
        expect.objectContaining({ code: 'producer', name: '制片人' })
      );
    });
  });

  it('opens edit modal with pre-filled values', async () => {
    const user = userEvent.setup();
    const mockedList = vi.mocked(roleApi.list);
    const mockedListPerms = vi.mocked(roleApi.listPermissions);
    const mockedGet = vi.mocked(roleApi.get);

    mockedList.mockResolvedValue([
      { id: 1, code: 'admin', name: '管理员', description: '系统管理员', data_scope: 'all', is_system: 1, status: 1 },
    ]);
    mockedListPerms.mockResolvedValue([]);
    mockedGet.mockResolvedValue({
      id: 1, code: 'admin', name: '管理员', description: '系统管理员', data_scope: 'all', is_system: 1, status: 1,
      permissions: ['user:read', 'user:write'],
    });

    renderRoles();

    await waitFor(() => {
      expect(screen.getByText('管理员')).toBeInTheDocument();
    });

    await user.click(screen.getByText('编辑'));

    await waitFor(() => {
      expect(screen.getByText('编辑角色')).toBeInTheDocument();
    });

    const codeInput = screen.getByLabelText('编码') as HTMLInputElement;
    expect(codeInput.value).toBe('admin');
  });

  it('updates a role successfully', async () => {
    const user = userEvent.setup();
    const mockedList = vi.mocked(roleApi.list);
    const mockedListPerms = vi.mocked(roleApi.listPermissions);
    const mockedGet = vi.mocked(roleApi.get);
    const mockedUpdate = vi.mocked(roleApi.update);

    mockedList.mockResolvedValue([
      { id: 2, code: 'editor', name: '编辑', description: '内容编辑', data_scope: 'dept', is_system: 0, status: 1 },
    ]);
    mockedListPerms.mockResolvedValue([]);
    mockedGet.mockResolvedValue({
      id: 2, code: 'editor', name: '编辑', description: '内容编辑', data_scope: 'dept', is_system: 0, status: 1,
      permissions: [],
    });
    mockedUpdate.mockResolvedValue({
      id: 2, code: 'editor', name: '高级编辑', description: '高级内容编辑', data_scope: 'all', is_system: 0, status: 1,
    });

    renderRoles();

    await waitFor(() => {
      expect(screen.getByText('编辑')).toBeInTheDocument();
    });

    await user.click(screen.getAllByText('编辑')[0]);

    await waitFor(() => {
      expect(screen.getByText('编辑角色')).toBeInTheDocument();
    });

    const nameInput = screen.getByLabelText('名称');
    await user.clear(nameInput);
    await user.type(nameInput, '高级编辑');

    await user.click(screen.getByRole('button', { name: /确定/i }));

    await waitFor(() => {
      expect(mockedUpdate).toHaveBeenCalledWith(
        2,
        expect.objectContaining({ name: '高级编辑' })
      );
    });
  });

  it('deletes a custom role after confirmation', async () => {
    const user = userEvent.setup();
    const mockedList = vi.mocked(roleApi.list);
    const mockedListPerms = vi.mocked(roleApi.listPermissions);
    const mockedDelete = vi.mocked(roleApi.delete);

    mockedList.mockResolvedValue([
      { id: 2, code: 'editor', name: '编辑', description: '', data_scope: 'dept', is_system: 0, status: 1 },
    ]);
    mockedListPerms.mockResolvedValue([]);
    mockedDelete.mockResolvedValue(undefined);

    renderRoles();

    await waitFor(() => {
      expect(screen.getByText('编辑')).toBeInTheDocument();
    });

    await user.click(screen.getByText('删除'));

    await waitFor(() => {
      expect(screen.getByText(/确认删除/)).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: /确定/i }));

    await waitFor(() => {
      expect(mockedDelete).toHaveBeenCalledWith(2);
    });
  });

  it('prevents deleting system role and shows warning', async () => {
    const mockedList = vi.mocked(roleApi.list);
    const mockedListPerms = vi.mocked(roleApi.listPermissions);

    mockedList.mockResolvedValue([
      { id: 1, code: 'admin', name: '管理员', description: '', data_scope: 'all', is_system: 1, status: 1 },
    ]);
    mockedListPerms.mockResolvedValue([]);

    renderRoles();

    await waitFor(() => {
      expect(screen.getByText('管理员')).toBeInTheDocument();
    });

    const deleteBtn = screen.getByText('删除').closest('button');
    expect(deleteBtn).toBeDisabled();
  });

  it('displays permission matrix in modal', async () => {
    const user = userEvent.setup();
    const mockedList = vi.mocked(roleApi.list);
    const mockedListPerms = vi.mocked(roleApi.listPermissions);

    mockedList.mockResolvedValue([]);
    mockedListPerms.mockResolvedValue([
      { id: 1, code: 'user:read', name: '查看用户', resource: 'user', action: 'read' },
      { id: 2, code: 'user:write', name: '编辑用户', resource: 'user', action: 'write' },
      { id: 3, code: 'dept:read', name: '查看部门', resource: 'dept', action: 'read' },
    ]);

    renderRoles();

    await waitFor(() => {
      expect(screen.getByText('新建角色')).toBeInTheDocument();
    });

    await user.click(screen.getByText('新建角色'));

    await waitFor(() => {
      expect(screen.getByText('user')).toBeInTheDocument();
    });

    expect(screen.getByText('查看用户')).toBeInTheDocument();
    expect(screen.getByText('编辑用户')).toBeInTheDocument();
    expect(screen.getByText('dept')).toBeInTheDocument();
    expect(screen.getByText('查看部门')).toBeInTheDocument();
  });

  it('handles API list error gracefully', async () => {
    const mockedList = vi.mocked(roleApi.list);
    const mockedListPerms = vi.mocked(roleApi.listPermissions);

    mockedList.mockRejectedValue(new Error('Network error'));
    mockedListPerms.mockRejectedValue(new Error('Network error'));

    renderRoles();

    await waitFor(() => {
      expect(screen.getByText('角色权限')).toBeInTheDocument();
    });
  });

  it('handles create API error gracefully', async () => {
    const user = userEvent.setup();
    const mockedList = vi.mocked(roleApi.list);
    const mockedListPerms = vi.mocked(roleApi.listPermissions);
    const mockedCreate = vi.mocked(roleApi.create);

    mockedList.mockResolvedValue([]);
    mockedListPerms.mockResolvedValue([]);
    mockedCreate.mockRejectedValue(new Error('Create failed'));

    renderRoles();

    await waitFor(() => {
      expect(screen.getByText('新建角色')).toBeInTheDocument();
    });

    await user.click(screen.getByText('新建角色'));

    await waitFor(() => {
      expect(screen.getByLabelText('编码')).toBeInTheDocument();
    });

    await user.type(screen.getByLabelText('编码'), 'fail');
    await user.type(screen.getByLabelText('名称'), '失败角色');
    await user.click(screen.getByRole('button', { name: /确定/i }));

    await waitFor(() => {
      expect(mockedCreate).toHaveBeenCalled();
    });
  });

  it('handles edit get API error gracefully', async () => {
    const user = userEvent.setup();
    const mockedList = vi.mocked(roleApi.list);
    const mockedListPerms = vi.mocked(roleApi.listPermissions);
    const mockedGet = vi.mocked(roleApi.get);

    mockedList.mockResolvedValue([
      { id: 1, code: 'admin', name: '管理员', description: '', data_scope: 'all', is_system: 1, status: 1 },
    ]);
    mockedListPerms.mockResolvedValue([]);
    mockedGet.mockRejectedValue(new Error('Get failed'));

    renderRoles();

    await waitFor(() => {
      expect(screen.getByText('管理员')).toBeInTheDocument();
    });

    await user.click(screen.getByText('编辑'));

    await waitFor(() => {
      expect(mockedGet).toHaveBeenCalledWith(1);
    });
  });
});
