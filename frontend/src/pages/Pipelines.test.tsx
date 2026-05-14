import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import PipelinesPage from './Pipelines';
import { pipelineApi, projectApi } from '@/api/modules';

// Mock API modules
vi.mock('@/api/modules', async () => {
  const actual = await vi.importActual<typeof import('@/api/modules')>('@/api/modules');
  return {
    ...actual,
    pipelineApi: {
      list: vi.fn(),
      get: vi.fn(),
      create: vi.fn(),
      update: vi.fn(),
      delete: vi.fn(),
      run: vi.fn(),
      listRuns: vi.fn(),
      getRun: vi.fn(),
      listSteps: vi.fn(),
    },
    projectApi: {
      list: vi.fn(),
    },
  };
});

// Mock useProgressWS hook
vi.mock('@/hooks/useProgressWS', () => ({
  useProgressWS: () => ({
    last: null,
    connected: false,
    events: [],
  }),
}));

// Mock ReactFlow (canvas is hard to test in jsdom)
vi.mock('reactflow', async () => {
  const actual = await vi.importActual<typeof import('reactflow')>('reactflow');
  return {
    ...actual,
    default: () => <div data-testid="react-flow">ReactFlow</div>,
    ReactFlowProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  };
});

// Mock AntApp.useApp
vi.mock('antd', async () => {
  const actual = await vi.importActual<typeof import('antd')>('antd');
  return {
    ...actual,
    App: {
      useApp: () => ({
        message: {
          success: vi.fn(),
          error: vi.fn(),
          warning: vi.fn(),
        },
        modal: {
          confirm: vi.fn(),
        },
        notification: {
          success: vi.fn(),
          error: vi.fn(),
        },
      }),
    },
  };
});

function renderPipelines() {
  return render(<PipelinesPage />);
}

