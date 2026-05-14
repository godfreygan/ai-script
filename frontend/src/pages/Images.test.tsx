import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import ImagesPage from './Images';
import { projectApi, modelApi, styleApi, imageApi } from '@/api/modules';

// Mock API modules
vi.mock('@/api/modules', async () => {
  const actual = await vi.importActual<typeof import('@/api/modules')>('@/api/modules');
  return {
    ...actual,
    projectApi: {
      list: vi.fn(),
    },
    modelApi: {
      list: vi.fn(),
    },
    styleApi: {
      list: vi.fn(),
    },
    imageApi: {
      list: vi.fn(),
      generate: vi.fn(),
      delete: vi.fn(),
    },
  };
});

// Mock useProgressWS hook
vi.mock('@/hooks/useProgressWS', () => ({
  useProgressWS: vi.fn(() => ({
    events: [],
    last: null,
    connected: false,
  })),
}));

function renderImages() {
  return render(<ImagesPage />);
}

describe('ImagesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders page with title', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedModelList = vi.mocked(modelApi.list);
    const mockedStyleList = vi.mocked(styleApi.list);
    const mockedImageList = vi.mocked(imageApi.list);

    mockedProjectList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedModelList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedStyleList.mockResolvedValue([]);
    mockedImageList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });

    renderImages();

    await waitFor(() => {
      expect(screen.getByText('图片库')).toBeInTheDocument();
    });
  });

  it('shows loading state initially', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedModelList = vi.mocked(modelApi.list);
    const mockedStyleList = vi.mocked(styleApi.list);
    const mockedImageList = vi.mocked(imageApi.list);

    mockedProjectList.mockReturnValue(new Promise(() => {}));
    mockedModelList.mockReturnValue(new Promise(() => {}));
    mockedStyleList.mockReturnValue(new Promise(() => {}));
    mockedImageList.mockReturnValue(new Promise(() => {}));

    renderImages();

    // The inner Card has loading={loading}; antd Card loading adds a spinning wrapper
    await waitFor(() => {
      expect(document.querySelector('.ant-card-loading')).toBeInTheDocument();
    });
  });

  it('displays empty state when no images', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedModelList = vi.mocked(modelApi.list);
    const mockedStyleList = vi.mocked(styleApi.list);
    const mockedImageList = vi.mocked(imageApi.list);

    mockedProjectList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedModelList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedStyleList.mockResolvedValue([]);
    mockedImageList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });

    renderImages();

    await waitFor(() => {
      expect(screen.getByText('暂无图片')).toBeInTheDocument();
    });

    expect(screen.getAllByRole('button', { name: /生成图片/i }).length).toBeGreaterThanOrEqual(1);
  });

  it('displays image list with cards', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedModelList = vi.mocked(modelApi.list);
    const mockedStyleList = vi.mocked(styleApi.list);
    const mockedImageList = vi.mocked(imageApi.list);

    const mockImages = [
      {
        id: 1,
        project_id: 1,
        storyboard_id: 1,
        src_type: 'generated',
        url: 'http://example.com/img1.jpg',
        thumb_url: 'http://example.com/img1_thumb.jpg',
        width: 1024,
        height: 1024,
        prompt: 'A beautiful sunset',
        neg_prompt: 'blurry',
        model_id: 1,
        params: {},
        status: 2,
        created_by: 1,
        created_at: '2024-01-01T00:00:00Z',
      },
      {
        id: 2,
        project_id: 1,
        storyboard_id: 2,
        src_type: 'generated',
        url: '',
        thumb_url: '',
        width: 0,
        height: 0,
        prompt: '',
        neg_prompt: '',
        model_id: 1,
        params: {},
        status: 0,
        created_by: 1,
        created_at: '2024-01-02T00:00:00Z',
      },
    ];

    mockedProjectList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedModelList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedStyleList.mockResolvedValue([]);
    mockedImageList.mockResolvedValue({ total: 2, list: mockImages as any, page: 1, page_size: 20 });

    renderImages();

    await waitFor(() => {
      expect(screen.getByText('A beautiful sunset')).toBeInTheDocument();
    });

    expect(screen.getByText('#1')).toBeInTheDocument();
    expect(screen.getByText('#2')).toBeInTheDocument();
    expect(screen.getByText('已完成')).toBeInTheDocument();
    expect(screen.getByText('草稿')).toBeInTheDocument();
    expect(screen.getByText('2 张')).toBeInTheDocument();
  });

  it('filters images by project', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedModelList = vi.mocked(modelApi.list);
    const mockedStyleList = vi.mocked(styleApi.list);
    const mockedImageList = vi.mocked(imageApi.list);

    const mockProjects = [
      { id: 1, name: 'Project A' },
      { id: 2, name: 'Project B' },
    ];

    mockedProjectList.mockResolvedValue({ total: 2, list: mockProjects as any, page: 1, page_size: 200 });
    mockedModelList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedStyleList.mockResolvedValue([]);
    mockedImageList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });

    renderImages();

    await waitFor(() => {
      expect(screen.getByText('图片库')).toBeInTheDocument();
    });

    // Open project select
    const select = screen.getAllByRole('combobox')[0];
    await userEvent.click(select);

    await waitFor(() => {
      expect(screen.getByText('Project A')).toBeInTheDocument();
    });

    await userEvent.click(screen.getByText('Project A'));

    await waitFor(() => {
      expect(mockedImageList).toHaveBeenCalledWith(
        expect.objectContaining({
          project_id: 1,
        }),
      );
    });
  });

  it('filters images by storyboard ID', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedModelList = vi.mocked(modelApi.list);
    const mockedStyleList = vi.mocked(styleApi.list);
    const mockedImageList = vi.mocked(imageApi.list);

    mockedProjectList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedModelList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedStyleList.mockResolvedValue([]);
    mockedImageList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });

    renderImages();

    await waitFor(() => {
      expect(screen.getByText('图片库')).toBeInTheDocument();
    });

    // Find storyboard ID input
    const input = screen.getByPlaceholderText('分镜 ID');
    await userEvent.type(input, '5');

    await waitFor(() => {
      expect(mockedImageList).toHaveBeenCalledWith(
        expect.objectContaining({
          storyboard_id: 5,
        }),
      );
    });
  });

  it('opens generate modal and validates required fields', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedModelList = vi.mocked(modelApi.list);
    const mockedStyleList = vi.mocked(styleApi.list);
    const mockedImageList = vi.mocked(imageApi.list);
    const mockedImageGenerate = vi.mocked(imageApi.generate);

    const mockModels = [
      { id: 1, name: 'SDXL', code: 'sdxl', type: 'image', provider: 'stability', endpoint: '', default_params: {}, capability_tags: [], enabled: 1, priority: 1, max_qps: 10, health_check_url: '', last_health_status: 1 },
    ];

    mockedProjectList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedModelList.mockResolvedValue({ total: 1, list: mockModels as any, page: 1, page_size: 200 });
    mockedStyleList.mockResolvedValue([]);
    mockedImageList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });
    mockedImageGenerate.mockResolvedValue({ task_id: 'task-123', topic: 'topic-123' });

    renderImages();

    await waitFor(() => {
      expect(screen.getByText('图片库')).toBeInTheDocument();
    });

    await userEvent.click(screen.getAllByRole('button', { name: /生成图片/i })[0]);

    await waitFor(() => {
      expect(screen.getByRole('dialog')).toBeInTheDocument();
    });

    // Fill prompt only; model_id is still empty
    await userEvent.type(screen.getByPlaceholderText('描述画面、人物、风格...'), 'A beautiful sunset');

    // Try to submit without filling required fields
    const dialog = screen.getByRole('dialog');
    const startBtn = dialog.querySelector('.ant-modal-footer button.ant-btn-primary') as HTMLElement;
    await userEvent.click(startBtn);

    // API should not be called due to validation failure
    await waitFor(() => {
      expect(mockedImageGenerate).not.toHaveBeenCalled();
    });
  });

  it('deletes an image after confirmation', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedModelList = vi.mocked(modelApi.list);
    const mockedStyleList = vi.mocked(styleApi.list);
    const mockedImageList = vi.mocked(imageApi.list);
    const mockedImageDelete = vi.mocked(imageApi.delete);

    const mockImages = [
      {
        id: 1,
        project_id: 1,
        storyboard_id: 1,
        src_type: 'generated',
        url: 'http://example.com/img1.jpg',
        thumb_url: 'http://example.com/img1_thumb.jpg',
        width: 1024,
        height: 1024,
        prompt: 'Test image',
        neg_prompt: '',
        model_id: 1,
        params: {},
        status: 2,
        created_by: 1,
        created_at: '2024-01-01T00:00:00Z',
      },
    ];

    mockedProjectList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedModelList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedStyleList.mockResolvedValue([]);
    mockedImageList.mockResolvedValue({ total: 1, list: mockImages as any, page: 1, page_size: 20 });
    mockedImageDelete.mockResolvedValue(undefined as any);

    renderImages();

    await waitFor(() => {
      expect(screen.getByText('Test image')).toBeInTheDocument();
    });

    // The delete action is wrapped in Popconfirm + Tooltip; click the delete icon span
    await userEvent.click(screen.getByLabelText('delete'));

    await waitFor(() => {
      expect(screen.getByText('确认删除?')).toBeInTheDocument();
    });

    // Click the last button which is the Popconfirm OK
    const allButtons = screen.getAllByRole('button');
    await userEvent.click(allButtons[allButtons.length - 1]);

    await waitFor(() => {
      expect(mockedImageDelete).toHaveBeenCalledWith(1);
    });
  });

  it('handles API errors gracefully', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedModelList = vi.mocked(modelApi.list);
    const mockedStyleList = vi.mocked(styleApi.list);
    const mockedImageList = vi.mocked(imageApi.list);

    mockedProjectList.mockRejectedValue(new Error('Network error'));
    mockedModelList.mockRejectedValue(new Error('Network error'));
    mockedStyleList.mockRejectedValue(new Error('Network error'));
    mockedImageList.mockRejectedValue(new Error('Network error'));

    renderImages();

    await waitFor(() => {
      expect(screen.getByText('图片库')).toBeInTheDocument();
    });

    // Should show empty state even when API fails
    await waitFor(() => {
      expect(screen.getByText('暂无图片')).toBeInTheDocument();
    });
  });

  it('displays pagination when there are images', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedModelList = vi.mocked(modelApi.list);
    const mockedStyleList = vi.mocked(styleApi.list);
    const mockedImageList = vi.mocked(imageApi.list);

    const mockImages = Array.from({ length: 25 }, (_, i) => ({
      id: i + 1,
      project_id: 1,
      storyboard_id: 1,
      src_type: 'generated',
      url: `http://example.com/img${i + 1}.jpg`,
      thumb_url: `http://example.com/img${i + 1}_thumb.jpg`,
      width: 1024,
      height: 1024,
      prompt: `Image ${i + 1}`,
      neg_prompt: '',
      model_id: 1,
      params: {},
      status: 2,
      created_by: 1,
      created_at: '2024-01-01T00:00:00Z',
    }));

    mockedProjectList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedModelList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedStyleList.mockResolvedValue([]);
    mockedImageList.mockResolvedValue({ total: 25, list: mockImages as any, page: 1, page_size: 20 });

    renderImages();

    await waitFor(() => {
      expect(screen.getByText('Image 1')).toBeInTheDocument();
    });

    // Pagination should be visible (antd pagination has specific structure)
    const pagination = document.querySelector('.ant-pagination');
    expect(pagination).toBeInTheDocument();
  });

  it('calls APIs with correct parameters on mount', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedModelList = vi.mocked(modelApi.list);
    const mockedStyleList = vi.mocked(styleApi.list);
    const mockedImageList = vi.mocked(imageApi.list);

    mockedProjectList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedModelList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedStyleList.mockResolvedValue([]);
    mockedImageList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });

    renderImages();

    await waitFor(() => {
      expect(mockedProjectList).toHaveBeenCalledWith({ page_size: 200 });
    });

    expect(mockedModelList).toHaveBeenCalledWith({ page_size: 200, type: 'image', enabled: 1 });
    expect(mockedStyleList).toHaveBeenCalledWith(undefined);
    expect(mockedImageList).toHaveBeenCalledWith({
      page: 1,
      page_size: 20,
      project_id: undefined,
      storyboard_id: undefined,
    });
  });
});
