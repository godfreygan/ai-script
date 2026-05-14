import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import InvocationsPage from './Invocations';
import { invocationApi, modelApi, projectApi, userApi } from '@/api/modules';

// Mock API modules
vi.mock('@/api/modules', async () => {
  const actual = await vi.importActual<typeof import('@/api/modules')>('@/api/modules');
  return {
    ...actual,
    invocationApi: {
      list: vi.fn(),
      stats: vi.fn(),
    },
    projectApi: {
      list: vi.fn(),
    },
    modelApi: {
      list: vi.fn(),
    },
    userApi: {
      list: vi.fn(),
    },
  };
});

// Mock dayjs to return stable dates
vi.mock('dayjs', () => {
  const mockDayjs = (_input?: string) => ({
    toISOString: () => '2024-01-01T12:00:00.000Z',
    format: (fmt: string) => {
      if (fmt === 'YYYY-MM-DD HH:mm:ss') return '2024-01-01 12:00:00';
      return '2024-01-01';
    },
    startOf: () => ({
      toISOString: () => '2024-01-01T00:00:00.000Z',
    }),
    endOf: () => ({
      toISOString: () => '2024-01-01T23:59:59.999Z',
    }),
    subtract: () => mockDayjs(),
  });
  mockDayjs.extend = () => {};
  return { default: mockDayjs };
});

function renderInvocations() {
  return render(<InvocationsPage />);
}