describe('PipelinesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders pipelines page with title', async () => {
    const mockedPipelineList = vi.mocked(pipelineApi.list);
    const mockedProjectList = vi.mocked(projectApi.list);

    mockedPipelineList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedProjectList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 100 });

    renderPipelines();

    await waitFor(() => {
      expect(screen.getByText('流水线编排')).toBeInTheDocument();
    });
  });

  it('shows pipeline list after data loads', async () => {
    const mockedPipelineList = vi.mocked(pipelineApi.list);
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedPipelineGet = vi.mocked(pipelineApi.get);

    const mockPipelines = [
      {
        id: 1,
        project_id: 1,
        name: '默认短剧流水线',
        description: '测试流水线',
        dag: '{"nodes":[],"edges":[]}',
        is_template: 0,
        enabled: 1,
        created_by: 1,
        created_at: '2024-01-01T00:00:00Z',
        updated_at: '2024-01-01T00:00:00Z',
      },
      {
        id: 2,
        project_id: 1,
        name: '模板流水线',
        description: '模板描述',
        dag: '{"nodes":[],"edges":[]}',
        is_template: 1,
        enabled: 0,
        created_by: 1,
        created_at: '2024-01-01T00:00:00Z',
        updated_at: '2024-01-01T00:00:00Z',
      },
    ];

    mockedPipelineList.mockResolvedValue({ total: 2, list: mockPipelines, page: 1, page_size: 200 });
    mockedProjectList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 100 });
    mockedPipelineGet.mockResolvedValue(mockPipelines[0]);

    renderPipelines();

    await waitFor(() => {
      expect(screen.getByText('默认短剧流水线')).toBeInTheDocument();
    });

    expect(screen.getByText('模板流水线')).toBeInTheDocument();
    expect(screen.getByText('模板')).toBeInTheDocument();
    expect(screen.getByText('禁用')).toBeInTheDocument();
  });

  it('shows empty state when no pipelines', async () => {
    const mockedPipelineList = vi.mocked(pipelineApi.list);
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedPipelineGet = vi.mocked(pipelineApi.get);

    mockedPipelineList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedProjectList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 100 });
    mockedPipelineGet.mockResolvedValue({
      id: 0, project_id: 0, name: '', description: '', dag: '', is_template: 0, enabled: 1, created_by: 0, created_at: '', updated_at: '',
    });

    renderPipelines();

    await waitFor(() => {
      expect(screen.getByText('暂无流水线')).toBeInTheDocument();
    });
  });

  it('shows empty canvas when no pipeline selected', async () => {
    const mockedPipelineList = vi.mocked(pipelineApi.list);
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedPipelineGet = vi.mocked(pipelineApi.get);

    mockedPipelineList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedProjectList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 100 });
    mockedPipelineGet.mockResolvedValue({
      id: 0, project_id: 0, name: '', description: '', dag: '', is_template: 0, enabled: 1, created_by: 0, created_at: '', updated_at: '',
    });

    renderPipelines();

    await waitFor(() => {
      expect(screen.getByText('请选择左侧流水线或新建一个')).toBeInTheDocument();
    });
  });

  it('selects first pipeline by default', async () => {
    const mockedPipelineList = vi.mocked(pipelineApi.list);
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedPipelineGet = vi.mocked(pipelineApi.get);

    const mockPipelines = [
      {
        id: 1,
        project_id: 1,
        name: '默认短剧流水线',
        description: '测试流水线',
        dag: '{"nodes":[],"edges":[]}',
        is_template: 0,
        enabled: 1,
        created_by: 1,
        created_at: '2024-01-01T00:00:00Z',
        updated_at: '2024-01-01T00:00:00Z',
      },
    ];

    mockedPipelineList.mockResolvedValue({ total: 1, list: mockPipelines, page: 1, page_size: 200 });
    mockedProjectList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 100 });
    mockedPipelineGet.mockResolvedValue(mockPipelines[0]);

    renderPipelines();

    await waitFor(() => {
      expect(screen.getByText('默认短剧流水线')).toBeInTheDocument();
    });

    expect(mockedPipelineGet).toHaveBeenCalledWith(1);
  });

  it('opens create pipeline modal', async () => {
    const mockedPipelineList = vi.mocked(pipelineApi.list);
    const mockedProjectList = vi.mocked(projectApi.list);

    mockedPipelineList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedProjectList.mockResolvedValue({
      total: 1,
      list: [{ id: 1, code: 'proj1', name: '项目1', description: '', status: 1, owner_id: 1, dept_id: 1, default_pipeline_id: 0, cover_url: '', created_at: '', updated_at: '' }],
      page: 1,
      page_size: 100,
    });

    renderPipelines();

    await waitFor(() => {
      expect(screen.getByText('新建')).toBeInTheDocument();
    });

    const newButton = screen.getByText('新建');
    await userEvent.click(newButton);

    await waitFor(() => {
      expect(screen.getByText('新建流水线')).toBeInTheDocument();
    });

    expect(screen.getByText('名称')).toBeInTheDocument();
    expect(screen.getByText('所属项目')).toBeInTheDocument();
    expect(screen.getByText('描述')).toBeInTheDocument();
  });

  it('handles API errors gracefully', async () => {
    const mockedPipelineList = vi.mocked(pipelineApi.list);
    const mockedProjectList = vi.mocked(projectApi.list);

    mockedPipelineList.mockRejectedValue(new Error('Network error'));
    mockedProjectList.mockRejectedValue(new Error('Network error'));

    renderPipelines();

    await waitFor(() => {
      expect(screen.getByText('流水线编排')).toBeInTheDocument();
    });

    // Should still render the page structure
    expect(screen.getByText('流水线列表')).toBeInTheDocument();
  });

  it('opens history drawer', async () => {
    const mockedPipelineList = vi.mocked(pipelineApi.list);
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedPipelineGet = vi.mocked(pipelineApi.get);
    const mockedListRuns = vi.mocked(pipelineApi.listRuns);

    const mockPipelines = [
      {
        id: 1,
        project_id: 1,
        name: '默认短剧流水线',
        description: '测试流水线',
        dag: '{"nodes":[],"edges":[]}',
        is_template: 0,
        enabled: 1,
        created_by: 1,
        created_at: '2024-01-01T00:00:00Z',
        updated_at: '2024-01-01T00:00:00Z',
      },
    ];

    mockedPipelineList.mockResolvedValue({ total: 1, list: mockPipelines, page: 1, page_size: 200 });
    mockedProjectList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 100 });
    mockedPipelineGet.mockResolvedValue(mockPipelines[0]);
    mockedListRuns.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 50 });

    renderPipelines();

    await waitFor(() => {
      expect(screen.getByText('历史')).toBeInTheDocument();
    });

    const historyButton = screen.getByText('历史');
    await userEvent.click(historyButton);

    await waitFor(() => {
      expect(mockedListRuns).toHaveBeenCalledWith(1, { page: 1, page_size: 50 });
    });
  });

  it('displays run history in drawer', async () => {
    const mockedPipelineList = vi.mocked(pipelineApi.list);
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedPipelineGet = vi.mocked(pipelineApi.get);
    const mockedListRuns = vi.mocked(pipelineApi.listRuns);

    const mockPipelines = [
      {
        id: 1,
        project_id: 1,
        name: '默认短剧流水线',
        description: '测试流水线',
        dag: '{"nodes":[],"edges":[]}',
        is_template: 0,
        enabled: 1,
        created_by: 1,
        created_at: '2024-01-01T00:00:00Z',
        updated_at: '2024-01-01T00:00:00Z',
      },
    ];

    const mockRuns = [
      {
        id: 1,
        pipeline_id: 1,
        project_id: 1,
        triggered_by: 1,
        trigger_type: 'manual',
        input: {},
        output: {},
        status: 'succeeded',
        started_at: '2024-01-01T10:00:00Z',
        ended_at: '2024-01-01T10:05:00Z',
        error_msg: '',
      },
      {
        id: 2,
        pipeline_id: 1,
        project_id: 1,
        triggered_by: 1,
        trigger_type: 'manual',
        input: {},
        output: {},
        status: 'failed',
        started_at: '2024-01-01T11:00:00Z',
        ended_at: '2024-01-01T11:02:00Z',
        error_msg: 'Timeout error',
      },
    ];

    mockedPipelineList.mockResolvedValue({ total: 1, list: mockPipelines, page: 1, page_size: 200 });
    mockedProjectList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 100 });
    mockedPipelineGet.mockResolvedValue(mockPipelines[0]);
    mockedListRuns.mockResolvedValue({ total: 2, list: mockRuns, page: 1, page_size: 50 });

    renderPipelines();

    await waitFor(() => {
      expect(screen.getByText('历史')).toBeInTheDocument();
    });

    const historyButton = screen.getByText('历史');
    await userEvent.click(historyButton);

    await waitFor(() => {
      expect(screen.getByText('succeeded')).toBeInTheDocument();
    });

    expect(screen.getByText('failed')).toBeInTheDocument();
  });

  it('shows loading state initially', async () => {
    const mockedPipelineList = vi.mocked(pipelineApi.list);
    const mockedProjectList = vi.mocked(projectApi.list);

    // Return pending promises to keep loading state
    mockedPipelineList.mockReturnValue(new Promise(() => {}));
    mockedProjectList.mockReturnValue(new Promise(() => {}));

    renderPipelines();

    // Should show loading state
    await waitFor(() => {
      expect(document.querySelector('[aria-busy="true"]')).toBeInTheDocument();
    });
  });

  it('calls APIs with correct parameters on mount', async () => {
    const mockedPipelineList = vi.mocked(pipelineApi.list);
    const mockedProjectList = vi.mocked(projectApi.list);

    mockedPipelineList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedProjectList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 100 });

    renderPipelines();

    await waitFor(() => {
      expect(mockedPipelineList).toHaveBeenCalledWith({ page: 1, page_size: 200 });
    });

    expect(mockedProjectList).toHaveBeenCalledWith({ page: 1, page_size: 100 });
  });
});
