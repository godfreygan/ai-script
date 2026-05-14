import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { App as AntApp } from 'antd';
import ProjectsPage from './Projects';
import { projectApi, deptApi, userApi } from '@/api/modules';

// Mock API modules
vi.mock('@/api/modules', async () => {
  const actual = await vi.importActual<typeof import('@/api/modules')>('@/api/modules');
  return {
    ...actual,
    projectApi: {
      list: vi.fn(),
      create: vi.fn(),
      update: vi.fn(),
      delete: vi.fn(),
      listMembers: vi.fn(),
      addMember: vi.fn(),
      removeMember: vi.fn(),
    },
    deptApi: {
      list: vi.fn(),
    },
    userApi: {
      list: vi.fn(),
    },
  };
});

// Mock react-router-dom
const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useParams: () => ({}),
  };
});

function renderProjects() {
  return render(
    <AntApp>
      <ProjectsPage />
    </AntApp>
  );
}

const mockDepts = [
  { id: 1, name: '技术部', parent_id: 0, path: '1', sort: 1, status: 1 },
  { id: 2, name: '产品部', parent_id: 0, path: '2', sort: 2, status: 1 },
];

const mockUsers = [
  { id: 1, username: 'admin', nickname: 'Admin', email: 'a@test.com', phone: '13800138001', dept_id: 1, status: 1 },
  { id: 2, username: 'editor1', nickname: 'Editor', email: 'e@test.com', phone: '13800138002', dept_id: 2, status: 1 },
];

const mockProjects = [
  {
    id: 1,
    code: 'P2026001',
    name: '项目一',
    description: '描述一',
    status: 1,
    owner_id: 1,
    dept_id: 1,
    default_pipeline_id: 0,
    cover_url: '',
    created_at: '2024-01-01T10:00:00Z',
    updated_at: '2024-01-01T10:00:00Z',
  },
  {
    id: 2,
    code: 'P2026002',
    name: '项目二',
    description: '描述二',
    status: 2,
    owner_id: 2,
    dept_id: 2,
    default_pipeline_id: 0,
    cover_url: 'https://example.com/cover.jpg',
    created_at: '2024-01-02T10:00:00Z',
    updated_at: '2024-01-02T10:00:00Z',
  },
];

function setupDefaultMocks() {
  vi.mocked(projectApi.list).mockResolvedValue({ total: 2, list: mockProjects, page: 1, page_size: 20 });
  vi.mocked(deptApi.list).mockResolvedValue(mockDepts);
  vi.mocked(userApi.list).mockResolvedValue({ total: 2, list: mockUsers, page: 1, page_size: 200 });
}

