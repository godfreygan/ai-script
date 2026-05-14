import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import DashboardPage from './Dashboard';
import { projectApi, invocationApi } from '@/api/modules';

// Mock API modules
vi.mock('@/api/modules', async () => {
  const actual = await vi.importActual<typeof import('@/api/modules')>('@/api/modules');
  return {
    ...actual,
    projectApi: {
      list: vi.fn(),
    },
    invocationApi: {
      list: vi.fn(),
      stats: vi.fn(),
    },
  };
});

// Mock dayjs to return stable dates
vi.mock('dayjs', () => {
  const mockDayjs = () => ({
    startOf: () => ({
      toISOString: () => '2024-01-01T00:00:00.000Z',
    }),
    endOf: () => ({
      toISOString: () => '2024-01-01T23:59:59.999Z',
    }),
    format: (fmt: string) => {
      if (fmt === 'YYYY-MM-DD HH:mm:ss') return '2024-01-01 12:00:00';
      return '2024-01-01';
    },
  });
  mockDayjs.extend = () => {};
  return { default: mockDayjs };
});

function renderDashboard() {
  return render(<DashboardPage />);
}

describe('DashboardPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders dashboard with title', () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedInvocationStats = vi.mocked(invocationApi.stats);
    const mockedInvocationList = vi.mocked(invocationApi.list);

    mockedProjectList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 1 });
    mockedInvocationStats.mockResolvedValue({ calls: 0, input_tokens: 0, output_tokens: 0, units: 0, cost: 0 });
    mockedInvocationList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 8 });

    renderDashboard();

    expect(screen.getByText('工作台')).toBeInTheDocument();
  });

  it('shows loading state initially', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedInvocationStats = vi.mocked(invocationApi.stats);
    const mockedInvocationList = vi.mocked(invocationApi.list);

    // Return pending promises to keep loading state
    mockedProjectList.mockReturnValue(new Promise(() => {}));
    mockedInvocationStats.mockReturnValue(new Promise(() => {}));
    mockedInvocationList.mockReturnValue(new Promise(() => {}));

    renderDashboard();

    // Antd Spin should be active (aria-busy is set on the spin element)
    await waitFor(() => {
      expect(document.querySelector('[aria-busy="true"]')).toBeInTheDocument();
    });
  });

  it('displays statistics cards after data loads', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedInvocationStats = vi.mocked(invocationApi.stats);
    const mockedInvocationList = vi.mocked(invocationApi.list);

    mockedProjectList.mockResolvedValue({ total: 5, list: [], page: 1, page_size: 1 });
    mockedInvocationStats.mockResolvedValue({ calls: 100, input_tokens: 1000, output_tokens: 500, units: 10, cost: 0.5 });
    mockedInvocationList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 8 });

    renderDashboard();

    await waitFor(() => {
      expect(screen.getByText('在产项目')).toBeInTheDocument();
    });

    // Check statistic values (use getAllByText because '100' is a substring of '1000')
    expect(screen.getByText('5')).toBeInTheDocument(); // 在产项目
    expect(screen.getAllByText('100').length).toBeGreaterThanOrEqual(1); // 今日总调用
    // antd Statistic splits number across spans; search within the card textContent
    const costCard = screen.getByText('今日累计成本').closest('.ant-card');
    expect(costCard?.textContent).toContain('0.5');
  });

  it('displays empty state for recent invocations table', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedInvocationStats = vi.mocked(invocationApi.stats);
    const mockedInvocationList = vi.mocked(invocationApi.list);

    mockedProjectList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 1 });
    mockedInvocationStats.mockResolvedValue({ calls: 0, input_tokens: 0, output_tokens: 0, units: 0, cost: 0 });
    mockedInvocationList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 8 });

    renderDashboard();

    await waitFor(() => {
      expect(screen.getByText('近期模型调用')).toBeInTheDocument();
    });

    // Table should show empty text
    expect(screen.getByText('暂无数据')).toBeInTheDocument();
  });

  it('displays recent invocation logs in table', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedInvocationStats = vi.mocked(invocationApi.stats);
    const mockedInvocationList = vi.mocked(invocationApi.list);

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

    mockedProjectList.mockResolvedValue({ total: 3, list: [], page: 1, page_size: 1 });
    mockedInvocationStats.mockResolvedValue({ calls: 50, input_tokens: 500, output_tokens: 200, units: 5, cost: 0.3 });
    mockedInvocationList.mockResolvedValue({ total: 2, list: mockLogs, page: 1, page_size: 8 });

    renderDashboard();

    await waitFor(() => {
      expect(screen.getByText('近期模型调用')).toBeInTheDocument();
    });

    // Check table content
    expect(screen.getByText('script_split')).toBeInTheDocument();
    expect(screen.getByText('image_gen')).toBeInTheDocument();
    expect(screen.getByText('succeeded')).toBeInTheDocument();
    expect(screen.getByText('failed')).toBeInTheDocument();
    expect(screen.getByText('#1')).toBeInTheDocument();
    expect(screen.getByText('#2')).toBeInTheDocument();
    expect(screen.getByText('1200')).toBeInTheDocument();
    expect(screen.getByText('3000')).toBeInTheDocument();
  });

  it('handles API errors gracefully and shows fallback values', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedInvocationStats = vi.mocked(invocationApi.stats);
    const mockedInvocationList = vi.mocked(invocationApi.list);

    mockedProjectList.mockRejectedValue(new Error('Network error'));
    mockedInvocationStats.mockRejectedValue(new Error('Network error'));
    mockedInvocationList.mockRejectedValue(new Error('Network error'));

    renderDashboard();

    await waitFor(() => {
      expect(screen.getByText('工作台')).toBeInTheDocument();
    });

    // Should show 0 for all statistics when APIs fail
    await waitFor(() => {
      // All stats should fallback to 0
      const zeroElements = screen.getAllByText('0');
      expect(zeroElements.length).toBeGreaterThan(0);
    });

    // Table should show empty state
    expect(screen.getByText('暂无数据')).toBeInTheDocument();
  });

  it('handles partial API failures', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedInvocationStats = vi.mocked(invocationApi.stats);
    const mockedInvocationList = vi.mocked(invocationApi.list);

    // Project API succeeds, invocation APIs fail
    mockedProjectList.mockResolvedValue({ total: 10, list: [], page: 1, page_size: 1 });
    mockedInvocationStats.mockRejectedValue(new Error('Stats error'));
    mockedInvocationList.mockRejectedValue(new Error('List error'));

    renderDashboard();

    await waitFor(() => {
      expect(screen.getByText('在产项目')).toBeInTheDocument();
    });

    // Project count should still show
    expect(screen.getByText('10')).toBeInTheDocument();

    // Other stats should fallback to 0
    expect(screen.getByText('暂无数据')).toBeInTheDocument();
  });

  it('displays correct biz_type tags with colors', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedInvocationStats = vi.mocked(invocationApi.stats);
    const mockedInvocationList = vi.mocked(invocationApi.list);

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
      },
    ];

    mockedProjectList.mockResolvedValue({ total: 1, list: [], page: 1, page_size: 1 });
    mockedInvocationStats.mockResolvedValue({ calls: 10, input_tokens: 100, output_tokens: 50, units: 1, cost: 0.05 });
    mockedInvocationList.mockResolvedValue({ total: 1, list: mockLogs, page: 1, page_size: 8 });

    renderDashboard();

    await waitFor(() => {
      expect(screen.getByText('video_gen')).toBeInTheDocument();
    });

    // Check status tag
    expect(screen.getByText('running')).toBeInTheDocument();
  });

  it('displays cost formatted with 4 decimal precision', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedInvocationStats = vi.mocked(invocationApi.stats);
    const mockedInvocationList = vi.mocked(invocationApi.list);

    mockedProjectList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 1 });
    mockedInvocationStats.mockResolvedValue({ calls: 0, input_tokens: 0, output_tokens: 0, units: 0, cost: 0.1234 });
    mockedInvocationList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 8 });

    renderDashboard();

    await waitFor(() => {
      // antd Statistic splits number across spans; search within the card textContent
      const costTitle = screen.getByText('今日累计成本');
      expect(costTitle.closest('.ant-card')?.textContent).toContain('0.1234');
    });
  });

  it('calls APIs with correct date range parameters', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedInvocationStats = vi.mocked(invocationApi.stats);
    const mockedInvocationList = vi.mocked(invocationApi.list);

    mockedProjectList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 1 });
    mockedInvocationStats.mockResolvedValue({ calls: 0, input_tokens: 0, output_tokens: 0, units: 0, cost: 0 });
    mockedInvocationList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 8 });

    renderDashboard();

    await waitFor(() => {
      expect(mockedProjectList).toHaveBeenCalledWith({ page: 1, page_size: 1 });
    });

    // Verify stats API was called with date range and biz_type filters
    expect(mockedInvocationStats).toHaveBeenCalledTimes(4);
    expect(mockedInvocationStats).toHaveBeenCalledWith(
      expect.objectContaining({
        from: '2024-01-01T00:00:00.000Z',
        to: '2024-01-01T23:59:59.999Z',
      })
    );
    expect(mockedInvocationStats).toHaveBeenCalledWith(
      expect.objectContaining({
        from: '2024-01-01T00:00:00.000Z',
        to: '2024-01-01T23:59:59.999Z',
        biz_type: 'script_split',
      })
    );
    expect(mockedInvocationStats).toHaveBeenCalledWith(
      expect.objectContaining({
        from: '2024-01-01T00:00:00.000Z',
        to: '2024-01-01T23:59:59.999Z',
        biz_type: 'image_gen',
      })
    );
    expect(mockedInvocationStats).toHaveBeenCalledWith(
      expect.objectContaining({
        from: '2024-01-01T00:00:00.000Z',
        to: '2024-01-01T23:59:59.999Z',
        biz_type: 'video_gen',
      })
    );

    // Verify list API call
    expect(mockedInvocationList).toHaveBeenCalledWith({ page: 1, page_size: 8 });
  });
});
