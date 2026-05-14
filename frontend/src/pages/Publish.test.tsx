import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import PublishPage from './Publish';
import { publishApi, fullVideoApi } from '@/api/modules';

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
    publishApi: {
      list: vi.fn(),
      publish: vi.fn(),
      unpublish: vi.fn(),
      updateWatermark: vi.fn(),
      incPlay: vi.fn(),
      incDownload: vi.fn(),
    },
    fullVideoApi: {
      list: vi.fn(),
      get: vi.fn(),
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

// Mock clipboard
Object.assign(navigator, {
  clipboard: {
    writeText: vi.fn(),
  },
});

function renderPublish() {
  return render(<PublishPage />);
}

describe('PublishPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders page with title and statistics', async () => {
    vi.mocked(publishApi.list).mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });

    renderPublish();

    expect(screen.getByText('发布管理')).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText('当前页发布数')).toBeInTheDocument();
    });

    expect(screen.getByText('本月新发布')).toBeInTheDocument();
    expect(screen.getByText('累计播放(当前页)')).toBeInTheDocument();
  });

  it('displays publish items in table', async () => {
    const mockItems = [
      {
        id: 1,
        full_video_id: 10,
        published_by: 1,
        published_at: '2024-01-01T10:00:00Z',
        status: 'on',
        watermark_config: { text: '@AI短剧', position: 'bottom-right', opacity: 0.6 },
        download_count: 5,
        play_count: 100,
        updated_at: '2024-01-01T10:00:00Z',
      },
      {
        id: 2,
        full_video_id: 20,
        published_by: 2,
        published_at: '2024-01-02T10:00:00Z',
        status: 'off',
        watermark_config: {},
        download_count: 0,
        play_count: 0,
        updated_at: '2024-01-02T10:00:00Z',
      },
    ];

    vi.mocked(publishApi.list).mockResolvedValue({ total: 2, list: mockItems, page: 1, page_size: 20 });

    renderPublish();

    await waitFor(() => {
      expect(screen.getByText('已发布')).toBeInTheDocument();
    });

    expect(screen.getByText('已下架')).toBeInTheDocument();
    expect(screen.getAllByText('100').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('5')).toBeInTheDocument();
  });

  it('shows empty state with publish button', async () => {
    vi.mocked(publishApi.list).mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });

    renderPublish();

    await waitFor(() => {
      expect(screen.getByText('暂无发布记录')).toBeInTheDocument();
    });

    const emptyBtn = screen.getAllByText('发布新视频').find((el) =>
      el.closest('.ant-empty')
    );
    expect(emptyBtn).toBeTruthy();
  });

  it('opens publish modal and submits new publish', async () => {
    vi.mocked(publishApi.list).mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });
    vi.mocked(fullVideoApi.list).mockResolvedValue({
      total: 1,
      list: [
        { id: 10, name: '测试视频', project_id: 1, version: 1, timeline: { clips: [] }, output_url: '', thumb_url: '', cover_url: '', duration_ms: 0, status: 'succeeded', render_progress: 100, error_msg: '', created_by: 1, created_at: '', updated_at: '' },
      ],
      page: 1,
      page_size: 200,
    });
    vi.mocked(publishApi.publish).mockResolvedValue({
      id: 1,
      full_video_id: 10,
      published_by: 1,
      published_at: '2024-01-01T10:00:00Z',
      status: 'on',
      watermark_config: {},
      download_count: 0,
      play_count: 0,
      updated_at: '2024-01-01T10:00:00Z',
    });

    renderPublish();

    await waitFor(() => {
      expect(screen.getAllByText('发布新视频').length).toBeGreaterThanOrEqual(1);
    });

    // Click the card-extra primary button (not the empty-state one)
    const publishBtn = screen.getAllByText('发布新视频').find((el) =>
      el.closest('button')?.classList.contains('ant-btn-primary')
    );
    expect(publishBtn).toBeTruthy();
    await userEvent.click(publishBtn!);

    await waitFor(() => {
      expect(screen.getByText('选择完整视频')).toBeInTheDocument();
    });

    const videoSelect = screen.getByLabelText('选择完整视频');
    await userEvent.click(videoSelect);

    await waitFor(() => {
      expect(screen.getByText('#10 测试视频')).toBeInTheDocument();
    });

    await userEvent.click(screen.getByText('#10 测试视频'));

    // Click modal OK button
    const okBtn = document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement;
    expect(okBtn).toBeTruthy();
    await userEvent.click(okBtn);

    await waitFor(() => {
      expect(vi.mocked(publishApi.publish)).toHaveBeenCalledWith(
        expect.objectContaining({
          full_video_id: 10,
        })
      );
    });
  });

  it('unpublishes a published item', async () => {
    const mockItems = [
      {
        id: 1,
        full_video_id: 10,
        published_by: 1,
        published_at: '2024-01-01T10:00:00Z',
        status: 'on',
        watermark_config: {},
        download_count: 0,
        play_count: 0,
        updated_at: '2024-01-01T10:00:00Z',
      },
    ];

    vi.mocked(publishApi.list).mockResolvedValue({ total: 1, list: mockItems, page: 1, page_size: 20 });
    vi.mocked(publishApi.unpublish).mockResolvedValue({} as any);

    renderPublish();

    await waitFor(() => {
      expect(screen.getByText('已发布')).toBeInTheDocument();
    });

    // Popconfirm wraps the button; click the danger button first, then confirm in the popover
    const dangerBtn = document.querySelector('button.ant-btn-dangerous') as HTMLElement;
    expect(dangerBtn).toBeTruthy();
    await userEvent.click(dangerBtn);

    // Antd Popconfirm renders a confirmation popup; click its OK button
    await waitFor(() => {
      const confirmOk = document.querySelector('.ant-popconfirm-buttons .ant-btn-primary') as HTMLElement;
      expect(confirmOk).toBeTruthy();
    });

    const confirmOk = document.querySelector('.ant-popconfirm-buttons .ant-btn-primary') as HTMLElement;
    await userEvent.click(confirmOk);

    await waitFor(() => {
      expect(vi.mocked(publishApi.unpublish)).toHaveBeenCalledWith(10);
    });
  });

  it('republishes an unpublished item', async () => {
    const mockItems = [
      {
        id: 1,
        full_video_id: 10,
        published_by: 1,
        published_at: '2024-01-01T10:00:00Z',
        status: 'off',
        watermark_config: {},
        download_count: 0,
        play_count: 0,
        updated_at: '2024-01-01T10:00:00Z',
      },
    ];

    vi.mocked(publishApi.list).mockResolvedValue({ total: 1, list: mockItems, page: 1, page_size: 20 });
    vi.mocked(publishApi.publish).mockResolvedValue({
      id: 1,
      full_video_id: 10,
      published_by: 1,
      published_at: '2024-01-01T10:00:00Z',
      status: 'on',
      watermark_config: {},
      download_count: 0,
      play_count: 0,
      updated_at: '2024-01-01T10:00:00Z',
    });

    renderPublish();

    await waitFor(() => {
      expect(screen.getByText('重新发布')).toBeInTheDocument();
    });

    await userEvent.click(screen.getByText('重新发布'));

    await waitFor(() => {
      expect(vi.mocked(publishApi.publish)).toHaveBeenCalledWith(
        expect.objectContaining({
          full_video_id: 10,
        })
      );
    });
  });

  it('opens watermark editor and saves', async () => {
    const mockItems = [
      {
        id: 1,
        full_video_id: 10,
        published_by: 1,
        published_at: '2024-01-01T10:00:00Z',
        status: 'on',
        watermark_config: { text: '@AI短剧', position: 'bottom-right', opacity: 0.6 },
        download_count: 0,
        play_count: 0,
        updated_at: '2024-01-01T10:00:00Z',
      },
    ];

    vi.mocked(publishApi.list).mockResolvedValue({ total: 1, list: mockItems, page: 1, page_size: 20 });
    vi.mocked(publishApi.updateWatermark).mockResolvedValue({} as any);

    renderPublish();

    await waitFor(() => {
      expect(screen.getAllByText('水印').length).toBeGreaterThanOrEqual(1);
    });

    const watermarkBtn = screen.getAllByText('水印').find((el) =>
      el.closest('button')
    );
    expect(watermarkBtn).toBeTruthy();
    await userEvent.click(watermarkBtn!);

    await waitFor(() => {
      expect(screen.getByText('编辑水印 (Video #10)')).toBeInTheDocument();
    });

    const textInput = screen.getByPlaceholderText('例如 @AI短剧');
    await userEvent.clear(textInput);
    await userEvent.type(textInput, '@NewMark');

    const okBtn = document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement;
    expect(okBtn).toBeTruthy();
    await userEvent.click(okBtn);

    await waitFor(() => {
      expect(vi.mocked(publishApi.updateWatermark)).toHaveBeenCalledWith(
        10,
        expect.objectContaining({ text: '@NewMark' })
      );
    });
  });

  it('opens preview drawer and shows video info', async () => {
    const mockItems = [
      {
        id: 1,
        full_video_id: 10,
        published_by: 1,
        published_at: '2024-01-01T10:00:00Z',
        status: 'on',
        watermark_config: {},
        download_count: 3,
        play_count: 50,
        updated_at: '2024-01-01T10:00:00Z',
      },
    ];

    vi.mocked(publishApi.list).mockResolvedValue({ total: 1, list: mockItems, page: 1, page_size: 20 });
    vi.mocked(fullVideoApi.get).mockResolvedValue({
      id: 10,
      name: '测试视频',
      project_id: 1,
      version: 2,
      timeline: { clips: [] },
      output_url: 'https://example.com/video.mp4',
      thumb_url: 'https://example.com/thumb.jpg',
      cover_url: 'https://example.com/cover.jpg',
      duration_ms: 15000,
      status: 'succeeded',
      render_progress: 100,
      error_msg: '',
      created_by: 1,
      created_at: '',
      updated_at: '',
    });
    vi.mocked(publishApi.incPlay).mockResolvedValue({} as any);

    renderPublish();

    await waitFor(() => {
      expect(screen.getAllByText('预览').length).toBeGreaterThanOrEqual(1);
    });

    const previewBtn = screen.getAllByText('预览').find((el) =>
      el.closest('button')
    );
    expect(previewBtn).toBeTruthy();
    await userEvent.click(previewBtn!);

    await waitFor(() => {
      expect(screen.getByText('预览发布 #1')).toBeInTheDocument();
    });

    expect(screen.getByText('#10 测试视频')).toBeInTheDocument();
    expect(screen.getByText('15.0s')).toBeInTheDocument();
    expect(screen.getByText('v2')).toBeInTheDocument();
  });

  it('copies share link in preview drawer', async () => {
    const mockItems = [
      {
        id: 1,
        full_video_id: 10,
        published_by: 1,
        published_at: '2024-01-01T10:00:00Z',
        status: 'on',
        watermark_config: {},
        download_count: 0,
        play_count: 0,
        updated_at: '2024-01-01T10:00:00Z',
      },
    ];

    vi.mocked(publishApi.list).mockResolvedValue({ total: 1, list: mockItems, page: 1, page_size: 20 });
    vi.mocked(fullVideoApi.get).mockResolvedValue({
      id: 10,
      name: '测试视频',
      project_id: 1,
      version: 1,
      timeline: { clips: [] },
      output_url: 'https://example.com/video.mp4',
      thumb_url: '',
      cover_url: '',
      duration_ms: 0,
      status: 'succeeded',
      render_progress: 100,
      error_msg: '',
      created_by: 1,
      created_at: '',
      updated_at: '',
    });
    vi.mocked(publishApi.incPlay).mockResolvedValue({} as any);
    vi.mocked(navigator.clipboard.writeText).mockResolvedValue(undefined);

    renderPublish();

    await waitFor(() => {
      expect(screen.getAllByText('预览').length).toBeGreaterThanOrEqual(1);
    });

    const previewBtn = screen.getAllByText('预览').find((el) =>
      el.closest('button')
    );
    expect(previewBtn).toBeTruthy();
    await userEvent.click(previewBtn!);

    await waitFor(() => {
      expect(screen.getByText('复制分享链接')).toBeInTheDocument();
    });

    await userEvent.click(screen.getByText('复制分享链接'));

    await waitFor(() => {
      expect(vi.mocked(navigator.clipboard.writeText)).toHaveBeenCalledWith('https://example.com/video.mp4');
    });
  });

  it('handles API errors gracefully', async () => {
    vi.mocked(publishApi.list).mockRejectedValue(new Error('Network error'));

    renderPublish();

    await waitFor(() => {
      expect(screen.getByText('发布管理')).toBeInTheDocument();
    });

    expect(screen.getByText('当前页发布数')).toBeInTheDocument();
  });

  it('refreshes list when refresh button clicked', async () => {
    vi.mocked(publishApi.list).mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });

    renderPublish();

    await waitFor(() => {
      expect(screen.getByText('刷新')).toBeInTheDocument();
    });

    await userEvent.click(screen.getByText('刷新'));

    await waitFor(() => {
      expect(vi.mocked(publishApi.list)).toHaveBeenCalledTimes(2);
    });
  });
});
