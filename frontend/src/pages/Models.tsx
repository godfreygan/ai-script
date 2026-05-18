import { useEffect, useState } from 'react';
import {
  App as AntApp,
  Button,
  Card,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  Tooltip,
} from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  HeartOutlined,
  CheckCircleTwoTone,
  CloseCircleTwoTone,
  QuestionCircleTwoTone,
} from '@ant-design/icons';
import { Model, modelApi } from '@/api/modules';

const typeColors: Record<string, string> = {
  text: 'blue',
  image: 'magenta',
  video: 'purple',
  audio: 'cyan',
  multi: 'gold',
};

const healthIcon = (s: number) => {
  if (s === 1) return <CheckCircleTwoTone twoToneColor="#52c41a" />;
  if (s === 2) return <CloseCircleTwoTone twoToneColor="#ff4d4f" />;
  return <QuestionCircleTwoTone twoToneColor="#bfbfbf" />;
};

interface FormValues {
  code: string;
  name: string;
  type: string;
  provider: string;
  endpoint: string;
  api_key?: string;
  model_name?: string;
  capability_tags?: string[];
  priority?: number;
  max_qps?: number;
  health_check_url?: string;
  enabled?: boolean;
}

function parseDefaultParams(raw: Model['default_params'] | undefined): Record<string, unknown> {
  if (!raw) return {};
  if (typeof raw === 'string') {
    try {
      const parsed = JSON.parse(raw) as unknown;
      return typeof parsed === 'object' && parsed !== null && !Array.isArray(parsed)
        ? (parsed as Record<string, unknown>)
        : {};
    } catch {
      return {};
    }
  }
  return raw;
}

function parseCapabilityTags(raw: Model['capability_tags'] | unknown): string[] {
  if (!raw) return [];
  if (Array.isArray(raw)) return raw.map(String);
  if (typeof raw === 'string') {
    try {
      const parsed = JSON.parse(raw) as unknown;
      if (Array.isArray(parsed)) return parsed.map(String);
    } catch {
      return [];
    }
  }
  return [];
}

function modelToFormValues(m: Model): FormValues {
  const params = parseDefaultParams(m.default_params);
  return {
    code: m.code,
    name: m.name,
    type: m.type,
    provider: m.provider,
    endpoint: m.endpoint,
    api_key: '',
    model_name: typeof params._model === 'string' ? params._model : '',
    capability_tags: parseCapabilityTags(m.capability_tags),
    priority: m.priority,
    max_qps: m.max_qps,
    health_check_url: m.health_check_url || '',
    enabled: m.enabled === 1,
  };
}

const createFormDefaults: Partial<FormValues> = {
  type: 'image',
  provider: 'litellm',
  priority: 10,
  max_qps: 5,
  enabled: true,
};

