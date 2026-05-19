import { useEffect, useMemo, useState } from 'react';
import {
  App as AntApp,
  Button,
  Card,
  Empty,
  Form,
  Input,
  Modal,
  Popconfirm,
  Progress,
  Slider,
  Space,
  Switch,
  Table,
  Tag,
  Tooltip,
} from 'antd';
import {
  DeleteOutlined,
  EditOutlined,
  ExperimentOutlined,
  PlusOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';
import dayjs from 'dayjs';
import { FeatureFlag, featureFlagApi } from '@/api/modules';

type FlagRules = { users?: number[]; depts?: number[]; projects?: number[] };

function parseRules(v: FeatureFlag['rules']): FlagRules {
  if (!v) return {};
  if (typeof v === 'string') {
    try {
      const o = JSON.parse(v);
      return o && typeof o === 'object' ? (o as FlagRules) : {};
    } catch {
      return {};
    }
  }
  return v as FlagRules;
}

function rulesSummary(v: FeatureFlag['rules']): string {
  const r = parseRules(v);
  const u = r.users?.length ?? 0;
  const d = r.depts?.length ?? 0;
  const p = r.projects?.length ?? 0;
  if (!u && !d && !p) return '-';
  return `用户:${u} / 部门:${d} / 项目:${p}`;
}

const RULES_PLACEHOLDER = '{"users":[1,2],"depts":[10],"projects":[]}';

export default function FeatureFlagsPage() {
  const { message } = AntApp.useApp();

  const [list, setList] = useState<FeatureFlag[]>([]);
  const [loading, setLoading] = useState(false);

  const [editOpen, setEditOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<FeatureFlag | null>(null);
  const [editForm] = Form.useForm();

  const [evalOpen, setEvalOpen] = useState(false);
  const [evalKey, setEvalKey] = useState('');
  const [evalResult, setEvalResult] = useState<{ key: string; enabled: boolean } | null>(null);
  const [evalLoading, setEvalLoading] = useState(false);

  const isEdit = useMemo(() => editTarget !== null, [editTarget]);

  const fetchList = async () => {
    setLoading(true);
    try {
      const data = await featureFlagApi.list();
      setList(data || []);
    } catch (err) {
      message.error((err as Error)?.message || '加载灰度开关失败');
      setList([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchList();
  }, []);

  const openCreate = () => {
    setEditTarget(null);
    editForm.resetFields();
    editForm.setFieldsValue({
      key: '',
      description: '',
      enabled: false,
      rollout: 0,
      rules_json: '',
    });
    setEditOpen(true);
  };

  const openEdit = (f: FeatureFlag) => {
    setEditTarget(f);
    const rules = parseRules(f.rules);
    editForm.setFieldsValue({
      key: f.key,
      description: f.description,
      enabled: f.enabled === 1,
      rollout: f.rollout,
      rules_json: Object.keys(rules).length ? JSON.stringify(rules, null, 2) : '',
    });
    setEditOpen(true);
  };

  const onSave = async () => {
    const v = await editForm.validateFields();
    let rulesObj: Record<string, unknown> | undefined;
    const rulesStr = (v.rules_json as string | undefined)?.trim();
    if (rulesStr) {
      try {
        const parsed = JSON.parse(rulesStr);
        if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
          message.error('rules 必须是 JSON 对象');
          return;
        }
        rulesObj = parsed as Record<string, unknown>;
      } catch {
        message.error('rules JSON 解析失败');
        return;
      }
    }
    try {
      if (isEdit && editTarget) {
        await featureFlagApi.update(editTarget.id, {
          description: v.description,
          enabled: v.enabled ? 1 : 0,
          rollout: v.rollout,
          rules: rulesObj,
        });
        message.success('已保存');
      } else {
        await featureFlagApi.create({
          key: v.key,
          description: v.description,
          enabled: v.enabled ? 1 : 0,
          rollout: v.rollout,
          rules: rulesObj,
        });
        message.success('已创建');
      }
      setEditOpen(false);
      fetchList();
    } catch (err) {
      message.error((err as Error)?.message || '保存失败');
    }
  };

  const onDelete = async (id: number) => {
    try {
      await featureFlagApi.delete(id);
      message.success('已删除');
      fetchList();
    } catch (err) {
      message.error((err as Error)?.message || '删除失败');
    }
  };

  const toggleEnabled = async (record: FeatureFlag, enabled: boolean) => {
    try {
      await featureFlagApi.update(record.id, { enabled: enabled ? 1 : 0 });
      message.success(enabled ? '已启用' : '已禁用');
      fetchList();
    } catch (err) {
      message.error((err as Error)?.message || '状态更新失败');
    }
  };

  const openEvaluate = (key?: string) => {
    setEvalKey(key || '');
    setEvalResult(null);
    setEvalOpen(true);
  };

  const runEvaluate = async () => {
    if (!evalKey.trim()) {
      message.warning('请输入要评估的 key');
      return;
    }
    setEvalLoading(true);
    try {
      const r = await featureFlagApi.evaluate(evalKey.trim());
      setEvalResult(r ?? null);
    } catch (err) {
      message.error((err as Error)?.message || '评估失败');
      setEvalResult(null);
    } finally {
      setEvalLoading(false);
    }
  };

  return (
    <Card
      title={
        <Space>
          <ExperimentOutlined />
          灰度开关管理
        </Space>
      }
      extra={
        <Space>
          <Tooltip title="按 key 在线评估某个开关对当前用户是否命中">
            <Button icon={<ThunderboltOutlined />} onClick={() => openEvaluate()}>
              评估
            </Button>
          </Tooltip>
          <Tooltip title="新建灰度开关(可配置启用/灰度比例/白名单)">
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
              新建开关
            </Button>
          </Tooltip>
        </Space>
      }
    >
      <Table
        rowKey="id"
        size="small"
        loading={loading}
        dataSource={list}
        pagination={false}
        locale={{
          emptyText: (
            <Empty description="还没有灰度开关">
              <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
                新建开关
              </Button>
            </Empty>
          ),
        }}
        columns={[
          { title: 'ID', dataIndex: 'id', width: 70 },
          {
            title: 'Key',
            dataIndex: 'key',
            width: 200,
            render: (v: string) => <Tag color="geekblue">{v}</Tag>,
          },
          { title: '描述', dataIndex: 'description', ellipsis: true },
          {
            title: '启用',
            dataIndex: 'enabled',
            width: 80,
            render: (v: number, r: FeatureFlag) => (
              <Tooltip title={v === 1 ? '点击禁用该开关' : '点击启用该开关'}>
                <Switch checked={v === 1} onChange={(c) => toggleEnabled(r, c)} />
              </Tooltip>
            ),
          },
          {
            title: '灰度',
            dataIndex: 'rollout',
            width: 140,
            render: (v: number) => (
              <Progress percent={Number(v) || 0} size="small" style={{ width: 120 }} />
            ),
          },
          {
            title: '规则',
            dataIndex: 'rules',
            width: 180,
            render: (v: FeatureFlag['rules']) => <span>{rulesSummary(v)}</span>,
          },
          {
            title: '创建时间',
            dataIndex: 'created_at',
            width: 170,
            render: (v: string) => (v ? dayjs(v).format('YYYY-MM-DD HH:mm:ss') : '-'),
          },
          {
            title: '操作',
            key: 'op',
            width: 240,
            render: (_: unknown, r: FeatureFlag) => (
              <Space size={4}>
                <Tooltip title="编辑该开关的描述/灰度/白名单(key 不可改)">
                  <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(r)}>
                    编辑
                  </Button>
                </Tooltip>
                <Tooltip title="基于该 key 对当前账号在线评估命中结果">
                  <Button
                    size="small"
                    icon={<ThunderboltOutlined />}
                    onClick={() => openEvaluate(r.key)}
                  >
                    评估
                  </Button>
                </Tooltip>
                <Popconfirm title="确认删除该开关?" onConfirm={() => onDelete(r.id)}>
                  <Tooltip title="删除该灰度开关(不可恢复)">
                    <Button size="small" danger icon={<DeleteOutlined />}>
                      删除
                    </Button>
                  </Tooltip>
                </Popconfirm>
              </Space>
            ),
          },
        ]}
      />

      <Modal
        open={editOpen}
        title={isEdit ? `编辑开关 - ${editTarget?.key ?? ''}` : '新建开关'}
        onCancel={() => setEditOpen(false)}
        onOk={onSave}
        width={640}
        destroyOnClose
      >
        <Form form={editForm} layout="vertical">
          <Form.Item
            name="key"
            label="Key"
            rules={[
              { required: true, message: '请输入 key' },
              { max: 128 },
              {
                pattern: /^[a-zA-Z0-9_.\-:]+$/,
                message: '只能包含字母、数字、_ . - :',
              },
            ]}
            extra="key 全局唯一,创建后不可修改"
          >
            <Input placeholder="如:video.publish.batch" disabled={isEdit} />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea rows={2} placeholder="开关用途说明" />
          </Form.Item>
          <Form.Item name="enabled" label="启用" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="rollout" label="灰度比例(%)">
            <Slider min={0} max={100} step={5} marks={{ 0: '0', 50: '50', 100: '100' }} />
          </Form.Item>
          <Form.Item
            name="rules_json"
            label="命中规则(JSON)"
            extra="支持 users / depts / projects 三类白名单,留空表示无白名单"
          >
            <Input.TextArea rows={6} placeholder={RULES_PLACEHOLDER} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        open={evalOpen}
        title="开关评估"
        onCancel={() => setEvalOpen(false)}
        onOk={runEvaluate}
        okText="评估"
        confirmLoading={evalLoading}
        destroyOnClose
      >
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <Input
            placeholder="输入要评估的 key"
            value={evalKey}
            onChange={(e) => setEvalKey(e.target.value)}
            onPressEnter={runEvaluate}
          />
          {evalResult && (
            <div>
              <Tag color="geekblue">{evalResult.key}</Tag>
              结果:
              <Tag color={evalResult.enabled ? 'success' : 'default'} style={{ marginLeft: 8 }}>
                {evalResult.enabled ? '命中(enabled)' : '未命中(disabled)'}
              </Tag>
            </div>
          )}
        </Space>
      </Modal>
    </Card>
  );
}