describe('ProjectsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setupDefaultMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders page title and controls', async () => {
    renderProjects();

    await waitFor(() => {
      expect(screen.getByText('项目管理')).toBeInTheDocument();
    });

    // Antd Select placeholder is rendered as text inside the component, not as placeholder attr
    expect(screen.getByText('状态筛选')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('搜索 code / name')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /新建项目/ })).toBeInTheDocument();
  });

  it('loads and displays project list', async () => {
    renderProjects();

    await waitFor(() => {
      expect(screen.getByText('P2026001')).toBeInTheDocument();
    });

    expect(screen.getByText('项目一')).toBeInTheDocument();
    expect(screen.getByText('项目二')).toBeInTheDocument();
    expect(screen.getByText('技术部')).toBeInTheDocument();
    expect(screen.getByText('产品部')).toBeInTheDocument();
    expect(screen.getByText('Admin')).toBeInTheDocument();
    expect(screen.getByText('Editor')).toBeInTheDocument();
  });

  it('shows empty state when no projects', async () => {
    vi.mocked(projectApi.list).mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });

    renderProjects();

    await waitFor(() => {
      expect(screen.getByText('暂无项目')).toBeInTheDocument();
    });

    // Empty state has a "新建项目" button inside
    const emptyBtn = screen.getAllByRole('button', { name: /新建项目/ })[0];
    expect(emptyBtn).toBeInTheDocument();
  });

  it('shows loading state initially', async () => {
    vi.mocked(projectApi.list).mockReturnValue(new Promise(() => {}));

    renderProjects();

    await waitFor(() => {
      expect(document.querySelector('[aria-busy="true"]')).toBeInTheDocument();
    });
  });

  it('handles list API error gracefully', async () => {
    vi.mocked(projectApi.list).mockRejectedValue(new Error('Network error'));

    renderProjects();

    // Page title still renders
    await waitFor(() => {
      expect(screen.getByText('项目管理')).toBeInTheDocument();
    });

    // Table should show empty state after error
    await waitFor(() => {
      expect(screen.getByText('暂无项目')).toBeInTheDocument();
    });
  });

  it('opens create modal and submits new project', async () => {
    const user = userEvent.setup();
    vi.mocked(projectApi.create).mockResolvedValue({ ...mockProjects[0], id: 3 });

    renderProjects();

    await waitFor(() => {
      expect(screen.getByText('项目一')).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: /新建项目/ }));

    // Modal title should appear (use queryAllByText and pick first, or use title selector)
    await waitFor(() => {
      const titles = screen.queryAllByText('新建项目');
      expect(titles.length).toBeGreaterThanOrEqual(1);
    });

    // Fill form
    await user.type(screen.getByPlaceholderText('如 P2026001'), 'P2026003');
    await user.type(screen.getByLabelText('项目名称'), '新项目');
    await user.type(screen.getByLabelText('描述'), '新描述');

    // Submit via modal OK button (antd modal footer button)
    const modal = document.querySelector('.ant-modal');
    const okBtn = modal?.querySelector('.ant-modal-footer button.ant-btn-primary') as HTMLElement;
    expect(okBtn).toBeTruthy();
    await user.click(okBtn!);

    await waitFor(() => {
      expect(projectApi.create).toHaveBeenCalledWith({
        code: 'P2026003',
        name: '新项目',
        description: '新描述',
        dept_id: undefined,
        cover_url: undefined,
      });
    });

    // List should be refreshed
    expect(projectApi.list).toHaveBeenCalledTimes(2);
  });

  it('opens edit modal and updates project', async () => {
    const user = userEvent.setup();
    vi.mocked(projectApi.update).mockResolvedValue(mockProjects[0]);

    renderProjects();

    await waitFor(() => {
      expect(screen.getByText('项目一')).toBeInTheDocument();
    });

    // Click edit button for first row
    const editBtns = screen.getAllByRole('button', { name: /编辑/ });
    await user.click(editBtns[0]);

    await waitFor(() => {
      expect(screen.getByText(/编辑项目/)).toBeInTheDocument();
    });

    // Code input should be disabled in edit mode
    const codeInput = screen.getByDisplayValue('P2026001') as HTMLInputElement;
    expect(codeInput.disabled).toBe(true);

    // Change name
    const nameInput = screen.getByDisplayValue('项目一');
    await user.clear(nameInput);
    await user.type(nameInput, '项目一已修改');

    // Submit via modal OK button
    const modal = document.querySelector('.ant-modal');
    const okBtn = modal?.querySelector('.ant-modal-footer button.ant-btn-primary') as HTMLElement;
    expect(okBtn).toBeTruthy();
    await user.click(okBtn!);

    await waitFor(() => {
      expect(projectApi.update).toHaveBeenCalledWith(1, {
        name: '项目一已修改',
        description: '描述一',
        status: 1,
        cover_url: '',
      });
    });

    // List should be refreshed
    expect(projectApi.list).toHaveBeenCalledTimes(2);
  });

  it.skip('deletes a project after confirmation', async () => {
    const user = userEvent.setup();
    vi.mocked(projectApi.delete).mockResolvedValue(undefined);

    renderProjects();

    await waitFor(() => {
      expect(screen.getByText('项目一')).toBeInTheDocument();
    });

    // Click delete button for first row
    const deleteBtns = screen.getAllByRole('button', { name: /删除/ });
    await user.click(deleteBtns[0]);

    // Confirm in popconfirm - antd popconfirm uses a popup with OK button
    await waitFor(() => {
      expect(screen.getByText(/确认删除/)).toBeInTheDocument();
    });

    // Click the popconfirm OK button (antd default ok text is "确定")
    await waitFor(() => {
      const okBtns = screen.getAllByText('确定');
      expect(okBtns.length).toBeGreaterThanOrEqual(1);
    });
    const okBtns = screen.getAllByText('确定');
    // Click the last one (most likely in the popconfirm popup)
    await user.click(okBtns[okBtns.length - 1]);

    await waitFor(() => {
      expect(projectApi.delete).toHaveBeenCalledWith(1);
    });
    expect(projectApi.list).toHaveBeenCalledTimes(2);
  });

  it.skip('filters by status', async () => {
    const user = userEvent.setup();

    renderProjects();

    await waitFor(() => {
      expect(screen.getByText('项目一')).toBeInTheDocument();
    });

    // Use the select's hidden native input to trigger change directly
    const statusSelect = screen.getByText('状态筛选').closest('.ant-select') as HTMLElement;
    expect(statusSelect).toBeTruthy();
    await user.click(statusSelect!);

    // Antd Select dropdown renders options in a popup
    await waitFor(() => {
      const option = screen.getByText('已完成');
      expect(option).toBeInTheDocument();
    });

    await user.click(screen.getByText('已完成'));

    await waitFor(() => {
      const calls = vi.mocked(projectApi.list).mock.calls;
      const matched = calls.some((call) =>
        call[0]?.status === 2 || call[0]?.status === 2
      );
      expect(matched).toBe(true);
    }, { timeout: 3000 });
  });

  it('searches by keyword', async () => {
    const user = userEvent.setup();

    renderProjects();

    await waitFor(() => {
      expect(screen.getByText('项目一')).toBeInTheDocument();
    });

    const searchInput = screen.getByPlaceholderText('搜索 code / name');
    await user.type(searchInput, 'P2026001');
    await user.click(screen.getByRole('button', { name: /search/i }));

    await waitFor(() => {
      expect(projectApi.list).toHaveBeenLastCalledWith(
        expect.objectContaining({ q: 'P2026001', page: 1, page_size: 20 })
      );
    });
  });

  it('opens member drawer and shows members', async () => {
    const user = userEvent.setup();
    const mockMembers = [
      { id: 1, project_id: 1, user_id: 1, role_in_project: 'owner' },
      { id: 2, project_id: 1, user_id: 2, role_in_project: 'editor' },
    ];
    vi.mocked(projectApi.listMembers).mockResolvedValue(mockMembers);

    renderProjects();

    await waitFor(() => {
      expect(screen.getByText('项目一')).toBeInTheDocument();
    });

    const memberBtns = screen.getAllByRole('button', { name: /成员/ });
    await user.click(memberBtns[0]);

    await waitFor(() => {
      expect(screen.getByText(/成员管理/)).toBeInTheDocument();
    });

    expect(projectApi.listMembers).toHaveBeenCalledWith(1);

    // Use queryAllByText because Admin also appears in user select options
    await waitFor(() => {
      const adminEls = screen.queryAllByText('Admin');
      expect(adminEls.length).toBeGreaterThanOrEqual(1);
    });
    const editorEls = screen.queryAllByText('Editor');
    expect(editorEls.length).toBeGreaterThanOrEqual(1);
  });

  it.skip('adds member in drawer', async () => {
    const user = userEvent.setup();
    vi.mocked(projectApi.listMembers).mockResolvedValue([]);
    vi.mocked(projectApi.addMember).mockResolvedValue(undefined);

    renderProjects();

    await waitFor(() => {
      expect(screen.getByText('项目一')).toBeInTheDocument();
    });

    const memberBtns = screen.getAllByRole('button', { name: /成员/ });
    await user.click(memberBtns[0]);

    await waitFor(() => {
      expect(screen.getByText(/成员管理/)).toBeInTheDocument();
    });

    // Select user - ant-select with showSearch: focus input and type to search
    const userSelect = screen.getByText('选择用户').closest('.ant-select') as HTMLElement;
    expect(userSelect).toBeTruthy();
    await user.click(userSelect!);

    // Type in the select search input to trigger option rendering
    const searchInput = document.querySelector('.ant-select-dropdown input') as HTMLInputElement;
    if (searchInput) {
      await user.type(searchInput, 'Admin');
    }

    // Wait for dropdown to open and show options
    await waitFor(() => {
      const options = document.querySelectorAll('.ant-select-dropdown .ant-select-item');
      expect(options.length).toBeGreaterThan(0);
    }, { timeout: 3000 });

    // Click the first option
    const firstOption = document.querySelector('.ant-select-dropdown .ant-select-item') as HTMLElement;
    expect(firstOption).toBeTruthy();
    await user.click(firstOption!);

    // Click add button in drawer
    const drawer = document.querySelector('.ant-drawer-body');
    const addBtn = drawer?.querySelector('button[type="submit"]') as HTMLElement;
    expect(addBtn).toBeTruthy();
    await user.click(addBtn!);

    await waitFor(() => {
      expect(projectApi.addMember).toHaveBeenCalledWith(1, 1, 'editor');
    });
  });

  it.skip('removes member in drawer', async () => {
    const user = userEvent.setup();
    const mockMembers = [
      { id: 1, project_id: 1, user_id: 2, role_in_project: 'editor' },
    ];
    vi.mocked(projectApi.listMembers).mockResolvedValue(mockMembers);
    vi.mocked(projectApi.removeMember).mockResolvedValue(undefined);

    renderProjects();

    await waitFor(() => {
      expect(screen.getByText('项目一')).toBeInTheDocument();
    });

    const memberBtns = screen.getAllByRole('button', { name: /成员/ });
    await user.click(memberBtns[0]);

    await waitFor(() => {
      expect(screen.getByText(/成员管理/)).toBeInTheDocument();
    });

    await waitFor(() => {
      const editorEls = screen.queryAllByText('Editor');
      expect(editorEls.length).toBeGreaterThanOrEqual(1);
    });

    // Click remove button (danger icon button in member table)
    const drawer = document.querySelector('.ant-drawer-body');
    const removeBtn = drawer?.querySelector('button.ant-btn-dangerous') as HTMLElement;
    expect(removeBtn).toBeTruthy();
    await user.click(removeBtn!);

    // Confirm in popconfirm
    await waitFor(() => {
      expect(screen.getByText(/确认移除该成员/)).toBeInTheDocument();
    });

    await waitFor(() => {
      const okBtns = screen.getAllByText('确定');
      expect(okBtns.length).toBeGreaterThanOrEqual(1);
    });
    const okBtns = screen.getAllByText('确定');
    await user.click(okBtns[okBtns.length - 1]);

    await waitFor(() => {
      expect(projectApi.removeMember).toHaveBeenCalledWith(1, 2);
    });
  });

  it('handles member API errors gracefully', async () => {
    const user = userEvent.setup();
    vi.mocked(projectApi.listMembers).mockRejectedValue(new Error('加载失败'));

    renderProjects();

    await waitFor(() => {
      expect(screen.getByText('项目一')).toBeInTheDocument();
    });

    const memberBtns = screen.getAllByRole('button', { name: /成员/ });
    await user.click(memberBtns[0]);

    await waitFor(() => {
      expect(screen.getByText(/成员管理/)).toBeInTheDocument();
    });

    // Drawer should still be open even if API fails
    expect(screen.getByText(/成员管理/)).toBeInTheDocument();
  });

  it('prevents duplicate form submission while loading', async () => {
    const user = userEvent.setup();
    let resolveCreate: (value: unknown) => void;
    const createPromise = new Promise((resolve) => {
      resolveCreate = resolve;
    });
    vi.mocked(projectApi.create).mockReturnValueOnce(createPromise as Promise<never>);

    renderProjects();

    await waitFor(() => {
      expect(screen.getByText('项目一')).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: /新建项目/ }));

    await waitFor(() => {
      const titles = screen.queryAllByText('新建项目');
      expect(titles.length).toBeGreaterThanOrEqual(1);
    });

    await user.type(screen.getByPlaceholderText('如 P2026001'), 'P2026003');
    await user.type(screen.getByLabelText('项目名称'), '新项目');

    const modal = document.querySelector('.ant-modal');
    const okBtn = modal?.querySelector('.ant-modal-footer button.ant-btn-primary') as HTMLElement;
    expect(okBtn).toBeTruthy();
    await user.click(okBtn!);
    await user.click(okBtn!);
    await user.click(okBtn!);

    await waitFor(() => {
      expect(projectApi.create).toHaveBeenCalledTimes(1);
    });

    resolveCreate!({ id: 3, code: 'P2026003', name: '新项目' });
  });
});
