import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import DeptsPage from './Depts';
import { deptApi } from '@/api/modules';

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
    deptApi: {
      list: vi.fn(),
      create: vi.fn(),
      update: vi.fn(),
      delete: vi.fn(),
    },
  };
});

function renderDepts() {
  return render(<DeptsPage />);
}

describe('DeptsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders with title', async () => {
    const mockedList = vi.mocked(deptApi.list);
    mockedList.mockResolvedValue([]);

    renderDepts();

    await waitFor(() => {
      expect(screen.getByText('部门管理')).toBeInTheDocument();
    });
  });

  it('shows loading state initially', async () => {
    const mockedList = vi.mocked(deptApi.list);
    mockedList.mockReturnValue(new Promise(() => {}));

    renderDepts();

    await waitFor(() => {
      expect(document.querySelector('[aria-busy="true"]')).toBeInTheDocument();
    });
  });

  it('displays empty state when no departments', async () => {
    const mockedList = vi.mocked(deptApi.list);
    mockedList.mockResolvedValue([]);

    renderDepts();

    await waitFor(() => {
      expect(screen.getByText('暂无数据')).toBeInTheDocument();
    });
  });

  it('displays department list in tree table', async () => {
    const mockedList = vi.mocked(deptApi.list);
    mockedList.mockResolvedValue([
      { id: 1, name: '技术部', parent_id: 0, path: '技术部', sort: 1, status: 1 },
      { id: 2, name: '前端组', parent_id: 1, path: '技术部/前端组', sort: 1, status: 1 },
      { id: 3, name: '后端组', parent_id: 1, path: '技术部/后端组', sort: 2, status: 2 },
    ]);

    renderDepts();

    await waitFor(() => {
      expect(screen.getByText('技术部')).toBeInTheDocument();
    });

    expect(screen.getByText('前端组')).toBeInTheDocument();
    expect(screen.getByText('后端组')).toBeInTheDocument();
    expect(screen.getByText('技术部/前端组')).toBeInTheDocument();
    expect(screen.getByText('技术部/后端组')).toBeInTheDocument();
  });

  it('shows status tags correctly', async () => {
    const mockedList = vi.mocked(deptApi.list);
    mockedList.mockResolvedValue([
      { id: 1, name: '启用部门', parent_id: 0, path: '启用部门', sort: 0, status: 1 },
      { id: 2, name: '禁用部门', parent_id: 0, path: '禁用部门', sort: 0, status: 2 },
    ]);

    renderDepts();

    await waitFor(() => {
      expect(screen.getByText('启用')).toBeInTheDocument();
    });

    expect(screen.getByText('禁用')).toBeInTheDocument();
  });

  it('opens create modal when clicking 新建部门', async () => {
    const user = userEvent.setup();
    const mockedList = vi.mocked(deptApi.list);
    mockedList.mockResolvedValue([]);

    renderDepts();

    await waitFor(() => {
      expect(screen.getByText('新建部门')).toBeInTheDocument();
    });

    await user.click(screen.getByText('新建部门'));

    await waitFor(() => {
      expect(screen.getByText('新建部门')).toBeInTheDocument();
    });

    expect(screen.getByLabelText('名称')).toBeInTheDocument();
    expect(screen.getByLabelText('上级部门')).toBeInTheDocument();
    expect(screen.getByLabelText('排序')).toBeInTheDocument();
  });

  it('creates a new department successfully', async () => {
    const user = userEvent.setup();
    const mockedList = vi.mocked(deptApi.list);
    const mockedCreate = vi.mocked(deptApi.create);

    mockedList.mockResolvedValue([]);
    mockedCreate.mockResolvedValue({ id: 10, name: '新产品部', parent_id: 0, path: '新产品部', sort: 0, status: 1 });

    renderDepts();

    await waitFor(() => {
      expect(screen.getByText('新建部门')).toBeInTheDocument();
    });

    await user.click(screen.getByText('新建部门'));

    await waitFor(() => {
      expect(screen.getByLabelText('名称')).toBeInTheDocument();
    });

    await user.type(screen.getByLabelText('名称'), '新产品部');
    await user.click(screen.getByRole('button', { name: /确定/i }));

    await waitFor(() => {
      expect(mockedCreate).toHaveBeenCalledWith(
        expect.objectContaining({ name: '新产品部' })
      );
    });
  });

  it('opens edit modal with pre-filled values', async () => {
    const user = userEvent.setup();
    const mockedList = vi.mocked(deptApi.list);

    mockedList.mockResolvedValue([
      { id: 1, name: '技术部', parent_id: 0, path: '技术部', sort: 5, status: 1 },
    ]);

    renderDepts();

    await waitFor(() => {
      expect(screen.getByText('技术部')).toBeInTheDocument();
    });

    await user.click(screen.getByText('编辑'));

    await waitFor(() => {
      expect(screen.getByText('编辑部门 - 技术部')).toBeInTheDocument();
    });

    const nameInput = screen.getByLabelText('名称') as HTMLInputElement;
    expect(nameInput.value).toBe('技术部');
  });

  it('updates a department successfully', async () => {
    const user = userEvent.setup();
    const mockedList = vi.mocked(deptApi.list);
    const mockedUpdate = vi.mocked(deptApi.update);

    mockedList.mockResolvedValue([
      { id: 1, name: '技术部', parent_id: 0, path: '技术部', sort: 5, status: 1 },
    ]);
    mockedUpdate.mockResolvedValue({ id: 1, name: '研发部', parent_id: 0, path: '研发部', sort: 5, status: 1 });

    renderDepts();

    await waitFor(() => {
      expect(screen.getByText('技术部')).toBeInTheDocument();
    });

    await user.click(screen.getByText('编辑'));

    await waitFor(() => {
      expect(screen.getByText('编辑部门 - 技术部')).toBeInTheDocument();
    });

    const nameInput = screen.getByLabelText('名称');
    await user.clear(nameInput);
    await user.type(nameInput, '研发部');

    await user.click(screen.getByRole('button', { name: /确定/i }));

    await waitFor(() => {
      expect(mockedUpdate).toHaveBeenCalledWith(
        1,
        expect.objectContaining({ name: '研发部' })
      );
    });
  });

  it('deletes a department after confirmation', async () => {
    const user = userEvent.setup();
    const mockedList = vi.mocked(deptApi.list);
    const mockedDelete = vi.mocked(deptApi.delete);

    mockedList.mockResolvedValue([
      { id: 1, name: '待删部门', parent_id: 0, path: '待删部门', sort: 0, status: 1 },
    ]);
    mockedDelete.mockResolvedValue(undefined);

    renderDepts();

    await waitFor(() => {
      expect(screen.getByText('待删部门')).toBeInTheDocument();
    });

    await user.click(screen.getByText('删除'));

    await waitFor(() => {
      expect(screen.getByText(/确认删除/)).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: /确定/i }));

    await waitFor(() => {
      expect(mockedDelete).toHaveBeenCalledWith(1);
    });
  });

  it('opens create child department modal', async () => {
    const user = userEvent.setup();
    const mockedList = vi.mocked(deptApi.list);

    mockedList.mockResolvedValue([
      { id: 1, name: '技术部', parent_id: 0, path: '技术部', sort: 0, status: 1 },
    ]);

    renderDepts();

    await waitFor(() => {
      expect(screen.getByText('技术部')).toBeInTheDocument();
    });

    await user.click(screen.getByText('子部门'));

    await waitFor(() => {
      expect(screen.getByText('新建部门')).toBeInTheDocument();
    });
  });

  it('handles API list error gracefully', async () => {
    const mockedList = vi.mocked(deptApi.list);
    mockedList.mockRejectedValue(new Error('Network error'));

    renderDepts();

    await waitFor(() => {
      expect(screen.getByText('部门管理')).toBeInTheDocument();
    });
  });

  it('handles create API error gracefully', async () => {
    const user = userEvent.setup();
    const mockedList = vi.mocked(deptApi.list);
    const mockedCreate = vi.mocked(deptApi.create);

    mockedList.mockResolvedValue([]);
    mockedCreate.mockRejectedValue(new Error('Create failed'));

    renderDepts();

    await waitFor(() => {
      expect(screen.getByText('新建部门')).toBeInTheDocument();
    });

    await user.click(screen.getByText('新建部门'));

    await waitFor(() => {
      expect(screen.getByLabelText('名称')).toBeInTheDocument();
    });

    await user.type(screen.getByLabelText('名称'), '失败部门');
    await user.click(screen.getByRole('button', { name: /确定/i }));

    await waitFor(() => {
      expect(mockedCreate).toHaveBeenCalled();
    });
  });

  it('handles delete API error gracefully', async () => {
    const user = userEvent.setup();
    const mockedList = vi.mocked(deptApi.list);
    const mockedDelete = vi.mocked(deptApi.delete);

    mockedList.mockResolvedValue([
      { id: 1, name: '删不掉', parent_id: 0, path: '删不掉', sort: 0, status: 1 },
    ]);
    mockedDelete.mockRejectedValue(new Error('Delete failed'));

    renderDepts();

    await waitFor(() => {
      expect(screen.getByText('删不掉')).toBeInTheDocument();
    });

    await user.click(screen.getByText('删除'));

    await waitFor(() => {
      expect(screen.getByText(/确认删除/)).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: /确定/i }));

    await waitFor(() => {
      expect(mockedDelete).toHaveBeenCalledWith(1);
    });
  });
});
