import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import ModelsPage from './Models';
import { modelApi } from '@/api/modules';

const messageMock = {
  success: vi.fn(),
  error: vi.fn(),
  warning: vi.fn(),
};

vi.mock('antd', async () => {
  const actual = await vi.importActual<typeof import('antd')>('antd');
  return {
    ...actual,
    App: {
      ...actual.App,
      useApp: () => ({ message: messageMock, modal: actual.App.useApp?.().modal || {} }),
    },
  };
});

vi.mock('@/api/modules', async () => {
  const actual = await vi.importActual<typeof import('@/api/modules')>('@/api/modules');
  return {
    ...actual,
    modelApi: {
      list: vi.fn(),
      get: vi.fn(),
      create: vi.fn(),
      update: vi.fn(),
      delete: vi.fn(),
      healthcheck: vi.fn(),
    },
  };
});

function renderModels() {
  return render(<ModelsPage />);
}

describe('ModelsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders with title', async () => {
    const mockedList = vi.mocked(modelApi.list);
    mockedList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });

    renderModels();

    await waitFor(() => {
      expect(screen.getByText('模型管理')).toBeInTheDocument();
    });
  });

  it('shows loading state initially', async () => {
    const mockedList = vi.mocked(modelApi.list);
    mockedList.mockReturnValue(new Promise(() => {}));

    renderModels();

    await waitFor(() => {
      expect(document.querySelector('[aria-busy="true"]')).toBeInTheDocument();
    });
  });

  it('displays empty state when no models', async () => {
    const mockedList = vi.mocked(modelApi.list);
    mockedList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });

    renderModels();

    await waitFor(() => {
      expect(screen.getByText('暂无数据')).toBeInTheDocument();
    });
  });

  it('displays model list with type tags', async () => {
    const mockedList = vi.mocked(modelApi.list);
    mockedList.mockResolvedValue({
      total: 3,
      list: [
        { id: 1, code: 'gpt-4o', name: 'GPT-4o', type: 'text', provider: 'openai', endpoint: 'https://api.openai.com/v1', default_params: {}, capability_tags: [], enabled: 1, priority: 10, max_qps: 10, health_check_url: '', last_health_status: 1, last_health_at: '2024-01-01T10:00:00Z' },
        { id: 2, code: 'sdxl', name: 'SDXL', type: 'image', provider: 'stability', endpoint: 'https://api.stability.ai/v1', default_params: {}, capability_tags: ['t2i'], enabled: 1, priority: 5, max_qps: 5, health_check_url: '', last_health_status: 2, last_health_at: '2024-01-01T09:00:00Z' },
        { id: 3, code: 'kling-v1', name: '可灵', type: 'video', provider: 'kuaishou', endpoint: 'https://api.kling.com/v1', default_params: {}, capability_tags: [], enabled: 2, priority: 8, max_qps: 3, health_check_url: '', last_health_status: 0, last_health_at: undefined },
      ],
      page: 1,
      page_size: 20,
    });

    renderModels();

    await waitFor(() => {
      expect(screen.getByText('GPT-4o')).toBeInTheDocument();
    });

    expect(screen.getByText('SDXL')).toBeInTheDocument();
    expect(screen.getByText('可灵')).toBeInTheDocument();
    expect(screen.getByText('text')).toBeInTheDocument();
    expect(screen.getByText('image')).toBeInTheDocument();
    expect(screen.getByText('video')).toBeInTheDocument();
    expect(screen.getByText('openai')).toBeInTheDocument();
    expect(screen.getByText('stability')).toBeInTheDocument();
    expect(screen.getByText('kuaishou')).toBeInTheDocument();
  });

  it('displays pagination info', async () => {
    const mockedList = vi.mocked(modelApi.list);
    mockedList.mockResolvedValue({
      total: 50,
      list: [
        { id: 1, code: 'gpt-4o', name: 'GPT-4o', type: 'text', provider: 'openai', endpoint: 'https://api.openai.com/v1', default_params: {}, capability_tags: [], enabled: 1, priority: 10, max_qps: 10, health_check_url: '', last_health_status: 1 },
      ],
      page: 1,
      page_size: 20,
    });

    renderModels();

    await waitFor(() => {
      expect(screen.getByText('GPT-4o')).toBeInTheDocument();
    });
  });

  it('opens create modal when clicking 注册模型', async () => {
    const user = userEvent.setup();
    const mockedList = vi.mocked(modelApi.list);
    mockedList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });

    renderModels();

    await waitFor(() => {
      expect(screen.getByText('注册模型')).toBeInTheDocument();
    });

    await user.click(screen.getByText('注册模型'));

    await waitFor(() => {
      expect(screen.getByText('注册模型')).toBeInTheDocument();
    });

    expect(screen.getByLabelText(/编码/i)).toBeInTheDocument();
    expect(screen.getByLabelText('名称')).toBeInTheDocument();
    expect(screen.getByLabelText('模型类型')).toBeInTheDocument();
    expect(screen.getByLabelText('提供方')).toBeInTheDocument();
    expect(screen.getByLabelText(/端点/i)).toBeInTheDocument();
  });

  it('creates a new model successfully', async () => {
    const user = userEvent.setup();
    const mockedList = vi.mocked(modelApi.list);
    const mockedCreate = vi.mocked(modelApi.create);

    mockedList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });
    mockedCreate.mockResolvedValue({
      id: 10, code: 'dall-e-3', name: 'DALL-E 3', type: 'image', provider: 'openai',
      endpoint: 'https://api.openai.com/v1', default_params: {}, capability_tags: [],
      enabled: 1, priority: 10, max_qps: 5, health_check_url: '', last_health_status: 0,
    });

    renderModels();

    await waitFor(() => {
      expect(screen.getByText('注册模型')).toBeInTheDocument();
    });

    await user.click(screen.getByText('注册模型'));

    await waitFor(() => {
      expect(screen.getByLabelText(/编码/i)).toBeInTheDocument();
    });

    await user.type(screen.getByLabelText(/编码/i), 'dall-e-3');
    await user.type(screen.getByLabelText('名称'), 'DALL-E 3');
    await user.type(screen.getByLabelText('提供方'), 'openai');
    await user.type(screen.getByLabelText(/端点/i), 'https://api.openai.com/v1');

    await user.click(screen.getByRole('button', { name: /确定/i }));

    await waitFor(() => {
      expect(mockedCreate).toHaveBeenCalledWith(
        expect.objectContaining({ code: 'dall-e-3', name: 'DALL-E 3' })
      );
    });
  });

  it('opens edit modal with pre-filled values', async () => {
    const user = userEvent.setup();
    const mockedList = vi.mocked(modelApi.list);
    const mockedGet = vi.mocked(modelApi.get);

    mockedList.mockResolvedValue({
      total: 1,
      list: [
        { id: 1, code: 'gpt-4o', name: 'GPT-4o', type: 'text', provider: 'openai', endpoint: 'https://api.openai.com/v1', default_params: { _model: 'gpt-4o' }, capability_tags: [], enabled: 1, priority: 10, max_qps: 10, health_check_url: '', last_health_status: 1 },
      ],
      page: 1,
      page_size: 20,
    });
    mockedGet.mockResolvedValue({
      id: 1, code: 'gpt-4o', name: 'GPT-4o', type: 'text', provider: 'openai', endpoint: 'https://api.openai.com/v1', default_params: { _model: 'gpt-4o' }, capability_tags: [], enabled: 1, priority: 10, max_qps: 10, health_check_url: '', last_health_status: 1,
    });

    renderModels();

    await waitFor(() => {
      expect(screen.getByText('GPT-4o')).toBeInTheDocument();
    });

    await user.click(screen.getAllByText('编辑')[0]);

    await waitFor(() => {
      expect(screen.getByText('编辑模型 - GPT-4o')).toBeInTheDocument();
    });

    const codeInput = screen.getByLabelText(/编码/i) as HTMLInputElement;
    const nameInput = screen.getByLabelText('名称') as HTMLInputElement;
    const endpointInput = screen.getByLabelText(/调用端点/i) as HTMLInputElement;
    await waitFor(() => {
      expect(codeInput.value).toBe('gpt-4o');
      expect(nameInput.value).toBe('GPT-4o');
      expect(endpointInput.value).toBe('https://api.openai.com/v1');
    });
  });

  it('updates a model successfully', async () => {
    const user = userEvent.setup();
    const mockedList = vi.mocked(modelApi.list);
    const mockedGet = vi.mocked(modelApi.get);
    const mockedUpdate = vi.mocked(modelApi.update);

    mockedList.mockResolvedValue({
      total: 1,
      list: [
        { id: 1, code: 'gpt-4o', name: 'GPT-4o', type: 'text', provider: 'openai', endpoint: 'https://api.openai.com/v1', default_params: {}, capability_tags: [], enabled: 1, priority: 10, max_qps: 10, health_check_url: '', last_health_status: 1 },
      ],
      page: 1,
      page_size: 20,
    });
    mockedGet.mockResolvedValue({
      id: 1, code: 'gpt-4o', name: 'GPT-4o', type: 'text', provider: 'openai', endpoint: 'https://api.openai.com/v1', default_params: {}, capability_tags: [], enabled: 1, priority: 10, max_qps: 10, health_check_url: '', last_health_status: 1,
    });
    mockedUpdate.mockResolvedValue({
      id: 1, code: 'gpt-4o', name: 'GPT-4o-Updated', type: 'text', provider: 'openai', endpoint: 'https://api.openai.com/v1', default_params: {}, capability_tags: [], enabled: 1, priority: 15, max_qps: 10, health_check_url: '', last_health_status: 1,
    });

    renderModels();

    await waitFor(() => {
      expect(screen.getByText('GPT-4o')).toBeInTheDocument();
    });

    await user.click(screen.getAllByText('编辑')[0]);

    await waitFor(() => {
      expect(screen.getByText('编辑模型 - GPT-4o')).toBeInTheDocument();
    });

    const nameInput = screen.getByLabelText('名称');
    await user.clear(nameInput);
    await user.type(nameInput, 'GPT-4o-Updated');

    await user.click(screen.getByRole('button', { name: /确定/i }));

    await waitFor(() => {
      expect(mockedUpdate).toHaveBeenCalledWith(
        1,
        expect.objectContaining({ name: 'GPT-4o-Updated' })
      );
    });
  });

  it('deletes a model after confirmation', async () => {
    const user = userEvent.setup();
    const mockedList = vi.mocked(modelApi.list);
    const mockedDelete = vi.mocked(modelApi.delete);

    mockedList.mockResolvedValue({
      total: 1,
      list: [
        { id: 1, code: 'old-model', name: '旧模型', type: 'text', provider: 'openai', endpoint: 'https://api.openai.com/v1', default_params: {}, capability_tags: [], enabled: 1, priority: 1, max_qps: 1, health_check_url: '', last_health_status: 0 },
      ],
      page: 1,
      page_size: 20,
    });
    mockedDelete.mockResolvedValue(undefined);

    renderModels();

    await waitFor(() => {
      expect(screen.getByText('旧模型')).toBeInTheDocument();
    });

    await user.click(screen.getByText('删除'));

    await waitFor(() => {
      expect(screen.getByText(/确认删除/)).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: /确定/i }));

    await waitFor(() => {
      expect(mockedDelete).toHaveBeenCalledWith(1);
    });
  });

  it('toggles model enabled status', async () => {
    const user = userEvent.setup();
    const mockedList = vi.mocked(modelApi.list);
    const mockedUpdate = vi.mocked(modelApi.update);

    mockedList.mockResolvedValue({
      total: 1,
      list: [
        { id: 1, code: 'gpt-4o', name: 'GPT-4o', type: 'text', provider: 'openai', endpoint: 'https://api.openai.com/v1', default_params: {}, capability_tags: [], enabled: 1, priority: 10, max_qps: 10, health_check_url: '', last_health_status: 1 },
      ],
      page: 1,
      page_size: 20,
    });
    mockedUpdate.mockResolvedValue({
      id: 1, code: 'gpt-4o', name: 'GPT-4o', type: 'text', provider: 'openai', endpoint: 'https://api.openai.com/v1', default_params: {}, capability_tags: [], enabled: 2, priority: 10, max_qps: 10, health_check_url: '', last_health_status: 1,
    });

    renderModels();

    await waitFor(() => {
      expect(screen.getByText('GPT-4o')).toBeInTheDocument();
    });

    const switchEl = document.querySelector('.ant-switch');
    expect(switchEl).toBeInTheDocument();

    if (switchEl) {
      await user.click(switchEl);
    }

    await waitFor(() => {
      expect(mockedUpdate).toHaveBeenCalledWith(1, { enabled: 2 });
    });
  });

  it('runs healthcheck successfully', async () => {
    const user = userEvent.setup();
    const mockedList = vi.mocked(modelApi.list);
    const mockedHealthcheck = vi.mocked(modelApi.healthcheck);

    mockedList.mockResolvedValue({
      total: 1,
      list: [
        { id: 1, code: 'gpt-4o', name: 'GPT-4o', type: 'text', provider: 'openai', endpoint: 'https://api.openai.com/v1', default_params: {}, capability_tags: [], enabled: 1, priority: 10, max_qps: 10, health_check_url: '', last_health_status: 0 },
      ],
      page: 1,
      page_size: 20,
    });
    mockedHealthcheck.mockResolvedValue({ healthy: true });

    renderModels();

    await waitFor(() => {
      expect(screen.getByText('GPT-4o')).toBeInTheDocument();
    });

    await user.click(screen.getByText('探活'));

    await waitFor(() => {
      expect(mockedHealthcheck).toHaveBeenCalledWith(1);
    });
  });

  it('handles healthcheck failure', async () => {
    const user = userEvent.setup();
    const mockedList = vi.mocked(modelApi.list);
    const mockedHealthcheck = vi.mocked(modelApi.healthcheck);

    mockedList.mockResolvedValue({
      total: 1,
      list: [
        { id: 1, code: 'gpt-4o', name: 'GPT-4o', type: 'text', provider: 'openai', endpoint: 'https://api.openai.com/v1', default_params: {}, capability_tags: [], enabled: 1, priority: 10, max_qps: 10, health_check_url: '', last_health_status: 0 },
      ],
      page: 1,
      page_size: 20,
    });
    mockedHealthcheck.mockResolvedValue({ healthy: false, error: 'Timeout' });

    renderModels();

    await waitFor(() => {
      expect(screen.getByText('GPT-4o')).toBeInTheDocument();
    });

    await user.click(screen.getByText('探活'));

    await waitFor(() => {
      expect(mockedHealthcheck).toHaveBeenCalledWith(1);
    });
  });

  it('filters by type', async () => {
    const user = userEvent.setup();
    const mockedList = vi.mocked(modelApi.list);

    mockedList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });

    renderModels();

    await waitFor(() => {
      expect(screen.getByText('模型管理')).toBeInTheDocument();
    });

    const typeSelect = document.querySelector('.ant-select');
    if (typeSelect) {
      await user.click(typeSelect);
    }

    await waitFor(() => {
      expect(mockedList).toHaveBeenCalled();
    });
  });

  it('searches by query', async () => {
    const user = userEvent.setup();
    const mockedList = vi.mocked(modelApi.list);

    mockedList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });

    renderModels();

    await waitFor(() => {
      expect(screen.getByText('模型管理')).toBeInTheDocument();
    });

    const searchInput = screen.getByPlaceholderText(/搜索/);
    await user.type(searchInput, 'gpt');
    await user.click(screen.getByRole('button', { name: /search/i }));

    await waitFor(() => {
      expect(mockedList).toHaveBeenCalledWith(
        expect.objectContaining({ q: 'gpt' })
      );
    });
  });

  it('handles API list error gracefully', async () => {
    const mockedList = vi.mocked(modelApi.list);
    mockedList.mockRejectedValue(new Error('Network error'));

    renderModels();

    await waitFor(() => {
      expect(screen.getByText('模型管理')).toBeInTheDocument();
    });
  });

  it('handles create API error gracefully', async () => {
    const user = userEvent.setup();
    const mockedList = vi.mocked(modelApi.list);
    const mockedCreate = vi.mocked(modelApi.create);

    mockedList.mockResolvedValue({ total: 0, list: [], page: 1, page_size: 20 });
    mockedCreate.mockRejectedValue(new Error('Create failed'));

    renderModels();

    await waitFor(() => {
      expect(screen.getByText('注册模型')).toBeInTheDocument();
    });

    await user.click(screen.getByText('注册模型'));

    await waitFor(() => {
      expect(screen.getByLabelText(/编码/i)).toBeInTheDocument();
    });

    await user.type(screen.getByLabelText(/编码/i), 'fail');
    await user.type(screen.getByLabelText('名称'), '失败模型');
    await user.type(screen.getByLabelText('提供方'), 'test');
    await user.type(screen.getByLabelText(/端点/i), 'https://test.com');

    await user.click(screen.getByRole('button', { name: /确定/i }));

    await waitFor(() => {
      expect(mockedCreate).toHaveBeenCalled();
    });
  });

  it('handles delete API error gracefully', async () => {
    const user = userEvent.setup();
    const mockedList = vi.mocked(modelApi.list);
    const mockedDelete = vi.mocked(modelApi.delete);

    mockedList.mockResolvedValue({
      total: 1,
      list: [
        { id: 1, code: 'del-fail', name: '删不掉', type: 'text', provider: 'openai', endpoint: 'https://api.openai.com/v1', default_params: {}, capability_tags: [], enabled: 1, priority: 1, max_qps: 1, health_check_url: '', last_health_status: 0 },
      ],
      page: 1,
      page_size: 20,
    });
    mockedDelete.mockRejectedValue(new Error('Delete failed'));

    renderModels();

    await waitFor(() => {
      expect(screen.getByText('删不掉')).toBeInTheDocument();
    });

    await user.click(screen.getByText('删除'));

    await waitFor(() => {
      expect(screen.getByText(/确认删除/)).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: /确定/i }));

    await waitFor(() => {
      expect(mockedDelete).toHaveBeenCalledWith(1);
    });
  });
});
