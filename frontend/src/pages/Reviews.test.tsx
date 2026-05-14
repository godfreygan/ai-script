import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import ReviewsPage from './Reviews';
import { reviewApi, fullVideoApi } from '@/api/modules';

const messageMock = {
  success: vi.fn(),
  error: vi.fn(),
  warning: vi.fn(),
};

// Mock API modules
vi.mock('@/api/modules', async () => {
  const actual = await vi.importActual<typeof import('@/api/modules')>('@/api/modules');
  return {
    ...actual,
    reviewApi: {
      listFlows: vi.fn(),
      listNodes: vi.fn(),
      submit: vi.fn(),
      listRecords: vi.fn(),
      getRecord: vi.fn(),
      listActions: vi.fn(),
      act: vi.fn(),
      cancel: vi.fn(),
    },
    fullVideoApi: {
      list: vi.fn(),
    },
  };
});

// Mock antd App.useApp
vi.mock('antd', async () => {
  const actual = await vi.importActual<typeof import('antd')>('antd');
  return {
    ...actual,
    App: {
      ...actual.App,
      useApp: () => ({ message: messageMock }),
    },
  };
});

// Mock auth store
vi.mock('@/stores/auth', () => ({
  useAuthStore: (selector: (s: { user: { id: number } }) => any) => selector({ user: { id: 1 } }),
}));

function renderReviews() {
  return render(<ReviewsPage />);
}