export default function ModelsPage() {
  const { message } = AntApp.useApp();
  const [list, setList] = useState<Model[]>([]);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [q, setQ] = useState('');
  const [typeFilter, setTypeFilter] = useState<string | undefined>();

  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<Model | null>(null);
  const [modalLoading, setModalLoading] = useState(false);
  const [healthcheckingId, setHealthcheckingId] = useState<number | null>(null);
  const [formKey, setFormKey] = useState('create');
  const [formInitialValues, setFormInitialValues] =
    useState<Partial<FormValues>>(createFormDefaults);
  const [form] = Form.useForm<FormValues>();

  const applyModalFormValues = (values: Partial<FormValues>) => {
    form.resetFields();
    form.setFieldsValue(values);
  };

  const fetchList = async () => {
    setLoading(true);
    try {
      const data = await modelApi.list({ page, page_size: pageSize, q, type: typeFilter });
      setList(data.list);
      setTotal(data.total);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchList();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, pageSize, typeFilter]);

  const openCreate = () => {
    setEditing(null);
    setFormInitialValues(createFormDefaults);
    setFormKey(`create-${Date.now()}`);
    setOpen(true);
  };

  const openEdit = async (m: Model) => {
    setModalLoading(true);
    try {
      const full = await modelApi.get(m.id);
      const values = modelToFormValues(full);
      setEditing(full);
      setFormInitialValues(values);
      setFormKey(`edit-${full.id}`);
      setOpen(true);
    } catch (e) {
      message.error((e as Error).message || '加载模型失败');
    } finally {
      setModalLoading(false);
    }
  };

  const handleModalAfterOpenChange = (visible: boolean) => {
    if (!visible) {
      setEditing(null);
      return;
    }
    // destroyOnClose 下 Form 晚于 open 挂载，须在动画结束后再写入
    requestAnimationFrame(() => {
      applyModalFormValues(formInitialValues);
    });
  };

  const onSubmit = async () => {
    const v = await form.validateFields();
    const defaultParams: Record<string, unknown> = v.model_name ? { _model: v.model_name } : {};

    if (editing) {
      await modelApi.update(editing.id, {
        name: v.name,
        endpoint: v.endpoint,
        api_key: v.api_key || undefined,
        default_params: defaultParams,
        capability_tags: v.capability_tags || [],
        enabled: v.enabled ? 1 : 2,
        priority: v.priority,
        max_qps: v.max_qps,
        health_check_url: v.health_check_url,
      });
      message.success('已更新');
    } else {
      await modelApi.create({
        code: v.code,
        name: v.name,
        type: v.type,
        provider: v.provider,
        endpoint: v.endpoint,
        api_key: v.api_key,
        model_name: v.model_name,
        default_params: defaultParams,
        capability_tags: v.capability_tags || [],
        priority: v.priority,
        max_qps: v.max_qps,
        health_check_url: v.health_check_url,
      });
      message.success('已创建');
    }
    setOpen(false);
    fetchList();
  };

  const onDelete = async (m: Model) => {
    await modelApi.delete(m.id);
    message.success('已删除');
    fetchList();
  };

  const onToggleEnabled = async (m: Model, enabled: boolean) => {
    await modelApi.update(m.id, { enabled: enabled ? 1 : 2 });
    message.success(enabled ? '已启用' : '已停用');
    fetchList();
  };

  const onHealthcheck = async (m: Model) => {
    if (healthcheckingId !== null) return;
    setHealthcheckingId(m.id);
    try {
      const r = await modelApi.healthcheck(m.id);
      if (r.healthy) {
        message.success(`${m.name} 健康`);
      } else {
        message.error(`${m.name} 异常: ${r.error || '未知'}`);
      }
      await fetchList();
    } catch (e) {
      const msg = e instanceof Error ? e.message : '探活请求失败';
      message.error(msg);
    } finally {
      setHealthcheckingId(null);
    }
  };

  return (
    <Card
      title="模型管理"
      extra={
        <Space>
          <Select
            allowClear
            placeholder="类型筛选"
            style={{ width: 140 }}
            value={typeFilter}
            onChange={(v) => {
              setTypeFilter(v);
              setPage(1);
            }}
            options={[
              { label: '文本 (text)', value: 'text' },
              { label: '图像 (image)', value: 'image' },
              { label: '视频 (video)', value: 'video' },
              { label: '音频 (audio)', value: 'audio' },
              { label: '多模态 (multi)', value: 'multi' },
            ]}
          />
          <Input.Search
            placeholder="搜索 code / name"
            allowClear
            style={{ width: 240 }}
            onSearch={(v) => {
              setQ(v);
              setPage(1);
              fetchList();
            }}
          />
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            注册模型
          </Button>
        </Space>
      }
    >
      <Table
        loading={loading}
        rowKey="id"
        dataSource={list}
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          onChange: (p, ps) => {
            setPage(p);
            setPageSize(ps);
          },
        }}
        columns={[
          { title: 'ID', dataIndex: 'id', width: 60 },
          { title: '编码', dataIndex: 'code', width: 160 },
          { title: '名称', dataIndex: 'name', width: 160 },
          {
            title: '类型',
            dataIndex: 'type',
            width: 90,
            render: (t: string) => <Tag color={typeColors[t] || 'default'}>{t}</Tag>,
          },
          { title: '提供方', dataIndex: 'provider', width: 100 },
          {
            title: '端点',
            dataIndex: 'endpoint',
            ellipsis: true,
            render: (s: string) => (
              <Tooltip title={s}>
                <span style={{ fontFamily: 'monospace', fontSize: 12 }}>{s}</span>
              </Tooltip>
            ),
          },
          {
            title: '优先级',
            dataIndex: 'priority',
            width: 80,
          },
          {
            title: 'QPS',
            dataIndex: 'max_qps',
            width: 70,
          },
          {
            title: '启用',
            dataIndex: 'enabled',
            width: 70,
            render: (v: number, m: Model) => (
              <Switch
                checked={v === 1}
                size="small"
                onChange={(c) => onToggleEnabled(m, c)}
              />
            ),
          },
          {
            title: '健康',
            dataIndex: 'last_health_status',
            width: 130,
            render: (s: number, m: Model) => (
              <Tooltip title={m.last_health_at || '未检测'}>
                <Space size={4}>
                  {healthIcon(s)}
                  <span style={{ fontSize: 12, color: '#999' }}>
                    {m.last_health_at ? m.last_health_at.slice(5, 16) : '-'}
                  </span>
                </Space>
              </Tooltip>
            ),
          },
          {
            title: '操作',
            width: 230,
            render: (_: unknown, m: Model) => (
              <Space>
                <Button
                  size="small"
                  icon={<HeartOutlined />}
                  loading={healthcheckingId === m.id}
                  disabled={healthcheckingId !== null && healthcheckingId !== m.id}
                  onClick={() => onHealthcheck(m)}
                >
                  探活
                </Button>
                <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(m)}>
                  编辑
                </Button>
                <Popconfirm
                  title={`确认删除 ${m.name}?`}
                  onConfirm={() => onDelete(m)}
                  okType="danger"
                >
                  <Button size="small" danger icon={<DeleteOutlined />}>
                    删除
                  </Button>
                </Popconfirm>
              </Space>
            ),
          },
        ]}
      />

      <Modal
        title={editing ? `编辑模型 - ${editing.name}` : '注册模型'}
        open={open}
        width={720}
        confirmLoading={modalLoading}
        onCancel={() => setOpen(false)}
        onOk={onSubmit}
        destroyOnClose
        afterOpenChange={handleModalAfterOpenChange}
      >
        <Form
          key={formKey}
          form={form}
          layout="vertical"
          preserve={false}
          initialValues={formInitialValues}
        >
          <Form.Item
            name="code"
            label="编码 (code)"
            rules={[{ required: !editing, message: '请输入编码' }]}
            tooltip="模型注册唯一标识,如 kling-v1 / sdxl-base / gpt-4o"
          >
            <Input disabled={!!editing} placeholder="kling-v1" />
          </Form.Item>
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input placeholder="可灵 v1 文生视频" />
          </Form.Item>
          <Space.Compact block>
            <Form.Item
              name="type"
              label="模型类型"
              rules={[{ required: true }]}
              style={{ flex: 1, marginRight: 8 }}
            >
              <Select
                disabled={!!editing}
                options={[
                  { label: '文本 (text)', value: 'text' },
                  { label: '图像 (image)', value: 'image' },
                  { label: '视频 (video)', value: 'video' },
                  { label: '音频 (audio)', value: 'audio' },
                  { label: '多模态 (multi)', value: 'multi' },
                ]}
              />
            </Form.Item>
            <Form.Item
              name="provider"
              label="提供方"
              rules={[{ required: true }]}
              style={{ flex: 1 }}
              tooltip="如 litellm / openai / aliyun / kuaishou-kling / volc-jimeng"
            >
              <Input placeholder="litellm" />
            </Form.Item>
          </Space.Compact>
          <Form.Item
            name="endpoint"
            label="调用端点 (endpoint)"
            rules={[{ required: true, message: '请输入端点 URL' }]}
            tooltip="兼容 OpenAI 协议的 base url,如 https://one-api.local/v1"
          >
            <Input placeholder="https://one-api.example.com/v1" />
          </Form.Item>
          <Form.Item
            name="model_name"
            label="上游模型名 (model_name)"
            tooltip="网关侧实际识别的模型 ID,会写入 default_params._model"
          >
            <Input placeholder="kling-1.5-pro / dall-e-3 / gpt-4o" />
          </Form.Item>
          <Form.Item
            name="api_key"
            label={editing ? '替换 API Key (留空保持原值)' : 'API Key'}
            rules={
              editing
                ? []
                : [{ required: true, message: '请输入 API Key,服务端 AES-256-GCM 加密存储' }]
            }
            tooltip="服务端 AES-256-GCM 加密后落库,前端无法回读"
          >
            <Input.Password
              autoComplete="new-password"
              placeholder={editing ? '••••••••(不修改请留空)' : 'sk-xxxx'}
            />
          </Form.Item>
          <Form.Item
            name="capability_tags"
            label="能力标签"
            tooltip="模型能力标签,用于流水线节点路由"
          >
            <Select
              mode="tags"
              tokenSeparators={[',']}
              placeholder="如 t2i / i2v / hires / lora"
            />
          </Form.Item>
          <Space.Compact block>
            <Form.Item name="priority" label="优先级" style={{ flex: 1, marginRight: 8 }}>
              <InputNumber min={0} max={100} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="max_qps" label="最大 QPS" style={{ flex: 1, marginRight: 8 }}>
              <InputNumber min={0} max={1000} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item
              name="enabled"
              label="启用"
              valuePropName="checked"
              style={{ width: 80 }}
            >
              <Switch />
            </Form.Item>
          </Space.Compact>
          <Form.Item
            name="health_check_url"
            label="健康检查 URL"
            tooltip="可选,留空时使用 endpoint + /models 探活"
          >
            <Input placeholder="https://.../health" />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
}
