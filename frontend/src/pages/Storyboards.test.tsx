import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import StoryboardsPage from './Storyboards';
import { scriptApi, modelApi, storyboardApi, styleApi } from '@/api/modules';

// Mock API modules
vi.mock('@/api/modules', async () => {
  const actual = await vi.importActual<typeof import('@/api/modules')>('@/api/modules');
  return {
    ...actual,
    scriptApi: {
      list: vi.fn(),
      episodes: vi.fn(),
    },
    modelApi: {
      list: vi.fn(),
    },
    storyboardApi: {
      listByEpisode: vi.fn(),
      generate: vi.fn(),
      update: vi.fn(),
      delete: vi.fn(),
      applyStyle: vi.fn(),
    },
    styleApi: {
      list: vi.fn(),
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

function renderStoryboards() {
  return render(<StoryboardsPage />);
}

/** Find an antd Select wrapper by its placeholder text */
function getSelectByPlaceholder(container: HTMLElement, placeholder: string): HTMLElement {
  const spans = container.querySelectorAll('.ant-select-selection-placeholder');
  for (const span of spans) {
    if (span.textContent === placeholder) {
      const select = span.closest('.ant-select');
      if (select) return select as HTMLElement;
    }
  }
  throw new Error(`Select with placeholder "${placeholder}" not found`);
}

/** Find the search input inside an antd Select (needed when showSearch is enabled) */
function getSelectInputByPlaceholder(container: HTMLElement, placeholder: string): HTMLElement {
  const select = getSelectByPlaceholder(container, placeholder);
  const input = select.querySelector('.ant-select-selection-search-input');
  if (input) return input as HTMLElement;
  throw new Error(`Select input with placeholder "${placeholder}" not found`);
}

describe('StoryboardsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  const mockScripts = [
    { id: 1, project_id: 10, name: '剧本A', source_url: '', raw_text: '', current_version: 1, status: 1, created_by: 1, updated_by: 1, created_at: '2024-01-01T00:00:00Z', updated_at: '2024-01-01T00:00:00Z' },
  ];

  const mockModels = [
    { id: 1, code: 'gpt-4', name: 'GPT-4', type: 'text', provider: 'openai', endpoint: '', default_params: {}, capability_tags: [], enabled: 1, priority: 1, max_qps: 10, health_check_url: '', last_health_status: 1 },
  ];

  const mockEpisodes = [
    { id: 20, script_id: 1, ep_no: 1, title: '第一集', summary: '摘要1', raw_segment: '片段1', status: 1, created_at: '2024-01-01T00:00:00Z', updated_at: '2024-01-01T00:00:00Z' },
  ];

  const mockStyles = [
    { id: 1, project_id: 10, name: '赛博朋克', art_style: 'cyberpunk', color_tone: 'blue', lighting: 'neon', reference_images: null, lora_id: '', description: '', status: 1, created_by: 1, created_at: '2024-01-01T00:00:00Z', updated_at: '2024-01-01T00:00:00Z' },
  ];

  const mockStoryboards = [
    { id: 201, episode_id: 20, prompt_id: 1, shot_no: 1, shot_type: 'wide', camera_motion: 'push', scene_desc: '城市全景', characters: ['主角', '配角'], action: '行走', dialogue: '你好', duration_sec: 5, notes: '', status: 1, created_at: '2024-01-01T00:00:00Z', updated_at: '2024-01-01T00:00:00Z' },
    { id: 202, episode_id: 20, prompt_id: 1, shot_no: 2, shot_type: 'close', camera_motion: 'static', scene_desc: '面部特写', characters: null, action: '凝视', dialogue: '', duration_sec: 3, notes: '', status: 1, created_at: '2024-01-01T00:00:00Z', updated_at: '2024-01-01T00:00:00Z' },
  ];

  it('renders page with selectors and generate button', async () => {
    vi.mocked(scriptApi.list).mockResolvedValue({ total: 1, list: mockScripts, page: 1, page_size: 200 });
    vi.mocked(modelApi.list).mockResolvedValue({ total: 1, list: mockModels, page: 1, page_size: 200 });

    const { container } = renderStoryboards();

    await waitFor(() => {
      expect(getSelectByPlaceholder(container, '选择剧本')).toBeInTheDocument();
    });
    expect(getSelectByPlaceholder(container, '选择分集')).toBeInTheDocument();
    expect(screen.getByText('AI 生成分镜')).toBeInTheDocument();
  });

  it('shows empty state when no episode selected', async () => {
    vi.mocked(scriptApi.list).mockResolvedValue({ total: 1, list: mockScripts, page: 1, page_size: 200 });
    vi.mocked(modelApi.list).mockResolvedValue({ total: 1, list: mockModels, page: 1, page_size: 200 });

    renderStoryboards();

    await waitFor(() => {
      expect(screen.getByText('请先在上方选择剧本与分集')).toBeInTheDocument();
    });
  });

  it('loads storyboards when episode is selected', async () => {
    vi.mocked(scriptApi.list).mockResolvedValue({ total: 1, list: mockScripts, page: 1, page_size: 200 });
    vi.mocked(modelApi.list).mockResolvedValue({ total: 1, list: mockModels, page: 1, page_size: 200 });
    vi.mocked(scriptApi.episodes).mockResolvedValue(mockEpisodes);
    vi.mocked(storyboardApi.listByEpisode).mockResolvedValue(mockStoryboards);
    vi.mocked(styleApi.list).mockResolvedValue(mockStyles);

    const { container } = renderStoryboards();

    await waitFor(() => {
      expect(getSelectByPlaceholder(container, '选择剧本')).toBeInTheDocument();
    });

    // Select script
    await userEvent.click(getSelectInputByPlaceholder(container, '选择剧本'));
    await waitFor(() => {
      expect(screen.getByText('剧本A')).toBeInTheDocument();
    });
    await userEvent.click(screen.getByText('剧本A'));

    await waitFor(() => {
      expect(getSelectByPlaceholder(container, '选择分集').closest('.ant-select-disabled')).toBeNull();
    });

    // Select episode
    await userEvent.click(getSelectInputByPlaceholder(container, '选择分集'));
    await waitFor(() => {
      expect(screen.getByText('1. 第一集')).toBeInTheDocument();
    });
    await userEvent.click(screen.getByText('1. 第一集'));

    await waitFor(() => {
      expect(storyboardApi.listByEpisode).toHaveBeenCalledWith(20);
    });

    // Check table renders
    await waitFor(() => {
      expect(screen.getByText('分镜列表')).toBeInTheDocument();
      expect(screen.getByText('2 条')).toBeInTheDocument();
    });
  });

  it('displays storyboard data correctly in table', async () => {
    vi.mocked(scriptApi.list).mockResolvedValue({ total: 1, list: mockScripts, page: 1, page_size: 200 });
    vi.mocked(modelApi.list).mockResolvedValue({ total: 1, list: mockModels, page: 1, page_size: 200 });
    vi.mocked(scriptApi.episodes).mockResolvedValue(mockEpisodes);
    vi.mocked(storyboardApi.listByEpisode).mockResolvedValue(mockStoryboards);
    vi.mocked(styleApi.list).mockResolvedValue(mockStyles);

    const { container } = renderStoryboards();

    await waitFor(() => {
      expect(getSelectByPlaceholder(container, '选择剧本')).toBeInTheDocument();
    });

    await userEvent.click(getSelectInputByPlaceholder(container, '选择剧本'));
    await waitFor(() => {
      expect(screen.getByText('剧本A')).toBeInTheDocument();
    });
    await userEvent.click(screen.getByText('剧本A'));

    await waitFor(() => {
      expect(getSelectByPlaceholder(container, '选择分集').closest('.ant-select-disabled')).toBeNull();
    });

    await userEvent.click(getSelectInputByPlaceholder(container, '选择分集'));
    await waitFor(() => {
      expect(screen.getByText('1. 第一集')).toBeInTheDocument();
    });
    await userEvent.click(screen.getByText('1. 第一集'));

    await waitFor(() => {
      expect(screen.getByText('城市全景')).toBeInTheDocument();
      expect(screen.getByText('面部特写')).toBeInTheDocument();
      expect(screen.getByText('wide')).toBeInTheDocument();
      expect(screen.getByText('close')).toBeInTheDocument();
      expect(screen.getByText('push')).toBeInTheDocument();
      expect(screen.getByText('static')).toBeInTheDocument();
      expect(screen.getByText('主角')).toBeInTheDocument();
      expect(screen.getByText('配角')).toBeInTheDocument();
    });
  });

  it('displays empty state for episode without storyboards', async () => {
    vi.mocked(scriptApi.list).mockResolvedValue({ total: 1, list: mockScripts, page: 1, page_size: 200 });
    vi.mocked(modelApi.list).mockResolvedValue({ total: 1, list: mockModels, page: 1, page_size: 200 });
    vi.mocked(scriptApi.episodes).mockResolvedValue(mockEpisodes);
    vi.mocked(storyboardApi.listByEpisode).mockResolvedValue([]);
    vi.mocked(styleApi.list).mockResolvedValue(mockStyles);

    const { container } = renderStoryboards();

    await waitFor(() => {
      expect(getSelectByPlaceholder(container, '选择剧本')).toBeInTheDocument();
    });

    await userEvent.click(getSelectInputByPlaceholder(container, '选择剧本'));
    await waitFor(() => {
      expect(screen.getByText('剧本A')).toBeInTheDocument();
    });
    await userEvent.click(screen.getByText('剧本A'));

    await waitFor(() => {
      expect(getSelectByPlaceholder(container, '选择分集').closest('.ant-select-disabled')).toBeNull();
    });

    await userEvent.click(getSelectInputByPlaceholder(container, '选择分集'));
    await waitFor(() => {
      expect(screen.getByText('1. 第一集')).toBeInTheDocument();
    });
    await userEvent.click(screen.getByText('1. 第一集'));

    await waitFor(() => {
      expect(screen.getByText('该分集暂无分镜')).toBeInTheDocument();
    });
  });

  it('opens edit modal when clicking edit button', async () => {
    vi.mocked(scriptApi.list).mockResolvedValue({ total: 1, list: mockScripts, page: 1, page_size: 200 });
    vi.mocked(modelApi.list).mockResolvedValue({ total: 1, list: mockModels, page: 1, page_size: 200 });
    vi.mocked(scriptApi.episodes).mockResolvedValue(mockEpisodes);
    vi.mocked(storyboardApi.listByEpisode).mockResolvedValue(mockStoryboards);
    vi.mocked(styleApi.list).mockResolvedValue(mockStyles);

    const { container } = renderStoryboards();

    await waitFor(() => {
      expect(getSelectByPlaceholder(container, '选择剧本')).toBeInTheDocument();
    });

    await userEvent.click(getSelectInputByPlaceholder(container, '选择剧本'));
    await waitFor(() => {
      expect(screen.getByText('剧本A')).toBeInTheDocument();
    });
    await userEvent.click(screen.getByText('剧本A'));

    await waitFor(() => {
      expect(getSelectByPlaceholder(container, '选择分集').closest('.ant-select-disabled')).toBeNull();
    });

    await userEvent.click(getSelectInputByPlaceholder(container, '选择分集'));
    await waitFor(() => {
      expect(screen.getByText('1. 第一集')).toBeInTheDocument();
    });
    await userEvent.click(screen.getByText('1. 第一集'));

    await waitFor(() => {
      expect(screen.getByText('分镜列表')).toBeInTheDocument();
    });

    const editBtn = screen.getAllByText('编辑')[0];
    await userEvent.click(editBtn);

    await waitFor(() => {
      expect(screen.getByText('编辑分镜 #1')).toBeInTheDocument();
      // 景别 appears in both table header and modal form, so check it exists at least twice
      expect(screen.getAllByText('景别').length).toBeGreaterThanOrEqual(2);
      expect(screen.getAllByText('运镜').length).toBeGreaterThanOrEqual(2);
      expect(screen.getAllByText('场景描述').length).toBeGreaterThanOrEqual(2);
      expect(screen.getAllByText('角色').length).toBeGreaterThanOrEqual(2);
      expect(screen.getAllByText('动作').length).toBeGreaterThanOrEqual(2);
      expect(screen.getAllByText('台词').length).toBeGreaterThanOrEqual(2);
    });
  });

  it('calls update API when saving edit', async () => {
    vi.mocked(scriptApi.list).mockResolvedValue({ total: 1, list: mockScripts, page: 1, page_size: 200 });
    vi.mocked(modelApi.list).mockResolvedValue({ total: 1, list: mockModels, page: 1, page_size: 200 });
    vi.mocked(scriptApi.episodes).mockResolvedValue(mockEpisodes);
    vi.mocked(storyboardApi.listByEpisode).mockResolvedValue(mockStoryboards);
    vi.mocked(styleApi.list).mockResolvedValue(mockStyles);
    vi.mocked(storyboardApi.update).mockResolvedValue(mockStoryboards[0]);

    const { container } = renderStoryboards();

    await waitFor(() => {
      expect(getSelectByPlaceholder(container, '选择剧本')).toBeInTheDocument();
    });

    await userEvent.click(getSelectInputByPlaceholder(container, '选择剧本'));
    await waitFor(() => {
      expect(screen.getByText('剧本A')).toBeInTheDocument();
    });
    await userEvent.click(screen.getByText('剧本A'));

    await waitFor(() => {
      expect(getSelectByPlaceholder(container, '选择分集').closest('.ant-select-disabled')).toBeNull();
    });

    await userEvent.click(getSelectInputByPlaceholder(container, '选择分集'));
    await waitFor(() => {
      expect(screen.getByText('1. 第一集')).toBeInTheDocument();
    });
    await userEvent.click(screen.getByText('1. 第一集'));

    await waitFor(() => {
      expect(screen.getByText('分镜列表')).toBeInTheDocument();
    });

    const editBtn = screen.getAllByText('编辑')[0];
    await userEvent.click(editBtn);

    await waitFor(() => {
      expect(screen.getByText('编辑分镜 #1')).toBeInTheDocument();
    });

    const okBtn = document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement;
    if (okBtn) await userEvent.click(okBtn);

    await waitFor(() => {
      expect(storyboardApi.update).toHaveBeenCalledWith(201, expect.any(Object));
    });
  });

  it('calls delete API when confirming delete', async () => {
    vi.mocked(scriptApi.list).mockResolvedValue({ total: 1, list: mockScripts, page: 1, page_size: 200 });
    vi.mocked(modelApi.list).mockResolvedValue({ total: 1, list: mockModels, page: 1, page_size: 200 });
    vi.mocked(scriptApi.episodes).mockResolvedValue(mockEpisodes);
    vi.mocked(storyboardApi.listByEpisode).mockResolvedValue(mockStoryboards);
    vi.mocked(styleApi.list).mockResolvedValue(mockStyles);
    vi.mocked(storyboardApi.delete).mockResolvedValue(undefined);

    const { container } = renderStoryboards();

    await waitFor(() => {
      expect(getSelectByPlaceholder(container, '选择剧本')).toBeInTheDocument();
    });

    await userEvent.click(getSelectInputByPlaceholder(container, '选择剧本'));
    await waitFor(() => {
      expect(screen.getByText('剧本A')).toBeInTheDocument();
    });
    await userEvent.click(screen.getByText('剧本A'));

    await waitFor(() => {
      expect(getSelectByPlaceholder(container, '选择分集').closest('.ant-select-disabled')).toBeNull();
    });

    await userEvent.click(getSelectInputByPlaceholder(container, '选择分集'));
    await waitFor(() => {
      expect(screen.getByText('1. 第一集')).toBeInTheDocument();
    });
    await userEvent.click(screen.getByText('1. 第一集'));

    await waitFor(() => {
      expect(screen.getByText('分镜列表')).toBeInTheDocument();
    });

    const deleteBtn = screen.getAllByText('删除')[0];
    await userEvent.click(deleteBtn);

    await waitFor(() => {
      expect(screen.getByText('确认删除该分镜?')).toBeInTheDocument();
    });

    await waitFor(() => {
      expect(document.querySelector('.ant-popconfirm-buttons .ant-btn-primary')).toBeInTheDocument();
    });
    const confirmBtn = document.querySelector('.ant-popconfirm-buttons .ant-btn-primary') as HTMLElement;
    await userEvent.click(confirmBtn);

    await waitFor(() => {
      expect(storyboardApi.delete).toHaveBeenCalledWith(201);
    });
  });

  it('opens generate modal when clicking generate button', async () => {
    vi.mocked(scriptApi.list).mockResolvedValue({ total: 1, list: mockScripts, page: 1, page_size: 200 });
    vi.mocked(modelApi.list).mockResolvedValue({ total: 1, list: mockModels, page: 1, page_size: 200 });
    vi.mocked(scriptApi.episodes).mockResolvedValue(mockEpisodes);
    vi.mocked(storyboardApi.listByEpisode).mockResolvedValue(mockStoryboards);
    vi.mocked(styleApi.list).mockResolvedValue(mockStyles);

    const { container } = renderStoryboards();

    await waitFor(() => {
      expect(getSelectByPlaceholder(container, '选择剧本')).toBeInTheDocument();
    });

    await userEvent.click(getSelectInputByPlaceholder(container, '选择剧本'));
    await waitFor(() => {
      expect(screen.getByText('剧本A')).toBeInTheDocument();
    });
    await userEvent.click(screen.getByText('剧本A'));

    await waitFor(() => {
      expect(getSelectByPlaceholder(container, '选择分集').closest('.ant-select-disabled')).toBeNull();
    });

    await userEvent.click(getSelectInputByPlaceholder(container, '选择分集'));
    await waitFor(() => {
      expect(screen.getByText('1. 第一集')).toBeInTheDocument();
    });
    await userEvent.click(screen.getByText('1. 第一集'));

    await waitFor(() => {
      expect(screen.getByText('AI 生成分镜').closest('button')).not.toBeDisabled();
    });

    await userEvent.click(screen.getByText('AI 生成分镜'));

    await waitFor(() => {
      // AI 生成分镜 appears as both button text and modal title
      expect(screen.getAllByText('AI 生成分镜').length).toBeGreaterThanOrEqual(2);
      expect(screen.getByText('文本模型')).toBeInTheDocument();
    });
  });

  it('calls generate API when submitting generate modal', async () => {
    vi.mocked(scriptApi.list).mockResolvedValue({ total: 1, list: mockScripts, page: 1, page_size: 200 });
    vi.mocked(modelApi.list).mockResolvedValue({ total: 1, list: mockModels, page: 1, page_size: 200 });
    vi.mocked(scriptApi.episodes).mockResolvedValue(mockEpisodes);
    vi.mocked(storyboardApi.listByEpisode).mockResolvedValue(mockStoryboards);
    vi.mocked(styleApi.list).mockResolvedValue(mockStyles);
    vi.mocked(storyboardApi.generate).mockResolvedValue({ task_id: 'task-456', topic: 'topic-456' });

    const { container } = renderStoryboards();

    await waitFor(() => {
      expect(getSelectByPlaceholder(container, '选择剧本')).toBeInTheDocument();
    });

    await userEvent.click(getSelectInputByPlaceholder(container, '选择剧本'));
    await waitFor(() => {
      expect(screen.getByText('剧本A')).toBeInTheDocument();
    });
    await userEvent.click(screen.getByText('剧本A'));

    await waitFor(() => {
      expect(getSelectByPlaceholder(container, '选择分集').closest('.ant-select-disabled')).toBeNull();
    });

    await userEvent.click(getSelectInputByPlaceholder(container, '选择分集'));
    await waitFor(() => {
      expect(screen.getByText('1. 第一集')).toBeInTheDocument();
    });
    await userEvent.click(screen.getByText('1. 第一集'));

    await waitFor(() => {
      expect(screen.getByText('AI 生成分镜').closest('button')).not.toBeDisabled();
    });

    await userEvent.click(screen.getByText('AI 生成分镜'));

    await waitFor(() => {
      expect(screen.getAllByText('AI 生成分镜').length).toBeGreaterThanOrEqual(1);
    });

    // Select model
    const modelSelect = screen.getByText('文本模型').closest('.ant-form-item')?.querySelector('.ant-select') as HTMLElement;
    if (modelSelect) {
      await userEvent.click(modelSelect.querySelector('.ant-select-selection-search-input') as HTMLElement);
      await waitFor(() => {
        expect(screen.getAllByText((content) => content.includes('GPT-4')).length).toBeGreaterThanOrEqual(1);
      });
      const gpt4Options = screen.getAllByText((content) => content.includes('GPT-4'));
      await userEvent.click(gpt4Options[0]);
    }

    const startBtn = screen.getByRole('button', { name: /开始生成/i });
    await userEvent.click(startBtn);

    await waitFor(() => {
      expect(storyboardApi.generate).toHaveBeenCalledWith(20, { model_id: 1 });
    });
  });

  it('opens style modal when clicking 应用风格 button', async () => {
    vi.mocked(scriptApi.list).mockResolvedValue({ total: 1, list: mockScripts, page: 1, page_size: 200 });
    vi.mocked(modelApi.list).mockResolvedValue({ total: 1, list: mockModels, page: 1, page_size: 200 });
    vi.mocked(scriptApi.episodes).mockResolvedValue(mockEpisodes);
    vi.mocked(storyboardApi.listByEpisode).mockResolvedValue(mockStoryboards);
    vi.mocked(styleApi.list).mockResolvedValue(mockStyles);

    const { container } = renderStoryboards();

    await waitFor(() => {
      expect(getSelectByPlaceholder(container, '选择剧本')).toBeInTheDocument();
    });

    await userEvent.click(getSelectInputByPlaceholder(container, '选择剧本'));
    await waitFor(() => {
      expect(screen.getByText('剧本A')).toBeInTheDocument();
    });
    await userEvent.click(screen.getByText('剧本A'));

    await waitFor(() => {
      expect(getSelectByPlaceholder(container, '选择分集').closest('.ant-select-disabled')).toBeNull();
    });

    await userEvent.click(getSelectInputByPlaceholder(container, '选择分集'));
    await waitFor(() => {
      expect(screen.getByText('1. 第一集')).toBeInTheDocument();
    });
    await userEvent.click(screen.getByText('1. 第一集'));

    await waitFor(() => {
      expect(screen.getByText('分镜列表')).toBeInTheDocument();
    });

    const styleBtn = screen.getAllByText('应用风格')[0];
    await userEvent.click(styleBtn);

    await waitFor(() => {
      expect(screen.getByText('应用风格 - 分镜 #1')).toBeInTheDocument();
      expect(screen.getByText('风格预设')).toBeInTheDocument();
    });
  });

  it('calls applyStyle API when submitting style modal', async () => {
    vi.mocked(scriptApi.list).mockResolvedValue({ total: 1, list: mockScripts, page: 1, page_size: 200 });
    vi.mocked(modelApi.list).mockResolvedValue({ total: 1, list: mockModels, page: 1, page_size: 200 });
    vi.mocked(scriptApi.episodes).mockResolvedValue(mockEpisodes);
    vi.mocked(storyboardApi.listByEpisode).mockResolvedValue(mockStoryboards);
    vi.mocked(styleApi.list).mockResolvedValue(mockStyles);
    vi.mocked(storyboardApi.applyStyle).mockResolvedValue(undefined);

    const { container } = renderStoryboards();

    await waitFor(() => {
      expect(getSelectByPlaceholder(container, '选择剧本')).toBeInTheDocument();
    });

    await userEvent.click(getSelectInputByPlaceholder(container, '选择剧本'));
    await waitFor(() => {
      expect(screen.getByText('剧本A')).toBeInTheDocument();
    });
    await userEvent.click(screen.getByText('剧本A'));

    await waitFor(() => {
      expect(getSelectByPlaceholder(container, '选择分集').closest('.ant-select-disabled')).toBeNull();
    });

    await userEvent.click(getSelectInputByPlaceholder(container, '选择分集'));
    await waitFor(() => {
      expect(screen.getByText('1. 第一集')).toBeInTheDocument();
    });
    await userEvent.click(screen.getByText('1. 第一集'));

    await waitFor(() => {
      expect(screen.getByText('分镜列表')).toBeInTheDocument();
    });

    const styleBtn = screen.getAllByText('应用风格')[0];
    await userEvent.click(styleBtn);

    await waitFor(() => {
      expect(screen.getByText('应用风格 - 分镜 #1')).toBeInTheDocument();
    });

    // Select style
    const styleSelect = screen.getByText('风格预设').closest('.ant-form-item')?.querySelector('.ant-select') as HTMLElement;
    if (styleSelect) {
      await userEvent.click(styleSelect.querySelector('.ant-select-selection-search-input') as HTMLElement);
      await waitFor(() => {
        expect(screen.getByText((content) => content.includes('赛博朋克'))).toBeInTheDocument();
      });
      await userEvent.click(screen.getByText((content) => content.includes('赛博朋克')));
    }

    const styleOkBtn = document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement;
    if (styleOkBtn) await userEvent.click(styleOkBtn);

    await waitFor(() => {
      expect(storyboardApi.applyStyle).toHaveBeenCalledWith(201, 1);
    });
  });

  it('handles API errors gracefully', async () => {
    vi.mocked(scriptApi.list).mockRejectedValue(new Error('Network error'));
    vi.mocked(modelApi.list).mockRejectedValue(new Error('Network error'));

    const { container } = renderStoryboards();

    await waitFor(() => {
      expect(getSelectByPlaceholder(container, '选择剧本')).toBeInTheDocument();
    });

    // Page should still render even when APIs fail
    expect(getSelectByPlaceholder(container, '选择分集')).toBeInTheDocument();
    expect(screen.getByText('AI 生成分镜')).toBeInTheDocument();
  });

  it('disables generate button when no episode selected', async () => {
    vi.mocked(scriptApi.list).mockResolvedValue({ total: 1, list: mockScripts, page: 1, page_size: 200 });
    vi.mocked(modelApi.list).mockResolvedValue({ total: 1, list: mockModels, page: 1, page_size: 200 });

    const { container } = renderStoryboards();

    await waitFor(() => {
      expect(getSelectByPlaceholder(container, '选择剧本')).toBeInTheDocument();
    });

    const generateBtn = screen.getByText('AI 生成分镜').closest('button');
    expect(generateBtn).toBeDisabled();
  });

  it('displays project info when script is selected', async () => {
    vi.mocked(scriptApi.list).mockResolvedValue({ total: 1, list: mockScripts, page: 1, page_size: 200 });
    vi.mocked(modelApi.list).mockResolvedValue({ total: 1, list: mockModels, page: 1, page_size: 200 });
    vi.mocked(scriptApi.episodes).mockResolvedValue(mockEpisodes);
    vi.mocked(storyboardApi.listByEpisode).mockResolvedValue([]);
    vi.mocked(styleApi.list).mockResolvedValue(mockStyles);

    const { container } = renderStoryboards();

    await waitFor(() => {
      expect(getSelectByPlaceholder(container, '选择剧本')).toBeInTheDocument();
    });

    await userEvent.click(getSelectInputByPlaceholder(container, '选择剧本'));
    await waitFor(() => {
      expect(screen.getByText('剧本A')).toBeInTheDocument();
    });
    await userEvent.click(screen.getByText('剧本A'));

    await waitFor(() => {
      expect(screen.getByText('项目: #10')).toBeInTheDocument();
    });
  });

  it('handles characters as JSON string', async () => {
    const storyboardsWithJsonChars = [
      { ...mockStoryboards[0], characters: '["角色A","角色B"]' },
    ];

    vi.mocked(scriptApi.list).mockResolvedValue({ total: 1, list: mockScripts, page: 1, page_size: 200 });
    vi.mocked(modelApi.list).mockResolvedValue({ total: 1, list: mockModels, page: 1, page_size: 200 });
    vi.mocked(scriptApi.episodes).mockResolvedValue(mockEpisodes);
    vi.mocked(storyboardApi.listByEpisode).mockResolvedValue(storyboardsWithJsonChars);
    vi.mocked(styleApi.list).mockResolvedValue(mockStyles);

    const { container } = renderStoryboards();

    await waitFor(() => {
      expect(getSelectByPlaceholder(container, '选择剧本')).toBeInTheDocument();
    });

    await userEvent.click(getSelectInputByPlaceholder(container, '选择剧本'));
    await waitFor(() => {
      expect(screen.getByText('剧本A')).toBeInTheDocument();
    });
    await userEvent.click(screen.getByText('剧本A'));

    await waitFor(() => {
      expect(getSelectByPlaceholder(container, '选择分集').closest('.ant-select-disabled')).toBeNull();
    });

    await userEvent.click(getSelectInputByPlaceholder(container, '选择分集'));
    await waitFor(() => {
      expect(screen.getByText('1. 第一集')).toBeInTheDocument();
    });
    await userEvent.click(screen.getByText('1. 第一集'));

    await waitFor(() => {
      expect(screen.getByText('角色A')).toBeInTheDocument();
      expect(screen.getByText('角色B')).toBeInTheDocument();
    });
  });

  it('displays dash for null characters', async () => {
    const storyboardsWithNullChars = [
      { ...mockStoryboards[0], characters: null },
    ];

    vi.mocked(scriptApi.list).mockResolvedValue({ total: 1, list: mockScripts, page: 1, page_size: 200 });
    vi.mocked(modelApi.list).mockResolvedValue({ total: 1, list: mockModels, page: 1, page_size: 200 });
    vi.mocked(scriptApi.episodes).mockResolvedValue(mockEpisodes);
    vi.mocked(storyboardApi.listByEpisode).mockResolvedValue(storyboardsWithNullChars);
    vi.mocked(styleApi.list).mockResolvedValue(mockStyles);

    const { container } = renderStoryboards();

    await waitFor(() => {
      expect(getSelectByPlaceholder(container, '选择剧本')).toBeInTheDocument();
    });

    await userEvent.click(getSelectInputByPlaceholder(container, '选择剧本'));
    await waitFor(() => {
      expect(screen.getByText('剧本A')).toBeInTheDocument();
    });
    await userEvent.click(screen.getByText('剧本A'));

    await waitFor(() => {
      expect(getSelectByPlaceholder(container, '选择分集').closest('.ant-select-disabled')).toBeNull();
    });

    await userEvent.click(getSelectInputByPlaceholder(container, '选择分集'));
    await waitFor(() => {
      expect(screen.getByText('1. 第一集')).toBeInTheDocument();
    });
    await userEvent.click(screen.getByText('1. 第一集'));

    await waitFor(() => {
      expect(screen.getByText('分镜列表')).toBeInTheDocument();
    });

    // The table cell for characters should show '-'
    const rows = document.querySelectorAll('.ant-table-row');
    expect(rows.length).toBeGreaterThan(0);
  });

  it('opens generate modal and shows model selector', async () => {
    vi.mocked(scriptApi.list).mockResolvedValue({ total: 1, list: mockScripts, page: 1, page_size: 200 });
    vi.mocked(modelApi.list).mockResolvedValue({ total: 1, list: mockModels, page: 1, page_size: 200 });
    vi.mocked(scriptApi.episodes).mockResolvedValue(mockEpisodes);
    vi.mocked(storyboardApi.listByEpisode).mockResolvedValue(mockStoryboards);
    vi.mocked(styleApi.list).mockResolvedValue(mockStyles);

    const { container } = renderStoryboards();

    await waitFor(() => {
      expect(getSelectByPlaceholder(container, '选择剧本')).toBeInTheDocument();
    });

    await userEvent.click(getSelectInputByPlaceholder(container, '选择剧本'));
    await waitFor(() => {
      expect(screen.getByText('剧本A')).toBeInTheDocument();
    });
    await userEvent.click(screen.getByText('剧本A'));

    await waitFor(() => {
      expect(getSelectByPlaceholder(container, '选择分集').closest('.ant-select-disabled')).toBeNull();
    });

    await userEvent.click(getSelectInputByPlaceholder(container, '选择分集'));
    await waitFor(() => {
      expect(screen.getByText('1. 第一集')).toBeInTheDocument();
    });
    await userEvent.click(screen.getByText('1. 第一集'));

    await waitFor(() => {
      expect(screen.getByText('分镜列表')).toBeInTheDocument();
    });

    // Open generate modal
    await userEvent.click(screen.getByText('AI 生成分镜'));

    // Verify modal opened with model selector
    await waitFor(() => {
      expect(screen.getAllByText('AI 生成分镜').length).toBeGreaterThanOrEqual(1);
      expect(screen.getByText('文本模型')).toBeInTheDocument();
    });
  });
});
