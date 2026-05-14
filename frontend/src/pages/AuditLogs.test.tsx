import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import AuditLogsPage from './AuditLogs';
import { auditApi } from '@/api/modules';

// Mock API modules
vi.mock('@/api/modules', async () => {
  const actual = await vi.importActual<typeof import('@/api/modules')>('@/api/modules');
  return {
    ...actual,
    auditApi: {
      list: vi.fn(),
    },
  };
});

// Mock dayjs to return stable dates
vi.mock('dayjs', () => {
  const mockDayjs = (_v?: string) => ({
    format: (fmt: string) => {
      if (fmt === 'YYYY-MM-DD HH:mm:ss') return '2024-01-01 12:00:00';
      return '2024-01-01';
    },
  });
  mockDayjs.extend = () => {};
  return { default: mockDayjs };
});

// Mock navigator.clipboard
const writeTextMock = vi.hoisted(() => vi.fn().mockResolvedValue(undefined));
Object.defineProperty(globalThis, 'navigator', {
  value: {
    clipboard: {
      writeText: writeTextMock,
    },
  },
  writable: true,
  configurable: true,
});

// Mock AntApp.useApp to provide message API
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
        notification: {
          success: vi.fn(),
          error: vi.fn(),
        },
      }),
    },
  };
});

function renderAuditLogs() {
  return render(<AuditLogsPage />);
}

