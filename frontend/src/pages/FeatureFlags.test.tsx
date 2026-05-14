import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import FeatureFlagsPage from './FeatureFlags';
import { featureFlagApi } from '@/api/modules';

// Mock API modules
vi.mock('@/api/modules', async () => {
  const actual = await vi.importActual<typeof import('@/api/modules')>('@/api/modules');
  return {
    ...actual,
    featureFlagApi: {
      list: vi.fn(),
      create: vi.fn(),
      update: vi.fn(),
      delete: vi.fn(),
      evaluate: vi.fn(),
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
Object.assign(navigator, {
  clipboard: {
    writeText: vi.fn().mockResolvedValue(undefined),
  },
});

function renderFeatureFlags() {
  return render(<FeatureFlagsPage />);
}

describe('FeatureFlagsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders page with title', async () => {
    const mockedList = vi.mocked(featureFlagApi.list);
    mockedList.mockResolvedValue([]);

    renderFeatureFlags();

    await waitFor(() => {
      expect(screen.getByText('灰度开关管理')).toBeInTheDocument();
    });
  });

  it('shows loading state initially', async () => {
    const mockedList = vi.mocked(featureFlagApi.list);
    mockedList.mockReturnValue(new Promise(() => {}));

    renderFeatureFlags();

    await waitFor(() => {
      expect(document.querySelector('[aria-busy="true"]')).toBeInTheDocument();
    });
  });

  it('displays feature flags list after data loads', async () => {
    const mockedList = vi.mocked(featureFlagApi.list);
    const mockFlags = [
      {
        id: 1,
        key: 'video.publish.batch',
        description: '批量发布视频',
        enabled: 1,
        rollout: 50,
        rules: { users: [1, 2], depts: [10], projects: [] },
        created_at: '2024-01-01T12:00:00Z',
        updated_at: '2024-01-01T12:00:00Z',
      },
      {
        id: 2,
        key: 'image.gen.hd',
        description: '高清图片生成',
        enabled: 0,
        rollout: 0,
        rules: {},
        created_at: '2024-01-02T10:00:00Z',
        updated_at: '2024-01-02T10:00:00Z',
      },
    ];
    mockedList.mockResolvedValue(mockFlags);

    renderFeatureFlags();

    await waitFor(() => {
      expect(screen.getByText('video.publish.batch')).toBeInTheDocument();
    });

    expect(screen.getByText('image.gen.hd')).toBeInTheDocument();
    expect(screen.getByText('批量发布视频')).toBeInTheDocument();
    expect(screen.getByText('高清图片生成')).toBeInTheDocument();
    expect(screen.getByText('用户:2 / 部门:1 / 项目:0')).toBeInTheDocument();
    expect(screen.getByText('-')).toBeInTheDocument();
  });

  it('displays empty state when no flags exist', async () => {
    const mockedList = vi.mocked(featureFlagApi.list);
    mockedList.mockResolvedValue([]);

    renderFeatureFlags();

    await waitFor(() => {
      expect(screen.getByText('还没有灰度开关')).toBeInTheDocument();
    });

    expect(screen.getByRole('button', { name: /新建开关/i })).toBeInTheDocument();
  });

  it('handles API error gracefully', async () => {
    const mockedList = vi.mocked(featureFlagApi.list);
    mockedList.mockRejectedValue(new Error('Network error'));

    renderFeatureFlags();

    await waitFor(() => {
      expect(screen.getByText('灰度开关管理')).toBeInTheDocument();
    });

    // Should show empty state after error
    await waitFor(() => {
      expect(screen.getByText('还没有灰度开关')).toBeInTheDocument();
    });
  });

  it('toggles feature flag enabled state', async () => {
    const user = userEvent.setup();
    const mockedList = vi.mocked(featureFlagApi.list);
    const mockedUpdate = vi.mocked(featureFlagApi.update);

    const mockFlag = {
      id: 1,
      key: 'test.flag',
      description: 'Test flag',
      enabled: 0,
      rollout: 0,
      rules: {},
      created_at: '2024-01-01T12:00:00Z',
      updated_at: '2024-01-01T12:00:00Z',
    };
    mockedList.mockResolvedValue([mockFlag]);
    mockedUpdate.mockResolvedValue({} as never);

    renderFeatureFlags();

    await waitFor(() => {
      expect(screen.getByText('test.flag')).toBeInTheDocument();
    });

    const switchBtn = screen.getByRole('switch');
    expect(switchBtn).toHaveAttribute('aria-checked', 'false');

    await user.click(switchBtn);

    await waitFor(() => {
      expect(mockedUpdate).toHaveBeenCalledWith(1, { enabled: 1 });
    });
  });

  it('opens create modal when clicking new button', async () => {
    const user = userEvent.setup();
    const mockedList = vi.mocked(featureFlagApi.list);
    mockedList.mockResolvedValue([]);

    renderFeatureFlags();

    await waitFor(() => {
      expect(screen.getByText('灰度开关管理')).toBeInTheDocument();
    });

    const newBtn = screen.getByRole('button', { name: /新建开关/i });
    await user.click(newBtn);

    await waitFor(() => {
      expect(screen.getByText('新建开关')).toBeInTheDocument();
    });

    expect(screen.getByLabelText(/Key/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/描述/i)).toBeInTheDocument();
    expect(screen.getByText(/灰度比例/i)).toBeInTheDocument();
    expect(screen.getByText(/命中规则/i)).toBeInTheDocument();
  });

  it('opens edit modal with pre-filled data', async () => {
    const user = userEvent.setup();
    const mockedList = vi.mocked(featureFlagApi.list);

    const mockFlag = {
      id: 1,
      key: 'video.publish.batch',
      description: '批量发布视频',
      enabled: 1,
      rollout: 75,
      rules: { users: [1, 2], depts: [], projects: [5] },
      created_at: '2024-01-01T12:00:00Z',
      updated_at: '2024-01-01T12:00:00Z',
    };
    mockedList.mockResolvedValue([mockFlag]);

    renderFeatureFlags();

    await waitFor(() => {
      expect(screen.getByText('video.publish.batch')).toBeInTheDocument();
    });

    const editBtn = screen.getByRole('button', { name: /编辑/i });
    await user.click(editBtn);

    await waitFor(() => {
      expect(screen.getByText('编辑开关 - video.publish.batch')).toBeInTheDocument();
    });
  });

  it('opens evaluate modal and evaluates feature flag', async () => {
    const user = userEvent.setup();
    const mockedList = vi.mocked(featureFlagApi.list);
    const mockedEvaluate = vi.mocked(featureFlagApi.evaluate);

    mockedList.mockResolvedValue([]);
    mockedEvaluate.mockResolvedValue({ key: 'test.flag', enabled: true });

    renderFeatureFlags();

    await waitFor(() => {
      expect(screen.getByText('灰度开关管理')).toBeInTheDocument();
    });

    const evalBtn = screen.getByRole('button', { name: /评估/i });
    await user.click(evalBtn);

    await waitFor(() => {
      expect(screen.getByText('开关评估')).toBeInTheDocument();
    });

    const input = screen.getByPlaceholderText('输入要评估的 key');
    await user.type(input, 'test.flag');

    const okBtn = screen.getByRole('button', { name: /评估/i });
    await user.click(okBtn);

    await waitFor(() => {
      expect(mockedEvaluate).toHaveBeenCalledWith('test.flag');
    });

    await waitFor(() => {
      expect(screen.getByText('命中(enabled)')).toBeInTheDocument();
    });
  });

  it('deletes feature flag after confirmation', async () => {
    const user = userEvent.setup();
    const mockedList = vi.mocked(featureFlagApi.list);
    const mockedDelete = vi.mocked(featureFlagApi.delete);

    const mockFlag = {
      id: 1,
      key: 'test.flag',
      description: 'Test flag',
      enabled: 1,
      rollout: 0,
      rules: {},
      created_at: '2024-01-01T12:00:00Z',
      updated_at: '2024-01-01T12:00:00Z',
    };
    mockedList.mockResolvedValue([mockFlag]);
    mockedDelete.mockResolvedValue({} as never);

    renderFeatureFlags();

    await waitFor(() => {
      expect(screen.getByText('test.flag')).toBeInTheDocument();
    });

    const deleteBtn = screen.getByRole('button', { name: /删除/i });
    await user.click(deleteBtn);

    await waitFor(() => {
      expect(screen.getByText('确认删除该开关?')).toBeInTheDocument();
    });

    const confirmBtn = screen.getByRole('button', { name: /确定/i });
    await user.click(confirmBtn);

    await waitFor(() => {
      expect(mockedDelete).toHaveBeenCalledWith(1);
    });
  });

  it('calls list API on mount', async () => {
    const mockedList = vi.mocked(featureFlagApi.list);
    mockedList.mockResolvedValue([]);

    renderFeatureFlags();

    await waitFor(() => {
      expect(mockedList).toHaveBeenCalledTimes(1);
    });
  });

  it('displays correct action buttons for each row', async () => {
    const mockedList = vi.mocked(featureFlagApi.list);

    const mockFlag = {
      id: 1,
      key: 'test.flag',
      description: 'Test flag',
      enabled: 1,
      rollout: 0,
      rules: {},
      created_at: '2024-01-01T12:00:00Z',
      updated_at: '2024-01-01T12:00:00Z',
    };
    mockedList.mockResolvedValue([mockFlag]);

    renderFeatureFlags();

    await waitFor(() => {
      expect(screen.getByText('test.flag')).toBeInTheDocument();
    });

    expect(screen.getByRole('button', { name: /编辑/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /评估/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /删除/i })).toBeInTheDocument();
  });
});
