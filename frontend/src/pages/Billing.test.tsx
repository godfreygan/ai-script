import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import BillingPage from './Billing';
import { billingApi, deptApi, modelApi, userApi } from '@/api/modules';

// Mock API modules
vi.mock('@/api/modules', async () => {
  const actual = await vi.importActual<typeof import('@/api/modules')>('@/api/modules');
  return {
    ...actual,
    billingApi: {
      listQuotas: vi.fn(),
      createQuota: vi.fn(),
      updateQuota: vi.fn(),
      deleteQuota: vi.fn(),
      listDaily: vi.fn(),
    },
    userApi: {
      list: vi.fn(),
    },
    deptApi: {
      list: vi.fn(),
    },
    modelApi: {
      list: vi.fn(),
    },
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

function renderBilling() {
  return render(<BillingPage />);
}

describe('BillingPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders billing page with title', async () => {
    const mockedUserList = vi.mocked(userApi.list);
    const mockedDeptList = vi.mocked(deptApi.list);
    const mockedModelList = vi.mocked(modelApi.list);
    const mockedListQuotas = vi.mocked(billingApi.listQuotas);
    const mockedListDaily = vi.mocked(billingApi.listDaily);

    mockedUserList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 500 });
    mockedDeptList.mockResolvedValue([]);
    mockedModelList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedListQuotas.mockResolvedValue([]);
    mockedListDaily.mockResolvedValue([]);

    renderBilling();

    await waitFor(() => {
      expect(screen.getByText('计费 / 额度管理')).toBeInTheDocument();
    });
  });

  it('shows quota list after data loads', async () => {
    const mockedUserList = vi.mocked(userApi.list);
    const mockedDeptList = vi.mocked(deptApi.list);
    const mockedModelList = vi.mocked(modelApi.list);
    const mockedListQuotas = vi.mocked(billingApi.listQuotas);
    const mockedListDaily = vi.mocked(billingApi.listDaily);

    mockedUserList.mockResolvedValue({
      total: 1,
      list: [{ id: 1, username: 'admin', nickname: 'Admin', email: '', phone: '', dept_id: 1, status: 1 }],
      page: 1,
      page_size: 500,
    });
    mockedDeptList.mockResolvedValue([{ id: 1, name: '技术部', parent_id: 0, path: '1', sort: 1, status: 1 }]);
    mockedModelList.mockResolvedValue({
      total: 1,
      list: [{ id: 1, code: 'gpt-4', name: 'GPT-4', type: 'llm', provider: 'openai', endpoint: '', default_params: {}, capability_tags: [], enabled: 1, priority: 1, max_qps: 10, health_check_url: '', last_health_status: 1 }],
      page: 1,
      page_size: 200,
    });

    const mockQuotas = [
      {
        id: 1,
        scope_type: 'user',
        scope_id: 1,
        model_id: 1,
        period: 'monthly',
        metric: 'calls',
        quota_value: 1000,
        used_value: 500,
        enabled: 1,
      },
      {
        id: 2,
        scope_type: 'dept',
        scope_id: 1,
        model_id: 0,
        period: 'daily',
        metric: 'cost',
        quota_value: 100,
        used_value: 80,
        enabled: 0,
      },
    ];

    mockedListQuotas.mockResolvedValue(mockQuotas);
    mockedListDaily.mockResolvedValue([]);

    renderBilling();

    await waitFor(() => {
      expect(screen.getByText('额度管理')).toBeInTheDocument();
    });

    // Check quota table columns are rendered
    expect(screen.getByText('ID')).toBeInTheDocument();
    expect(screen.getByText('范围')).toBeInTheDocument();
    expect(screen.getByText('模型')).toBeInTheDocument();
    expect(screen.getByText('周期')).toBeInTheDocument();
    expect(screen.getByText('指标')).toBeInTheDocument();
    expect(screen.getByText('额度')).toBeInTheDocument();
    expect(screen.getByText('已用')).toBeInTheDocument();
    expect(screen.getByText('使用率')).toBeInTheDocument();
    expect(screen.getByText('启用')).toBeInTheDocument();
    expect(screen.getByText('操作')).toBeInTheDocument();
  });

  it('displays quota data correctly', async () => {
    const mockedUserList = vi.mocked(userApi.list);
    const mockedDeptList = vi.mocked(deptApi.list);
    const mockedModelList = vi.mocked(modelApi.list);
    const mockedListQuotas = vi.mocked(billingApi.listQuotas);
    const mockedListDaily = vi.mocked(billingApi.listDaily);

    mockedUserList.mockResolvedValue({
      total: 1,
      list: [{ id: 1, username: 'admin', nickname: 'Admin', email: '', phone: '', dept_id: 1, status: 1 }],
      page: 1,
      page_size: 500,
    });
    mockedDeptList.mockResolvedValue([{ id: 1, name: '技术部', parent_id: 0, path: '1', sort: 1, status: 1 }]);
    mockedModelList.mockResolvedValue({
      total: 1,
      list: [{ id: 1, code: 'gpt-4', name: 'GPT-4', type: 'llm', provider: 'openai', endpoint: '', default_params: {}, capability_tags: [], enabled: 1, priority: 1, max_qps: 10, health_check_url: '', last_health_status: 1 }],
      page: 1,
      page_size: 200,
    });

    mockedListQuotas.mockResolvedValue([
      {
        id: 1,
        scope_type: 'user',
        scope_id: 1,
        model_id: 1,
        period: 'monthly',
        metric: 'calls',
        quota_value: 1000,
        used_value: 500,
        enabled: 1,
      },
    ]);
    mockedListDaily.mockResolvedValue([]);

    renderBilling();

    await waitFor(() => {
      expect(screen.getByText('用户')).toBeInTheDocument();
    });

    expect(screen.getByText('按月')).toBeInTheDocument();
    expect(screen.getByText('调用次数')).toBeInTheDocument();
    expect(screen.getByText('1,000')).toBeInTheDocument();
    expect(screen.getByText('500')).toBeInTheDocument();
  });

  it('shows empty state for quota list', async () => {
    const mockedUserList = vi.mocked(userApi.list);
    const mockedDeptList = vi.mocked(deptApi.list);
    const mockedModelList = vi.mocked(modelApi.list);
    const mockedListQuotas = vi.mocked(billingApi.listQuotas);
    const mockedListDaily = vi.mocked(billingApi.listDaily);

    mockedUserList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 500 });
    mockedDeptList.mockResolvedValue([]);
    mockedModelList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedListQuotas.mockResolvedValue([]);
    mockedListDaily.mockResolvedValue([]);

    renderBilling();

    await waitFor(() => {
      expect(screen.getByText('额度管理')).toBeInTheDocument();
    });

    // Table should show empty text (use queryAllByText and check at least one exists)
    expect(screen.queryAllByText('暂无数据').length).toBeGreaterThanOrEqual(1);
  });

  it('switches to usage tab and displays daily data', async () => {
    const mockedUserList = vi.mocked(userApi.list);
    const mockedDeptList = vi.mocked(deptApi.list);
    const mockedModelList = vi.mocked(modelApi.list);
    const mockedListQuotas = vi.mocked(billingApi.listQuotas);
    const mockedListDaily = vi.mocked(billingApi.listDaily);

    mockedUserList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 500 });
    mockedDeptList.mockResolvedValue([]);
    mockedModelList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedListQuotas.mockResolvedValue([]);
    mockedListDaily.mockResolvedValue([
      {
        id: 1,
        stat_date: '2024-01-01',
        model_id: 1,
        dept_id: 0,
        user_id: 1,
        calls: 100,
        input_tokens: 1000,
        output_tokens: 500,
        units: 10,
        cost: 0.5,
      },
      {
        id: 2,
        stat_date: '2024-01-02',
        model_id: 1,
        dept_id: 0,
        user_id: 1,
        calls: 200,
        input_tokens: 2000,
        output_tokens: 1000,
        units: 20,
        cost: 1.0,
      },
    ]);

    renderBilling();

    await waitFor(() => {
      expect(screen.getByText('额度管理')).toBeInTheDocument();
    });

    // Click on usage tab
    const usageTab = screen.getByText('用量统计');
    await userEvent.click(usageTab);

    await waitFor(() => {
      expect(screen.getByText('总调用数')).toBeInTheDocument();
    });

    expect(screen.getByText('总 Token 数')).toBeInTheDocument();
    expect(screen.getByText('总成本')).toBeInTheDocument();
    expect(screen.getByText('按日趋势')).toBeInTheDocument();
    expect(screen.getByText('明细')).toBeInTheDocument();
  });

  it('handles API errors gracefully', async () => {
    const mockedUserList = vi.mocked(userApi.list);
    const mockedDeptList = vi.mocked(deptApi.list);
    const mockedModelList = vi.mocked(modelApi.list);
    const mockedListQuotas = vi.mocked(billingApi.listQuotas);
    const mockedListDaily = vi.mocked(billingApi.listDaily);

    mockedUserList.mockRejectedValue(new Error('Network error'));
    mockedDeptList.mockRejectedValue(new Error('Network error'));
    mockedModelList.mockRejectedValue(new Error('Network error'));
    mockedListQuotas.mockRejectedValue(new Error('Network error'));
    mockedListDaily.mockRejectedValue(new Error('Network error'));

    renderBilling();

    await waitFor(() => {
      expect(screen.getByText('计费 / 额度管理')).toBeInTheDocument();
    });

    // Should still render the page structure
    expect(screen.getByText('额度管理')).toBeInTheDocument();
    expect(screen.getByText('用量统计')).toBeInTheDocument();
  });

  it('filters quotas by scope type', async () => {
    const mockedUserList = vi.mocked(userApi.list);
    const mockedDeptList = vi.mocked(deptApi.list);
    const mockedModelList = vi.mocked(modelApi.list);
    const mockedListQuotas = vi.mocked(billingApi.listQuotas);
    const mockedListDaily = vi.mocked(billingApi.listDaily);

    mockedUserList.mockResolvedValue({
      total: 1,
      list: [{ id: 1, username: 'admin', nickname: 'Admin', email: '', phone: '', dept_id: 1, status: 1 }],
      page: 1,
      page_size: 500,
    });
    mockedDeptList.mockResolvedValue([{ id: 1, name: '技术部', parent_id: 0, path: '1', sort: 1, status: 1 }]);
    mockedModelList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedListQuotas.mockResolvedValue([]);
    mockedListDaily.mockResolvedValue([]);

    renderBilling();

    await waitFor(() => {
      expect(screen.getByText('额度管理')).toBeInTheDocument();
    });

    // The scope type filter select should be present
    const scopeTypeSelect = screen.getByText('范围类型');
    expect(scopeTypeSelect).toBeInTheDocument();
  });

  it('opens create quota modal', async () => {
    const mockedUserList = vi.mocked(userApi.list);
    const mockedDeptList = vi.mocked(deptApi.list);
    const mockedModelList = vi.mocked(modelApi.list);
    const mockedListQuotas = vi.mocked(billingApi.listQuotas);
    const mockedListDaily = vi.mocked(billingApi.listDaily);

    mockedUserList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 500 });
    mockedDeptList.mockResolvedValue([]);
    mockedModelList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedListQuotas.mockResolvedValue([]);
    mockedListDaily.mockResolvedValue([]);

    renderBilling();

    await waitFor(() => {
      expect(screen.getByText('新建额度')).toBeInTheDocument();
    });

    // Use the button specifically (first occurrence)
    const createButton = screen.getAllByText('新建额度')[0];
    await userEvent.click(createButton);

    await waitFor(() => {
      // Modal title should now also be present
      expect(screen.getAllByText('新建额度').length).toBeGreaterThanOrEqual(2);
    });

    // Modal form fields
    expect(screen.getByText('范围类型')).toBeInTheDocument();
    expect(screen.getByText('范围对象')).toBeInTheDocument();
    expect(screen.getByText('模型 (0=全部模型)')).toBeInTheDocument();
    expect(screen.getByText('周期')).toBeInTheDocument();
    expect(screen.getByText('指标')).toBeInTheDocument();
    expect(screen.getByText('额度值')).toBeInTheDocument();
    expect(screen.getByText('启用')).toBeInTheDocument();
  });

  it('displays cost formatted correctly', async () => {
    const mockedUserList = vi.mocked(userApi.list);
    const mockedDeptList = vi.mocked(deptApi.list);
    const mockedModelList = vi.mocked(modelApi.list);
    const mockedListQuotas = vi.mocked(billingApi.listQuotas);
    const mockedListDaily = vi.mocked(billingApi.listDaily);

    mockedUserList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 500 });
    mockedDeptList.mockResolvedValue([]);
    mockedModelList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedListQuotas.mockResolvedValue([
      {
        id: 1,
        scope_type: 'user',
        scope_id: 1,
        model_id: 0,
        period: 'monthly',
        metric: 'cost',
        quota_value: 1000,
        used_value: 500.5,
        enabled: 1,
      },
    ]);
    mockedListDaily.mockResolvedValue([]);

    renderBilling();

    await waitFor(() => {
      expect(screen.getByText('成本(元)')).toBeInTheDocument();
    });

    // Cost should be formatted
    expect(screen.getByText('¥1,000.00')).toBeInTheDocument();
    expect(screen.getByText('¥500.50')).toBeInTheDocument();
  });

  it('calls listQuotas with correct parameters', async () => {
    const mockedUserList = vi.mocked(userApi.list);
    const mockedDeptList = vi.mocked(deptApi.list);
    const mockedModelList = vi.mocked(modelApi.list);
    const mockedListQuotas = vi.mocked(billingApi.listQuotas);
    const mockedListDaily = vi.mocked(billingApi.listDaily);

    mockedUserList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 500 });
    mockedDeptList.mockResolvedValue([]);
    mockedModelList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedListQuotas.mockResolvedValue([]);
    mockedListDaily.mockResolvedValue([]);

    renderBilling();

    await waitFor(() => {
      expect(mockedListQuotas).toHaveBeenCalledWith({});
    });
  });
});