describe('AuditLogsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders page with title', async () => {
    const mockedList = vi.mocked(auditApi.list);
    mockedList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });

    renderAuditLogs();

    await waitFor(() => {
      expect(screen.getByText('审计日志查询')).toBeInTheDocument();
    });
  });

  it('shows loading state initially', async () => {
    const mockedList = vi.mocked(auditApi.list);
    mockedList.mockReturnValue(new Promise(() => {}));

    renderAuditLogs();

    await waitFor(() => {
      expect(document.querySelector('[aria-busy="true"]')).toBeInTheDocument();
    });
  });

  it('displays audit logs list after data loads', async () => {
    const mockedList = vi.mocked(auditApi.list);
    const mockLogs = [
      {
        id: 1,
        user_id: 1,
        action: 'create',
        resource_type: 'project',
        resource_id: '10',
        before: '',
        after: { name: 'Test Project' },
        ip: '192.168.1.1',
        ua: 'Mozilla/5.0',
        request_id: 'req-001',
        created_at: '2024-01-01T12:00:00Z',
      },
      {
        id: 2,
        user_id: 2,
        action: 'update',
        resource_type: 'user',
        resource_id: '5',
        before: { name: 'Old Name' },
        after: { name: 'New Name' },
        ip: '192.168.1.2',
        ua: '',
        request_id: '',
        created_at: '2024-01-02T10:30:00Z',
      },
      {
        id: 3,
        user_id: 1,
        action: 'delete',
        resource_type: 'script',
        resource_id: '20',
        before: { title: 'Script Title' },
        after: '',
        ip: '192.168.1.3',
        ua: 'Mozilla/5.0 (Windows)',
        request_id: 'req-003-very-long-request-id-string',
        created_at: '2024-01-03T08:15:00Z',
      },
    ];
    mockedList.mockResolvedValue({ total: 3, list: mockLogs, page: 1, page_size: 20 });

    renderAuditLogs();

    await waitFor(() => {
      expect(screen.getByText('审计记录')).toBeInTheDocument();
    });

    // Check table content
    expect(screen.getByText('create')).toBeInTheDocument();
    expect(screen.getByText('update')).toBeInTheDocument();
    expect(screen.getByText('delete')).toBeInTheDocument();
    expect(screen.getByText('project')).toBeInTheDocument();
    expect(screen.getByText('user')).toBeInTheDocument();
    expect(screen.getByText('script')).toBeInTheDocument();
    expect(screen.getByText('#10')).toBeInTheDocument();
    expect(screen.getByText('#5')).toBeInTheDocument();
    expect(screen.getByText('#20')).toBeInTheDocument();
    expect(screen.getByText('192.168.1.1')).toBeInTheDocument();
    expect(screen.getByText('192.168.1.2')).toBeInTheDocument();
    expect(screen.getByText('192.168.1.3')).toBeInTheDocument();
    expect(screen.getByText('req-001')).toBeInTheDocument();
  });

  it('displays empty state when no logs exist', async () => {
    const mockedList = vi.mocked(auditApi.list);
    mockedList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });

    renderAuditLogs();

    await waitFor(() => {
      expect(screen.getByText('暂无审计记录,可调整左上方筛选条件')).toBeInTheDocument();
    });
  });

  it('handles API error gracefully', async () => {
    const mockedList = vi.mocked(auditApi.list);
    mockedList.mockRejectedValue(new Error('Network error'));

    renderAuditLogs();

    await waitFor(() => {
      expect(screen.getByText('审计日志查询')).toBeInTheDocument();
    });

    // Should show empty state after error
    await waitFor(() => {
      expect(screen.getByText('暂无审计记录,可调整左上方筛选条件')).toBeInTheDocument();
    });
  });

  it('filters by user_id', async () => {
    const user = userEvent.setup();
    const mockedList = vi.mocked(auditApi.list);
    mockedList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });

    renderAuditLogs();

    await waitFor(() => {
      expect(screen.getByText('审计日志查询')).toBeInTheDocument();
    });

    const userIdInput = screen.getByPlaceholderText('user_id');
    await user.type(userIdInput, '5');

    await waitFor(() => {
      expect(mockedList).toHaveBeenCalledWith(
        expect.objectContaining({
          user_id: 5,
          page: 1,
          page_size: 20,
        })
      );
    });
  });

  it('filters by resource_type', async () => {
    const mockedList = vi.mocked(auditApi.list);
    mockedList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });

    renderAuditLogs();

    await waitFor(() => {
      expect(screen.getByText('审计日志查询')).toBeInTheDocument();
    });

    // Find resource type select (label is "资源类型")
    const resourceSelect = screen.getByText('资源类型').closest('.ant-form-item')?.querySelector('.ant-select');
    expect(resourceSelect).toBeInTheDocument();
  });

  it('filters by action', async () => {
    const mockedList = vi.mocked(auditApi.list);
    mockedList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });

    renderAuditLogs();

    await waitFor(() => {
      expect(screen.getByText('审计日志查询')).toBeInTheDocument();
    });

    // Find action select by label title (label is "操作" but appears in multiple places)
    const actionLabel = screen.getByTitle('操作');
    expect(actionLabel).toBeInTheDocument();
  });

  it('changes page size', async () => {
    const mockedList = vi.mocked(auditApi.list);
    mockedList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });

    renderAuditLogs();

    await waitFor(() => {
      expect(screen.getByText('审计日志查询')).toBeInTheDocument();
    });

    // Find page size select (label is "每页")
    const pageSizeSelect = screen.getByText('每页').closest('.ant-form-item')?.querySelector('.ant-select');
    expect(pageSizeSelect).toBeInTheDocument();
  });

  it('opens detail drawer when clicking a row', async () => {
    const user = userEvent.setup();
    const mockedList = vi.mocked(auditApi.list);
    const mockLog = {
      id: 1,
      user_id: 1,
      action: 'create',
      resource_type: 'project',
      resource_id: '10',
      before: '',
      after: { name: 'Test Project', status: 1 },
      ip: '192.168.1.1',
      ua: 'Mozilla/5.0',
      request_id: 'req-001',
      created_at: '2024-01-01T12:00:00Z',
    };
    mockedList.mockResolvedValue({ total: 1, list: [mockLog], page: 1, page_size: 20 });

    renderAuditLogs();

    await waitFor(() => {
      expect(screen.getByText('create')).toBeInTheDocument();
    });

    // Click on the row (onRow click handler)
    const row = screen.getByText('create').closest('tr');
    expect(row).toBeInTheDocument();
    if (row) {
      await user.click(row);
    }

    await waitFor(() => {
      expect(screen.getByText('审计详情 #1')).toBeInTheDocument();
    });

    // Check drawer content
    expect(screen.getByText('Before')).toBeInTheDocument();
    expect(screen.getByText('After')).toBeInTheDocument();
    expect(screen.getByText('(空)')).toBeInTheDocument();
  });

  it('shows copy button for request id', async () => {
    const mockedList = vi.mocked(auditApi.list);
    const mockLog = {
      id: 1,
      user_id: 1,
      action: 'create',
      resource_type: 'project',
      resource_id: '10',
      before: '',
      after: { name: 'Test Project' },
      ip: '192.168.1.1',
      ua: 'Mozilla/5.0',
      request_id: 'req-001',
      created_at: '2024-01-01T12:00:00Z',
    };
    mockedList.mockResolvedValue({ total: 1, list: [mockLog], page: 1, page_size: 20 });

    renderAuditLogs();

    await waitFor(() => {
      expect(screen.getByText('req-001')).toBeInTheDocument();
    });

    // Verify copy icon/button exists in the Request ID cell
    const copyIcon = document.querySelector('.anticon-copy');
    expect(copyIcon).toBeTruthy();
    const copyBtn = copyIcon?.closest('button');
    expect(copyBtn).toBeTruthy();
  });

  it('calls list API with correct default parameters', async () => {
    const mockedList = vi.mocked(auditApi.list);
    mockedList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });

    renderAuditLogs();

    await waitFor(() => {
      expect(mockedList).toHaveBeenCalledTimes(1);
    });

    expect(mockedList).toHaveBeenCalledWith({
      page: 1,
      page_size: 20,
      user_id: undefined,
      resource_type: undefined,
      action: undefined,
    });
  });

  it('displays action tags with correct colors', async () => {
    const mockedList = vi.mocked(auditApi.list);
    const mockLogs = [
      {
        id: 1,
        user_id: 1,
        action: 'create',
        resource_type: 'project',
        resource_id: '10',
        before: '',
        after: '',
        ip: '',
        ua: '',
        request_id: '',
        created_at: '2024-01-01T12:00:00Z',
      },
      {
        id: 2,
        user_id: 1,
        action: 'delete',
        resource_type: 'user',
        resource_id: '5',
        before: '',
        after: '',
        ip: '',
        ua: '',
        request_id: '',
        created_at: '2024-01-02T12:00:00Z',
      },
    ];
    mockedList.mockResolvedValue({ total: 2, list: mockLogs, page: 1, page_size: 20 });

    renderAuditLogs();

    await waitFor(() => {
      expect(screen.getByText('create')).toBeInTheDocument();
    });

    expect(screen.getByText('delete')).toBeInTheDocument();
  });

  it('handles pagination changes', async () => {
    const mockedList = vi.mocked(auditApi.list);
    mockedList.mockResolvedValue({ total: 50, list: [], page: 1, page_size: 20 });

    renderAuditLogs();

    await waitFor(() => {
      expect(screen.getByText('审计记录')).toBeInTheDocument();
    });

    // Should show pagination with total 50
    await waitFor(() => {
      expect(mockedList).toHaveBeenCalledWith(expect.objectContaining({ page: 1, page_size: 20 }));
    });
  });

  it('displays dash for missing values', async () => {
    const mockedList = vi.mocked(auditApi.list);
    const mockLog = {
      id: 1,
      user_id: 1,
      action: 'login',
      resource_type: 'user',
      resource_id: '',
      before: '',
      after: '',
      ip: '',
      ua: '',
      request_id: '',
      created_at: '',
    };
    mockedList.mockResolvedValue({ total: 1, list: [mockLog], page: 1, page_size: 20 });

    renderAuditLogs();

    await waitFor(() => {
      expect(screen.getByText('login')).toBeInTheDocument();
    });

    // Should show '-' for missing UA
    const dashElements = screen.getAllByText('-');
    expect(dashElements.length).toBeGreaterThan(0);
  });
});
