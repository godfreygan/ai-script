import { useEffect, useMemo, useState } from 'react';
import {
  Alert,
  App as AntApp,
  Button,
  Card,
  Drawer,
  Empty,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Progress,
  Select,
  Space,
  Table,
  Tag,
  Tooltip,
  Typography,
} from 'antd';
import {
  DeleteOutlined,
  EyeOutlined,
  PlusOutlined,
  ScissorOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import {
  Episode,
  Model,
  Project,
  Script,
  modelApi,
  projectApi,
  scriptApi,
} from '@/api/modules';
import { useProgressWS } from '@/hooks/useProgressWS';

const statusMap: Record<number, { color: string; label: string }> = {
  0: { color: 'default', label: '草稿' },
  1: { color: 'blue', label: '已上传' },
  2: { color: 'processing', label: '拆分中' },
  3: { color: 'success', label: '已拆分' },
  4: { color: 'default', label: '已归档' },
};

export default function ScriptsPage() {
  const { message } = AntApp.useApp();
  const navigate = useNavigate();

  const [list, setList] = useState<Script[]>([]);
  const [projects, setProjects] = useState<Project[]>([]);
  const [models, setModels] = useState<Model[]>([]);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [q, setQ] = useState('');
  const [projectId, setProjectId] = useState<number | undefined>();
  const [statusFilter, setStatusFilter] = useState<number | undefined>();

  const [createOpen, setCreateOpen] = useState(false);
  const [createForm] = Form.useForm();

  const [splitOpen, setSplitOpen] = useState(false);
  const [splitTarget, setSplitTarget] = useState<Script | null>(null);
  const [splitForm] = Form.useForm();
  const [splitTopic, setSplitTopic] = useState<string | null>(null);

  const [episodeDrawerOpen, setEpisodeDrawerOpen] = useState(false);
  const [episodeTarget, setEpisodeTarget] = useState<Script | null>(null);
  const [episodes, setEpisodes] = useState<Episode[]>([]);
  const [episodeLoading, setEpisodeLoading] = useState(false);

  const splitProgress = useProgressWS(splitTopic);
  const splitPercent = useMemo(() => {
    if (!splitProgress.last) return 0;
    return Math.round((splitProgress.last.percent ?? 0) * 100);
  }, [splitProgress.last]);

  const fetchList = async () => {
    setLoading(true);
    try {
      const data = await scriptApi.list({
        page,
        page_size: pageSize,
        q,
        project_id: projectId,
        status: statusFilter,
      });
      // 防护: data 为 undefined 时避免炸
      setList(data?.list ?? []);
      setTotal(data?.total ?? 0);
    } catch (err) {
      message.error((err as Error)?.message || '加载剧本列表失败');
      setList([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  };

  const fetchRefs = async () => {
    try {
      const [p, m] = await Promise.all([
        projectApi.list({ page_size: 200 }),
        modelApi.list({ page_size: 200, type: 'text', enabled: 1 }),
      ]);
      // 防护: 接口返回 undefined 时避免炸
      setProjects(p?.list ?? []);
      setModels(m?.list ?? []);
    } catch (err) {
      message.error((err as Error)?.message || '加载参考数据失败');
    }
  };

  useEffect(() => {
    fetchRefs();
  }, []);

  useEffect(() => {
    fetchList();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, pageSize, projectId, statusFilter]);

  // 当 done/error 时刷新列表
  useEffect(() => {
    if (!splitProgress.last) return;
    const t = splitProgress.last.type;
    if (t === 'done') {
      message.success(splitProgress.last.message || '拆分完成');
      fetchList();
    } else if (t === 'error') {
      message.error(splitProgress.last.message || '拆分失败');
      fetchList();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [splitProgress.last?.type, splitProgress.last?.time]);

  const closeSplit = () => {
    setSplitOpen(false);
    // 断开 WS 订阅,避免 modal 关闭后仍占用连接
    setSplitTopic(null);
    setSplitTarget(null);
    splitForm.resetFields();
  };

  const onCreate = async () => {
    const v = await createForm.validateFields();
    try {
      await scriptApi.create(v);
      message.success('剧本已上传');
      setCreateOpen(false);
      createForm.resetFields();
      fetchList();
    } catch (err) {
      message.error((err as Error)?.message || '创建失败');
    }
  };

  const onDelete = async (id: number) => {
    try {
      await scriptApi.delete(id);
      message.success('已删除');
      fetchList();
    } catch (err) {
      message.error((err as Error)?.message || '删除失败');
    }
  };

  const openSplit = (sc: Script) => {
    setSplitTarget(sc);
    splitForm.resetFields();
    splitForm.setFieldsValue({ episode_count: 12, target_chars: 800 });
    setSplitTopic(null);
    setSplitOpen(true);
  };

  // 状态变为 succeeded(3=已拆分) 后,若当前抽屉打开同一剧本则重拉分集
  useEffect(() => {
    if (splitProgress.last?.type !== 'done') return;
    if (episodeDrawerOpen && episodeTarget) {
      void openEpisodes(episodeTarget);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [splitProgress.last?.type, splitProgress.last?.time]);

  const onSplitSubmit = async () => {
    if (!splitTarget) return;
    const v = await splitForm.validateFields();
    try {
      const r = await scriptApi.split(splitTarget.id, v);
      setSplitTopic(r?.topic ?? null);
      message.success(`已入队任务 ${r?.task_id}`);
    } catch (err) {
      message.error((err as Error)?.message || '拆分失败');
    }
  };

  const openEpisodes = async (sc: Script) => {
    setEpisodeTarget(sc);
    setEpisodeDrawerOpen(true);
    setEpisodeLoading(true);
    try {
      const eps = await scriptApi.episodes(sc.id);
      // 防护: 接口返回 undefined 时避免炸
      setEpisodes(eps ?? []);
    } catch (err) {
      message.error((err as Error)?.message || '加载分集失败');
      setEpisodes([]);
    } finally {
      setEpisodeLoading(false);
    }
  };

  return (
    <Card
      title="剧本管理"
      extra={
        <Space>
          <Input.Search
            allowClear
            placeholder="搜索名称/原文"
            style={{ width: 220 }}
            value={q}
            onChange={(e) => setQ(e.target.value)}
            onSearch={() => {
              setPage(1);
              fetchList();
            }}
          />
          <Select
            allowClear
            placeholder="项目"
            style={{ width: 180 }}
            value={projectId}
            options={projects.map((p) => ({ label: p.name, value: p.id }))}
            onChange={(v) => {
              setProjectId(v);
              setPage(1);
            }}
          />
          <Select
            allowClear
            placeholder="状态"
            style={{ width: 120 }}
            value={statusFilter}
            options={Object.entries(statusMap).map(([k, v]) => ({
              label: v.label,
              value: Number(k),
            }))}
            onChange={(v) => {
              setStatusFilter(v);
              setPage(1);
            }}
          />
          <Tooltip title="上传新的剧本原文">
            <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
              新建剧本
            </Button>
          </Tooltip>
        </Space>
      }
    >
      <Table
        rowKey="id"
        loading={loading}
        dataSource={list}
        locale={{
          emptyText: (
            <Empty description="还没有剧本">
              <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
                立即上传剧本
              </Button>
            </Empty>
          ),
        }}
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
          { title: 'ID', dataIndex: 'id', width: 80 },
          { title: '名称', dataIndex: 'name' },
          {
            title: '项目',
            dataIndex: 'project_id',
            width: 160,
            render: (v: number) => projects.find((p) => p.id === v)?.name || `#${v}`,
          },
          {
            title: '版本',
            dataIndex: 'current_version',
            width: 80,
            render: (v: number) => <Tag>v{v}</Tag>,
          },
          {
            title: '状态',
            dataIndex: 'status',
            width: 100,
            render: (s: number) => {
              const m = statusMap[s] || statusMap[0];
              return <Tag color={m.color}>{m.label}</Tag>;
            },
          },
          {
            title: '更新时间',
            dataIndex: 'updated_at',
            width: 180,
          },
          {
            title: '操作',
            key: 'op',
            width: 320,
            render: (_: unknown, r: Script) => (
              <Space size={4} wrap>
                <Tooltip title="查看该剧本的分集列表">
                  <Button size="small" icon={<EyeOutlined />} onClick={() => openEpisodes(r)}>
                    分集
                  </Button>
                </Tooltip>
                <Tooltip title="调用 LLM 把整本剧本拆解成若干集">
                  <Button
                    size="small"
                    type="primary"
                    icon={<ScissorOutlined />}
                    onClick={() => openSplit(r)}
                  >
                    AI 拆分
                  </Button>
                </Tooltip>
                <Popconfirm title="确认删除剧本及其分集?" onConfirm={() => onDelete(r.id)}>
                  <Tooltip title="删除剧本及全部分集(不可恢复)">
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

      {/* 新建剧本 */}
      <Modal
        open={createOpen}
        title="新建剧本"
        onCancel={() => setCreateOpen(false)}
        onOk={onCreate}
        width={680}
        destroyOnClose
      >
        <Form form={createForm} layout="vertical">
          <Form.Item name="project_id" label="所属项目" rules={[{ required: true }]}>
            <Select
              showSearch
              optionFilterProp="label"
              options={projects.map((p) => ({ label: p.name, value: p.id }))}
            />
          </Form.Item>
          <Form.Item name="name" label="名称" rules={[{ required: true, max: 128 }]}>
            <Input />
          </Form.Item>
          <Form.Item name="source_url" label="来源链接(可选)">
            <Input placeholder="https://..." />
          </Form.Item>
          <Form.Item name="raw_text" label="剧本原文" rules={[{ required: true, min: 100 }]}>
            <Input.TextArea rows={12} placeholder="粘贴完整剧本(>= 100 字)" />
          </Form.Item>
        </Form>
      </Modal>

      {/* AI 拆分 */}
      <Modal
        open={splitOpen}
        title={`AI 拆分: ${splitTarget?.name ?? ''}`}
        onCancel={closeSplit}
        footer={
          splitTopic ? (
            <Button onClick={closeSplit}>关闭</Button>
          ) : (
            <Space>
              <Button onClick={closeSplit}>取消</Button>
              <Button type="primary" icon={<ThunderboltOutlined />} onClick={onSplitSubmit}>
                开始拆分
              </Button>
            </Space>
          )
        }
        width={560}
        destroyOnClose
      >
        {splitTopic ? (
          <div>
            <Progress percent={splitPercent} status={
              splitProgress.last?.type === 'error' ? 'exception'
              : splitProgress.last?.type === 'done' ? 'success' : 'active'
            } />
            {splitProgress.last?.type === 'error' && (
              <Alert
                type="error"
                showIcon
                style={{ marginTop: 12 }}
                message="拆分失败"
                description={splitProgress.last?.message || '请检查模型配置或重试'}
              />
            )}
            <Typography.Paragraph type="secondary" style={{ marginTop: 12 }}>
              {splitProgress.connected ? '已连接进度通道' : '正在连接...'}
            </Typography.Paragraph>
            <Typography.Paragraph>
              <Typography.Text strong>最新消息: </Typography.Text>
              {splitProgress.last?.message || '等待 worker 响应...'}
            </Typography.Paragraph>
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              topic: {splitTopic}
            </Typography.Text>
          </div>
        ) : (
          <Form form={splitForm} layout="vertical">
            <Form.Item name="model_id" label="文本模型" rules={[{ required: true }]}>
              <Select
                options={models.map((m) => ({
                  label: `${m.name} (${m.code})`,
                  value: m.id,
                }))}
                placeholder="选择启用中的 LLM"
              />
            </Form.Item>
            <Space wrap>
              <Form.Item name="episode_count" label="集数" style={{ width: 200 }}>
                <InputNumber min={1} max={100} style={{ width: '100%' }} />
              </Form.Item>
              <Form.Item name="target_chars" label="每集字数" style={{ width: 200 }}>
                <InputNumber min={200} max={5000} step={100} style={{ width: '100%' }} />
              </Form.Item>
            </Space>
          </Form>
        )}
      </Modal>

      {/* 分集抽屉 */}
      <Drawer
        open={episodeDrawerOpen}
        onClose={() => setEpisodeDrawerOpen(false)}
        width={720}
        title={`分集: ${episodeTarget?.name ?? ''}`}
        extra={
          <Tooltip title="重新拉取分集列表">
            <Button size="small" onClick={() => episodeTarget && openEpisodes(episodeTarget)}>
              刷新
            </Button>
          </Tooltip>
        }
      >
        <Table
          rowKey="id"
          loading={episodeLoading}
          dataSource={episodes}
          pagination={false}
          size="small"
          columns={[
            { title: '#', dataIndex: 'ep_no', width: 56 },
            { title: '标题', dataIndex: 'title' },
            {
              title: '摘要',
              dataIndex: 'summary',
              ellipsis: true,
              render: (s: string) => <Tooltip title={s}>{s}</Tooltip>,
            },
            {
              title: '操作',
              key: 'op',
              width: 120,
              render: (_: unknown, ep: Episode) => (
                <Tooltip title="跳转到提示词页面,基于本集生成">
                  <Button
                    size="small"
                    type="link"
                    onClick={() => navigate(`/prompts?episode_id=${ep.id}`)}
                  >
                    生成提示词
                  </Button>
                </Tooltip>
              ),
            },
          ]}
        />
      </Drawer>
    </Card>
  );
}