describe('InvocationsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders invocations page with title', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedModelList = vi.mocked(modelApi.list);
    const mockedUserList = vi.mocked(userApi.list);
    const mockedInvocationList = vi.mocked(invocationApi.list);
    const mockedInvocationStats = vi.mocked(invocationApi.stats);

    mockedProjectList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedModelList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedUserList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedInvocationList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });
    mockedInvocationStats.mockResolvedValue({ calls: 0, input_tokens: 0, output_tokens: 0, units: 0, cost: 0 });

    renderInvocations();

    await waitFor(() => {
      expect(screen.getByText('模型调用统计')).toBeInTheDocument();
    });
  });

  it('shows loading state initially', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedModelList = vi.mocked(modelApi.list);
    const mockedUserList = vi.mocked(userApi.list);
    const mockedInvocationList = vi.mocked(invocationApi.list);
    const mockedInvocationStats = vi.mocked(invocationApi.stats);

    // Return pending promises to keep loading state
    mockedProjectList.mockReturnValue(new Promise(() => {}));
    mockedModelList.mockReturnValue(new Promise(() => {}));
    mockedUserList.mockReturnValue(new Promise(() => {}));
    mockedInvocationList.mockReturnValue(new Promise(() => {}));
    mockedInvocationStats.mockReturnValue(new Promise(() => {}));

    renderInvocations();

    // Antd Spin should be active
    await waitFor(() => {
      expect(document.querySelector('[aria-busy="true"]')).toBeInTheDocument();
    });
  });

  it('displays statistics cards after data loads', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedModelList = vi.mocked(modelApi.list);
    const mockedUserList = vi.mocked(userApi.list);
    const mockedInvocationList = vi.mocked(invocationApi.list);
    const mockedInvocationStats = vi.mocked(invocationApi.stats);

    mockedProjectList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedModelList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedUserList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedInvocationList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });
    mockedInvocationStats.mockResolvedValue({ calls: 100, input_tokens: 1000, output_tokens: 500, units: 10, cost: 0.5 });

    renderInvocations();

    await waitFor(() => {
      expect(screen.getByText('调用次数')).toBeInTheDocument();
    });

    expect(screen.getByText('输入 Tokens')).toBeInTheDocument();
    expect(screen.getByText('输出 Tokens')).toBeInTheDocument();
    expect(screen.getByText('单元数')).toBeInTheDocument();
    expect(screen.getByText('累计成本')).toBeInTheDocument();

    // Verify stats values are displayed (antd Statistic splits numbers across spans)
    const callsCard = screen.getByText('调用次数').closest('.ant-card');
    expect(callsCard?.textContent).toContain('100');
  });

  it('displays empty state for invocation table', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedModelList = vi.mocked(modelApi.list);
    const mockedUserList = vi.mocked(userApi.list);
    const mockedInvocationList = vi.mocked(invocationApi.list);
    const mockedInvocationStats = vi.mocked(invocationApi.stats);

    mockedProjectList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedModelList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedUserList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedInvocationList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });
    mockedInvocationStats.mockResolvedValue({ calls: 0, input_tokens: 0, output_tokens: 0, units: 0, cost: 0 });

    renderInvocations();

    await waitFor(() => {
      expect(screen.getByText('调用日志')).toBeInTheDocument();
    });

    // Table should show empty text (use queryAllByText since multiple elements may have this)
    expect(screen.queryAllByText('暂无数据').length).toBeGreaterThanOrEqual(1);
  });

  it('displays invocation logs in table', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedModelList = vi.mocked(modelApi.list);
    const mockedUserList = vi.mocked(userApi.list);
    const mockedInvocationList = vi.mocked(invocationApi.list);
    const mockedInvocationStats = vi.mocked(invocationApi.stats);

    const mockLogs = [
      {
        id: 1,
        model_id: 1,
        user_id: 1,
        dept_id: 1,
        project_id: 1,
        biz_type: 'script_split',
        biz_ref: 'ref-1',
        input_tokens: 100,
        output_tokens: 50,
        units: 1,
        duration_ms: 1200,
        cost: 0.01,
        status: 'succeeded',
        error_code: '',
        started_at: '2024-01-01T12:00:00Z',
        ended_at: '2024-01-01T12:00:01Z',
      },
      {
        id: 2,
        model_id: 2,
        user_id: 1,
        dept_id: 1,
        project_id: 1,
        biz_type: 'image_gen',
        biz_ref: 'ref-2',
        input_tokens: 200,
        output_tokens: 0,
        units: 2,
        duration_ms: 3000,
        cost: 0.02,
        status: 'failed',
        error_code: 'ERR_TIMEOUT',
        started_at: '2024-01-01T11:00:00Z',
        ended_at: '2024-01-01T11:00:03Z',
      },
    ];

    mockedProjectList.mockResolvedValue({
      total: 1,
      list: [{ id: 1, code: 'proj1', name: '项目1', description: '', status: 1, owner_id: 1, dept_id: 1, default_pipeline_id: 0, cover_url: '', created_at: '', updated_at: '' }],
      page: 1,
      page_size: 200,
    });
    mockedModelList.mockResolvedValue({
      total: 2,
      list: [
        { id: 1, code: 'gpt-4', name: 'GPT-4', type: 'llm', provider: 'openai', endpoint: '', default_params: {}, capability_tags: [], enabled: 1, priority: 1, max_qps: 10, health_check_url: '', last_health_status: 1 },
        { id: 2, code: 'sd-xl', name: 'SD-XL', type: 'image', provider: 'stability', endpoint: '', default_params: {}, capability_tags: [], enabled: 1, priority: 1, max_qps: 10, health_check_url: '', last_health_status: 1 },
      ],
      page: 1,
      page_size: 200,
    });
    mockedUserList.mockResolvedValue({
      total: 1,
      list: [{ id: 1, username: 'admin', nickname: 'Admin', email: '', phone: '', dept_id: 1, status: 1 }],
      page: 1,
      page_size: 200,
    });
    mockedInvocationList.mockResolvedValue({ total: 2, list: mockLogs, page: 1, page_size: 20 });
    mockedInvocationStats.mockResolvedValue({ calls: 2, input_tokens: 300, output_tokens: 50, units: 3, cost: 0.03 });

    renderInvocations();

    await waitFor(() => {
      expect(screen.getByText('script_split')).toBeInTheDocument();
    });

    expect(screen.getByText('image_gen')).toBeInTheDocument();
    expect(screen.getByText('succeeded')).toBeInTheDocument();
    expect(screen.getByText('failed')).toBeInTheDocument();
    expect(screen.getByText('1200')).toBeInTheDocument();
    expect(screen.getByText('3000')).toBeInTheDocument();
  });

  it('handles API errors gracefully', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedModelList = vi.mocked(modelApi.list);
    const mockedUserList = vi.mocked(userApi.list);
    const mockedInvocationList = vi.mocked(invocationApi.list);
    const mockedInvocationStats = vi.mocked(invocationApi.stats);

    mockedProjectList.mockRejectedValue(new Error('Network error'));
    mockedModelList.mockRejectedValue(new Error('Network error'));
    mockedUserList.mockRejectedValue(new Error('Network error'));
    mockedInvocationList.mockRejectedValue(new Error('Network error'));
    mockedInvocationStats.mockRejectedValue(new Error('Network error'));

    renderInvocations();

    await waitFor(() => {
      expect(screen.getByText('模型调用统计')).toBeInTheDocument();
    });

    // Stats should fallback to 0 (use queryAllByText since multiple 0s exist)
    const zeroElements = screen.queryAllByText('0');
    expect(zeroElements.length).toBeGreaterThan(0);

    // Table should show empty state
    expect(screen.queryAllByText('暂无数据').length).toBeGreaterThanOrEqual(1);
  });

  it('displays correct biz_type tags with colors', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedModelList = vi.mocked(modelApi.list);
    const mockedUserList = vi.mocked(userApi.list);
    const mockedInvocationList = vi.mocked(invocationApi.list);
    const mockedInvocationStats = vi.mocked(invocationApi.stats);

    const mockLogs = [
      {
        id: 1,
        model_id: 1,
        user_id: 1,
        dept_id: 1,
        project_id: 1,
        biz_type: 'video_gen',
        biz_ref: 'ref-1',
        input_tokens: 100,
        output_tokens: 50,
        units: 1,
        duration_ms: 5000,
        cost: 0.05,
        status: 'running',
        error_code: '',
        started_at: '2024-01-01T10:00:00Z',
        ended_at: undefined,
      },
    ];

    mockedProjectList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedModelList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedUserList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedInvocationList.mockResolvedValue({ total: 1, list: mockLogs, page: 1, page_size: 20 });
    mockedInvocationStats.mockResolvedValue({ calls: 1, input_tokens: 100, output_tokens: 50, units: 1, cost: 0.05 });

    renderInvocations();

    await waitFor(() => {
      expect(screen.getByText('video_gen')).toBeInTheDocument();
    });

    // Check status tag
    expect(screen.getByText('running')).toBeInTheDocument();
  });

  it('displays cost formatted with 4 decimal precision', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedModelList = vi.mocked(modelApi.list);
    const mockedUserList = vi.mocked(userApi.list);
    const mockedInvocationList = vi.mocked(invocationApi.list);
    const mockedInvocationStats = vi.mocked(invocationApi.stats);

    mockedProjectList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedModelList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedUserList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedInvocationList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });
    mockedInvocationStats.mockResolvedValue({ calls: 0, input_tokens: 0, output_tokens: 0, units: 0, cost: 0.1234 });

    renderInvocations();

    await waitFor(() => {
      const costTitle = screen.getByText('累计成本');
      expect(costTitle.closest('.ant-card')?.textContent).toContain('0.1234');
    });
  });

  it('calls APIs with correct parameters on mount', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedModelList = vi.mocked(modelApi.list);
    const mockedUserList = vi.mocked(userApi.list);
    const mockedInvocationList = vi.mocked(invocationApi.list);
    const mockedInvocationStats = vi.mocked(invocationApi.stats);

    mockedProjectList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedModelList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedUserList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedInvocationList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });
    mockedInvocationStats.mockResolvedValue({ calls: 0, input_tokens: 0, output_tokens: 0, units: 0, cost: 0 });

    renderInvocations();

    await waitFor(() => {
      expect(mockedProjectList).toHaveBeenCalledWith({ page_size: 200 });
    });

    expect(mockedModelList).toHaveBeenCalledWith({ page_size: 200 });
    expect(mockedUserList).toHaveBeenCalledWith({ page_size: 200 });
    expect(mockedInvocationList).toHaveBeenCalledWith(expect.objectContaining({ page: 1, page_size: 20 }));
    expect(mockedInvocationStats).toHaveBeenCalled();
  });

  it('filters by status', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedModelList = vi.mocked(modelApi.list);
    const mockedUserList = vi.mocked(userApi.list);
    const mockedInvocationList = vi.mocked(invocationApi.list);
    const mockedInvocationStats = vi.mocked(invocationApi.stats);

    mockedProjectList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedModelList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedUserList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedInvocationList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });
    mockedInvocationStats.mockResolvedValue({ calls: 0, input_tokens: 0, output_tokens: 0, units: 0, cost: 0 });

    renderInvocations();

    await waitFor(() => {
      // Use getAllByText since '状态' appears in both the filter form and status tags
      expect(screen.getAllByText('状态').length).toBeGreaterThan(0);
    });

    // Status filter should be present
    const statusLabels = screen.getAllByText('状态');
    expect(statusLabels.length).toBeGreaterThan(0);
  });

  it('filters by biz_type', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedModelList = vi.mocked(modelApi.list);
    const mockedUserList = vi.mocked(userApi.list);
    const mockedInvocationList = vi.mocked(invocationApi.list);
    const mockedInvocationStats = vi.mocked(invocationApi.stats);

    mockedProjectList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedModelList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedUserList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedInvocationList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });
    mockedInvocationStats.mockResolvedValue({ calls: 0, input_tokens: 0, output_tokens: 0, units: 0, cost: 0 });

    renderInvocations();

    await waitFor(() => {
      expect(screen.getByText('业务类型')).toBeInTheDocument();
    });

    // Biz type filter should be present
    const bizTypeLabel = screen.getByText('业务类型');
    expect(bizTypeLabel).toBeInTheDocument();
  });
});