describe('ReviewsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders page with title and tabs', async () => {
    vi.mocked(reviewApi.listRecords).mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });
    vi.mocked(reviewApi.listFlows).mockResolvedValue([]);
    vi.mocked(fullVideoApi.list).mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });

    renderReviews();

    expect(screen.getByText('审核管理')).toBeInTheDocument();
    expect(screen.getByText('审核记录')).toBeInTheDocument();
    expect(screen.getByText('审核流配置')).toBeInTheDocument();
  });

  it('displays review records in table', async () => {
    const mockRecords = [
      {
        id: 1,
        target_type: 'full_video',
        target_id: 10,
        flow_id: 1,
        current_step: 1,
        status: 'pending',
        submitted_by: 1,
        created_at: '2024-01-01T10:00:00Z',
        updated_at: '2024-01-01T10:00:00Z',
      },
      {
        id: 2,
        target_type: 'full_video',
        target_id: 20,
        flow_id: 1,
        current_step: 2,
        status: 'approved',
        submitted_by: 2,
        created_at: '2024-01-02T10:00:00Z',
        updated_at: '2024-01-02T10:00:00Z',
      },
    ];

    vi.mocked(reviewApi.listRecords).mockResolvedValue({ total: 2, list: mockRecords, page: 1, page_size: 20 });
    vi.mocked(reviewApi.listFlows).mockResolvedValue([]);
    vi.mocked(fullVideoApi.list).mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });

    renderReviews();

    await waitFor(() => {
      expect(screen.getByText('#1')).toBeInTheDocument();
    });

    expect(screen.getByText('待审核')).toBeInTheDocument();
    expect(screen.getByText('已通过')).toBeInTheDocument();
    expect(screen.getAllByText('第 1 步').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('第 2 步')).toBeInTheDocument();
  });

  it('shows empty state when no records', async () => {
    vi.mocked(reviewApi.listRecords).mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });
    vi.mocked(reviewApi.listFlows).mockResolvedValue([]);
    vi.mocked(fullVideoApi.list).mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });

    renderReviews();

    await waitFor(() => {
      expect(screen.getByText('暂无审核记录')).toBeInTheDocument();
    });
  });

  it('opens drawer and shows record details', async () => {
    const mockRecord = {
      id: 1,
      target_type: 'full_video',
      target_id: 10,
      flow_id: 1,
      current_step: 1,
      status: 'pending',
      submitted_by: 1,
      created_at: '2024-01-01T10:00:00Z',
      updated_at: '2024-01-01T10:00:00Z',
    };

    vi.mocked(reviewApi.listRecords).mockResolvedValue({ total: 1, list: [mockRecord], page: 1, page_size: 20 });
    vi.mocked(reviewApi.listFlows).mockResolvedValue([]);
    vi.mocked(fullVideoApi.list).mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    vi.mocked(reviewApi.listActions).mockResolvedValue([]);
    vi.mocked(reviewApi.listNodes).mockResolvedValue([
      { id: 1, flow_id: 1, step_no: 1, name: '初审', approver_type: 'user', approver_value: '1', allow_timeout_pass: 0, timeout_hours: 24 },
    ]);

    renderReviews();

    await waitFor(() => {
      expect(screen.getByText('详情')).toBeInTheDocument();
    });

    await userEvent.click(screen.getByText('详情'));

    await waitFor(() => {
      expect(screen.getByText('审核记录 #1')).toBeInTheDocument();
    });

    expect(screen.getByText('full_video')).toBeInTheDocument();
    expect(screen.getByText('(初审)')).toBeInTheDocument();
  });

  it('submits a review action (approve)', async () => {
    const mockRecord = {
      id: 1,
      target_type: 'full_video',
      target_id: 10,
      flow_id: 1,
      current_step: 1,
      status: 'pending',
      submitted_by: 1,
      created_at: '2024-01-01T10:00:00Z',
      updated_at: '2024-01-01T10:00:00Z',
    };

    vi.mocked(reviewApi.listRecords).mockResolvedValue({ total: 1, list: [mockRecord], page: 1, page_size: 20 });
    vi.mocked(reviewApi.listFlows).mockResolvedValue([]);
    vi.mocked(fullVideoApi.list).mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    vi.mocked(reviewApi.listActions).mockResolvedValue([]);
    vi.mocked(reviewApi.listNodes).mockResolvedValue([
      { id: 1, flow_id: 1, step_no: 1, name: '初审', approver_type: 'user', approver_value: '1', allow_timeout_pass: 1, timeout_hours: 24 },
    ]);
    vi.mocked(reviewApi.act).mockResolvedValue({ ...mockRecord, status: 'approved' } as any);
    vi.mocked(reviewApi.getRecord).mockResolvedValue({ ...mockRecord, status: 'approved' });

    renderReviews();

    await waitFor(() => {
      expect(screen.getByText('详情')).toBeInTheDocument();
    });

    await userEvent.click(screen.getByText('详情'));

    await waitFor(() => {
      expect(screen.getByText('通过')).toBeInTheDocument();
    });

    await userEvent.click(screen.getByText('通过'));

    await waitFor(() => {
      expect(vi.mocked(reviewApi.act)).toHaveBeenCalledWith(1, { action: 'approve', comment: undefined });
    });
  });

  it('submits a review record', async () => {
    vi.mocked(reviewApi.listRecords).mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });
    vi.mocked(reviewApi.listFlows).mockResolvedValue([
      { id: 1, name: '默认审核流', description: '', target_type: 'full_video', enabled: 1, is_default: 1 },
    ]);
    vi.mocked(fullVideoApi.list).mockResolvedValue({
      total: 1,
      list: [{ id: 10, name: '测试视频', project_id: 1, version: 1, timeline: { clips: [] }, output_url: '', thumb_url: '', cover_url: '', duration_ms: 0, status: 'succeeded', render_progress: 100, error_msg: '', created_by: 1, created_at: '', updated_at: '' }],
      page: 1,
      page_size: 200,
    });
    vi.mocked(reviewApi.submit).mockResolvedValue({
      id: 3,
      target_type: 'full_video',
      target_id: 10,
      flow_id: 1,
      current_step: 1,
      status: 'pending',
      submitted_by: 1,
      created_at: '2024-01-01T10:00:00Z',
      updated_at: '2024-01-01T10:00:00Z',
    } as any);

    renderReviews();

    await waitFor(() => {
      expect(screen.getByText('提交审核')).toBeInTheDocument();
    });

    await userEvent.click(screen.getByText('提交审核'));

    await waitFor(() => {
      // Modal title should appear (disambiguate from the page button by using a heading query)
      expect(screen.getByRole('dialog')).toBeInTheDocument();
    });

    // Select target video
    const videoSelect = screen.getByLabelText('目标视频');
    await userEvent.click(videoSelect);

    await waitFor(() => {
      expect(screen.getByText('测试视频 (#10)')).toBeInTheDocument();
    });

    await userEvent.click(screen.getByText('测试视频 (#10)'));

    // Click the OK/submit button in the modal footer
    const submitBtn = document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement;
    expect(submitBtn).toBeTruthy();
    await userEvent.click(submitBtn);

    await waitFor(() => {
      expect(vi.mocked(reviewApi.submit)).toHaveBeenCalledWith(
        expect.objectContaining({
          target_type: 'full_video',
          target_id: 10,
        })
      );
    });
  });

  it('handles API errors gracefully', async () => {
    vi.mocked(reviewApi.listRecords).mockRejectedValue(new Error('Network error'));
    vi.mocked(reviewApi.listFlows).mockRejectedValue(new Error('Network error'));
    vi.mocked(fullVideoApi.list).mockRejectedValue(new Error('Network error'));

    renderReviews();

    await waitFor(() => {
      expect(screen.getByText('审核管理')).toBeInTheDocument();
    });

    expect(screen.getByText('审核记录')).toBeInTheDocument();
    expect(screen.getByText('审核流配置')).toBeInTheDocument();
  });

  it('switches to flows tab and displays flows', async () => {
    const mockFlows = [
      { id: 1, name: '视频审核流', description: '', target_type: 'full_video', enabled: 1, is_default: 1 },
      { id: 2, name: '图片审核流', description: '', target_type: 'image', enabled: 0, is_default: 0 },
    ];

    vi.mocked(reviewApi.listRecords).mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });
    vi.mocked(reviewApi.listFlows).mockResolvedValue(mockFlows);
    vi.mocked(fullVideoApi.list).mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });

    renderReviews();

    await waitFor(() => {
      expect(screen.getByText('审核流配置')).toBeInTheDocument();
    });

    await userEvent.click(screen.getByText('审核流配置'));

    await waitFor(() => {
      expect(screen.getByText('视频审核流')).toBeInTheDocument();
    });

    // "默认" appears in the flows table; use getAllByText and verify at least one
    expect(screen.getAllByText('默认').length).toBeGreaterThanOrEqual(1);
  });

  it('cancels a pending review record', async () => {
    const mockRecord = {
      id: 1,
      target_type: 'full_video',
      target_id: 10,
      flow_id: 1,
      current_step: 1,
      status: 'pending',
      submitted_by: 1,
      created_at: '2024-01-01T10:00:00Z',
      updated_at: '2024-01-01T10:00:00Z',
    };

    vi.mocked(reviewApi.listRecords).mockResolvedValue({ total: 1, list: [mockRecord], page: 1, page_size: 20 });
    vi.mocked(reviewApi.listFlows).mockResolvedValue([]);
    vi.mocked(fullVideoApi.list).mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    vi.mocked(reviewApi.listActions).mockResolvedValue([]);
    vi.mocked(reviewApi.listNodes).mockResolvedValue([
      { id: 1, flow_id: 1, step_no: 1, name: '初审', approver_type: 'user', approver_value: '1', allow_timeout_pass: 0, timeout_hours: 24 },
    ]);
    vi.mocked(reviewApi.cancel).mockResolvedValue({} as any);
    vi.mocked(reviewApi.getRecord).mockResolvedValue({ ...mockRecord, status: 'cancelled' });

    renderReviews();

    await waitFor(() => {
      expect(screen.getByText('详情')).toBeInTheDocument();
    });

    await userEvent.click(screen.getByText('详情'));

    await waitFor(() => {
      expect(screen.getByText('审核记录 #1')).toBeInTheDocument();
    });

    // Find the 撤回 button by its text inside the drawer
    const cancelBtn = screen.getByText('撤回').closest('button') as HTMLElement;
    expect(cancelBtn).toBeTruthy();
    await userEvent.click(cancelBtn);

    await waitFor(() => {
      expect(vi.mocked(reviewApi.cancel)).toHaveBeenCalledWith(1);
    });
  });
});
