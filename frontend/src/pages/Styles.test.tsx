import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import StylesPage from './Styles';
import { projectApi, styleApi } from '@/api/modules';

// Mock API modules
vi.mock('@/api/modules', async () => {
  const actual = await vi.importActual<typeof import('@/api/modules')>('@/api/modules');
  return {
    ...actual,
    projectApi: {
      list: vi.fn(),
    },
    styleApi: {
      list: vi.fn(),
      create: vi.fn(),
      update: vi.fn(),
      delete: vi.fn(),
    },
    uploadApi: {
      upload: vi.fn(),
    },
  };
});

function renderStyles() {
  return render(<StylesPage />);
}

describe('StylesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders page with title', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedStyleList = vi.mocked(styleApi.list);

    mockedProjectList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedStyleList.mockResolvedValue([]);

    renderStyles();

    await waitFor(() => {
      expect(screen.getByText('风格预设管理')).toBeInTheDocument();
    });
  });

  it('shows loading state initially', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedStyleList = vi.mocked(styleApi.list);

    mockedProjectList.mockReturnValue(new Promise(() => {}));
    mockedStyleList.mockReturnValue(new Promise(() => {}));

    renderStyles();

    await waitFor(() => {
      expect(document.querySelector('[aria-busy="true"]')).toBeInTheDocument();
    });
  });

  it('displays empty state when no styles', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedStyleList = vi.mocked(styleApi.list);

    mockedProjectList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedStyleList.mockResolvedValue([]);

    renderStyles();

    await waitFor(() => {
      expect(screen.getByText('还没有风格预设')).toBeInTheDocument();
    });

    expect(screen.getAllByRole('button', { name: /新建风格/i }).length).toBeGreaterThanOrEqual(1);
  });

  it('displays style list in table', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedStyleList = vi.mocked(styleApi.list);

    const mockProjects = [
      { id: 1, name: 'Project A' },
    ];

    const mockStyles = [
      {
        id: 1,
        project_id: 1,
        name: '都市霓虹',
        art_style: '写实',
        color_tone: '冷色',
        lighting: '逆光',
        reference_images: null,
        lora_id: '',
        description: '',
        status: 1,
        created_by: 1,
        created_at: '2024-01-01T00:00:00Z',
        updated_at: '2024-01-01T00:00:00Z',
      },
      {
        id: 2,
        project_id: 0,
        name: '复古港片',
        art_style: '胶片',
        color_tone: '暖色',
        lighting: '侧光',
        reference_images: ['http://example.com/ref1.jpg'],
        lora_id: 'lora-1',
        description: '80年代香港风格',
        status: 0,
        created_by: 1,
        created_at: '2024-01-02T00:00:00Z',
        updated_at: '2024-01-02T00:00:00Z',
      },
    ];

    mockedProjectList.mockResolvedValue({ total: 1, list: mockProjects as any, page: 1, page_size: 200 });
    mockedStyleList.mockResolvedValue(mockStyles as any);

    renderStyles();

    await waitFor(() => {
      expect(screen.getByText('都市霓虹')).toBeInTheDocument();
    });

    expect(screen.getByText('复古港片')).toBeInTheDocument();
    expect(screen.getByText('Project A')).toBeInTheDocument();
    expect(screen.getByText('写实')).toBeInTheDocument();
    expect(screen.getByText('胶片')).toBeInTheDocument();
    expect(screen.getByText('冷色')).toBeInTheDocument();
    expect(screen.getByText('暖色')).toBeInTheDocument();
    expect(screen.getByText('逆光')).toBeInTheDocument();
    expect(screen.getByText('侧光')).toBeInTheDocument();
    expect(screen.getByText('lora-1')).toBeInTheDocument();
    expect(screen.getByText('启用')).toBeInTheDocument();
    expect(screen.getByText('禁用')).toBeInTheDocument();
  });

  it('filters styles by project', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedStyleList = vi.mocked(styleApi.list);

    const mockProjects = [
      { id: 1, name: 'Project A' },
      { id: 2, name: 'Project B' },
    ];

    mockedProjectList.mockResolvedValue({ total: 2, list: mockProjects as any, page: 1, page_size: 200 });
    mockedStyleList.mockResolvedValue([]);

    renderStyles();

    await waitFor(() => {
      expect(screen.getByText('风格预设管理')).toBeInTheDocument();
    });

    // Open project select
    const select = screen.getByRole('combobox');
    await userEvent.click(select);

    await waitFor(() => {
      expect(screen.getByText('Project A')).toBeInTheDocument();
    });

    await userEvent.click(screen.getByText('Project A'));

    await waitFor(() => {
      expect(mockedStyleList).toHaveBeenCalledWith(1);
    });
  });

  it('opens create modal and submits new style', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedStyleList = vi.mocked(styleApi.list);
    const mockedStyleCreate = vi.mocked(styleApi.create);

    mockedProjectList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedStyleList.mockResolvedValue([]);
    mockedStyleCreate.mockResolvedValue({ id: 3 } as any);

    renderStyles();

    await waitFor(() => {
      expect(screen.getByText('风格预设管理')).toBeInTheDocument();
    });

    await userEvent.click(screen.getAllByRole('button', { name: /新建风格/i })[0]);

    await waitFor(() => {
      expect(screen.getByRole('dialog')).toBeInTheDocument();
    });

    // Fill form
    await userEvent.type(screen.getByPlaceholderText('如:都市霓虹、复古港片...'), '测试风格');
    await userEvent.type(screen.getByPlaceholderText('如:写实/动漫/水彩'), '写实');
    await userEvent.type(screen.getByPlaceholderText('如:冷色/暖色/高饱和'), '冷色');
    await userEvent.type(screen.getByPlaceholderText('如:逆光/侧光/柔光'), '柔光');

    // Click the OK button in the modal footer by querying all buttons and taking the last one
    const allButtons = screen.getAllByRole('button');
    await userEvent.click(allButtons[allButtons.length - 1]);

    await waitFor(() => {
      expect(mockedStyleCreate).toHaveBeenCalledWith(
        expect.objectContaining({
          name: '测试风格',
          art_style: '写实',
          color_tone: '冷色',
          lighting: '柔光',
          reference_images: [],
        }),
      );
    });
  });

  it('opens edit modal with pre-filled values', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedStyleList = vi.mocked(styleApi.list);
    const mockedStyleUpdate = vi.mocked(styleApi.update);

    const mockProjects = [{ id: 1, name: 'Project A' }];
    const mockStyles = [
      {
        id: 1,
        project_id: 1,
        name: '都市霓虹',
        art_style: '写实',
        color_tone: '冷色',
        lighting: '逆光',
        reference_images: null,
        lora_id: 'lora-1',
        description: '描述文本',
        status: 1,
        created_by: 1,
        created_at: '2024-01-01T00:00:00Z',
        updated_at: '2024-01-01T00:00:00Z',
      },
    ];

    mockedProjectList.mockResolvedValue({ total: 1, list: mockProjects as any, page: 1, page_size: 200 });
    mockedStyleList.mockResolvedValue(mockStyles as any);
    mockedStyleUpdate.mockResolvedValue({ id: 1 } as any);

    renderStyles();

    await waitFor(() => {
      expect(screen.getByText('都市霓虹')).toBeInTheDocument();
    });

    await userEvent.click(screen.getAllByRole('button', { name: /编辑/i })[0]);

    await waitFor(() => {
      expect(screen.getByRole('dialog')).toBeInTheDocument();
    });

    // Check pre-filled values
    expect(screen.getByDisplayValue('都市霓虹')).toBeInTheDocument();
    expect(screen.getByDisplayValue('写实')).toBeInTheDocument();
    expect(screen.getByDisplayValue('冷色')).toBeInTheDocument();
    expect(screen.getByDisplayValue('逆光')).toBeInTheDocument();
    expect(screen.getByDisplayValue('lora-1')).toBeInTheDocument();
    expect(screen.getByDisplayValue('描述文本')).toBeInTheDocument();

    // Update name
    const nameInput = screen.getByDisplayValue('都市霓虹');
    await userEvent.clear(nameInput);
    await userEvent.type(nameInput, '更新后的名称');

    // Click the OK button in the modal footer by querying all buttons and taking the last one
    const allButtons2 = screen.getAllByRole('button');
    await userEvent.click(allButtons2[allButtons2.length - 1]);

    await waitFor(() => {
      expect(mockedStyleUpdate).toHaveBeenCalledWith(
        1,
        expect.objectContaining({
          name: '更新后的名称',
        }),
      );
    });
  });

  it('deletes a style after confirmation', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedStyleList = vi.mocked(styleApi.list);
    const mockedStyleDelete = vi.mocked(styleApi.delete);

    const mockStyles = [
      {
        id: 1,
        project_id: 0,
        name: '待删除风格',
        art_style: '',
        color_tone: '',
        lighting: '',
        reference_images: null,
        lora_id: '',
        description: '',
        status: 1,
        created_by: 1,
        created_at: '2024-01-01T00:00:00Z',
        updated_at: '2024-01-01T00:00:00Z',
      },
    ];

    mockedProjectList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedStyleList.mockResolvedValue(mockStyles as any);
    mockedStyleDelete.mockResolvedValue(undefined as any);

    renderStyles();

    await waitFor(() => {
      expect(screen.getByText('待删除风格')).toBeInTheDocument();
    });

    await userEvent.click(screen.getAllByRole('button', { name: /删除/i })[0]);

    await waitFor(() => {
      expect(screen.getByText('确认删除该风格?')).toBeInTheDocument();
    });

    // Click the OK button in the Popconfirm / modal by querying all buttons and taking the last one
    const allButtons3 = screen.getAllByRole('button');
    await userEvent.click(allButtons3[allButtons3.length - 1]);

    await waitFor(() => {
      expect(mockedStyleDelete).toHaveBeenCalledWith(1);
    });
  });

  it('handles API errors gracefully', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedStyleList = vi.mocked(styleApi.list);

    mockedProjectList.mockRejectedValue(new Error('Network error'));
    mockedStyleList.mockRejectedValue(new Error('Network error'));

    renderStyles();

    await waitFor(() => {
      expect(screen.getByText('风格预设管理')).toBeInTheDocument();
    });

    // Should show empty state even when API fails
    await waitFor(() => {
      expect(screen.getByText('还没有风格预设')).toBeInTheDocument();
    });
  });

  it('displays reference images with +N tag when more than 3', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedStyleList = vi.mocked(styleApi.list);

    const mockStyles = [
      {
        id: 1,
        project_id: 0,
        name: '多图风格',
        art_style: '',
        color_tone: '',
        lighting: '',
        reference_images: ['http://example.com/1.jpg', 'http://example.com/2.jpg', 'http://example.com/3.jpg', 'http://example.com/4.jpg', 'http://example.com/5.jpg'],
        lora_id: '',
        description: '',
        status: 1,
        created_by: 1,
        created_at: '2024-01-01T00:00:00Z',
        updated_at: '2024-01-01T00:00:00Z',
      },
    ];

    mockedProjectList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedStyleList.mockResolvedValue(mockStyles as any);

    renderStyles();

    await waitFor(() => {
      expect(screen.getByText('多图风格')).toBeInTheDocument();
    });

    expect(screen.getByText('+2')).toBeInTheDocument();
  });

  it('displays dash for missing project', async () => {
    const mockedProjectList = vi.mocked(projectApi.list);
    const mockedStyleList = vi.mocked(styleApi.list);

    const mockStyles = [
      {
        id: 1,
        project_id: 0,
        name: '全局风格',
        art_style: '',
        color_tone: '',
        lighting: '',
        reference_images: null,
        lora_id: '',
        description: '',
        status: 1,
        created_by: 1,
        created_at: '2024-01-01T00:00:00Z',
        updated_at: '2024-01-01T00:00:00Z',
      },
    ];

    mockedProjectList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 200 });
    mockedStyleList.mockResolvedValue(mockStyles as any);

    renderStyles();

    await waitFor(() => {
      expect(screen.getByText('全局风格')).toBeInTheDocument();
    });

    // The project cell should show '-' for no project
    const row = screen.getByText('全局风格').closest('tr');
    expect(row?.textContent).toContain('-');
  });
});
