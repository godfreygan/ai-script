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
  Switch,
  Table,
  Tag,
  Tooltip,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import {
  DeleteOutlined,
  EditOutlined,
  PlayCircleOutlined,
  PlusOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import {
  fullVideoApi,
  modelApi,
  projectApi,
  type FullVideo,
  type Model,
  type Project,
  type Timeline,
  type TimelineClip,
} from '@/api/modules';
import { useProgressWS } from '@/hooks/useProgressWS';

const statusColor: Record<string, string> = {
  draft: 'default',
  queued: 'blue',
  running: 'processing',
  succeeded: 'success',
  failed: 'error',
};

const statusLabel: Record<string, string> = {
  draft: '草稿',
  queued: '排队中',
  running: '渲染中',
  succeeded: '已完成',
  failed: '失败',
};

interface ClipDraft extends TimelineClip {
  _k: number;
}

let _clipKey = 0;
const nextClipKey = () => ++_clipKey;

function toClipDrafts(clips: TimelineClip[] | undefined): ClipDraft[] {
  if (!clips || !clips.length) return [];
  return clips.map((c) => ({ ...c, _k: nextClipKey() }));
}

function parseTimeline(tl: FullVideo['timeline']): Timeline {
  if (!tl) return { clips: [] };
  if (typeof tl === 'string') {
    try {
      return JSON.parse(tl) as Timeline;
    } catch {
      return { clips: [] };
    }
  }
  return tl;
}

export default function FullVideosPage() {
  const { message } = AntApp.useApp();

  // --- 列表 / 筛选 ---
  const [list, setList] = useState<FullVideo[]>([]);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [projectId, setProjectId] = useState<number | undefined>();
  const [statusFilter, setStatusFilter] = useState<string | undefined>();

  // --- 参考数据 ---
  const [projects, setProjects] = useState<Project[]>([]);
  const [audioModels, setAudioModels] = useState<Model[]>([]);

  // --- 创建 modal ---
  const [createOpen, setCreateOpen] = useState(false);
  const [createForm] = Form.useForm<{
    project_id: number;
    name: string;
    background_audio_url?: string;
    tts_model_id?: number;
    burn_subtitles?: boolean;
  }>();
  const [createClips, setCreateClips] = useState<ClipDraft[]>([]);

  // --- 编辑 drawer ---
  const [editOpen, setEditOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<FullVideo | null>(null);
  const [editForm] = Form.useForm<{
    name: string;
    background_audio_url?: string;
    tts_model_id?: number;
    burn_subtitles?: boolean;
  }>();
  const [editClips, setEditClips] = useState<ClipDraft[]>([]);

  // --- 渲染进度 WS ---
  const [renderTopic, setRenderTopic] = useState<string | null>(null);
  const [renderingId, setRenderingId] = useState<number | null>(null);
  const { last: progressLast, connected: wsConnected } = useProgressWS(renderTopic);
  const progressPercent = useMemo(() => {
    if (typeof progressLast?.percent !== 'number') return 0;
    return Math.min(100, Math.max(0, Math.round(progressLast.percent * 100)));
  }, [progressLast]);

  const fetchList = async () => {
    setLoading(true);
    try {
      const data = await fullVideoApi.list({
        page,
        page_size: pageSize,
        project_id: projectId,
        status: statusFilter,
      });
      setList(data.list);
      setTotal(data.total);
    } catch {
      /* api 已 toast */
    } finally {
      setLoading(false);
    }
  };

  const fetchRefs = async () => {
    try {
      const [p, m] = await Promise.all([
        projectApi.list({ page_size: 200 }),
        modelApi.list({ page_size: 200, type: 'audio', enabled: 1 }),
      ]);
      setProjects(p.list);
      setAudioModels(m.list);
    } catch {
      /* ignore */
    }
  };

  useEffect(() => {
    fetchRefs();
  }, []);

  useEffect(() => {
    fetchList();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, pageSize, projectId, statusFilter]);

  // 监听渲染进度: done / error 时清理
  useEffect(() => {
    if (!progressLast) return;
    const t = progressLast.type;
    if (t === 'done') {
      message.success(progressLast.message || '渲染完成');
      setRenderTopic(null);
      setRenderingId(null);
      fetchList();
    } else if (t === 'error') {
      message.error(progressLast.message || '渲染失败');
      setRenderTopic(null);
      setRenderingId(null);
      fetchList();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [progressLast?.type, progressLast?.time]);

  // ---------------- 创建 ----------------
  const openCreate = () => {
    createForm.resetFields();
    setCreateClips([{ _k: nextClipKey() }]);
    setCreateOpen(true);
  };

  const onCreate = async () => {
    const v = await createForm.validateFields();
    const timeline: Timeline = {
      clips: createClips.map(({ _k, ...rest }) => {
        void _k;
        return rest;
      }),
      background_audio_url: v.background_audio_url || undefined,
      tts_model_id: v.tts_model_id || undefined,
      burn_subtitles: !!v.burn_subtitles,
    };
    if (!timeline.clips.length) {
      message.warning('至少添加 1 个时间线片段');
      return;
    }
    try {
      await fullVideoApi.create({
        project_id: v.project_id,
        name: v.name,
        timeline,
      });
      message.success('已创建');
      setCreateOpen(false);
      fetchList();
    } catch {
      /* ignore */
    }
  };

  // ---------------- 编辑 ----------------
  const openEdit = (fv: FullVideo) => {
    setEditTarget(fv);
    const tl = parseTimeline(fv.timeline);
    editForm.setFieldsValue({
      name: fv.name,
      background_audio_url: tl.background_audio_url,
      tts_model_id: tl.tts_model_id,
      burn_subtitles: !!tl.burn_subtitles,
    });
    setEditClips(toClipDrafts(tl.clips));
    setEditOpen(true);
  };

  const onSaveEdit = async () => {
    if (!editTarget) return;
    const v = await editForm.validateFields();
    const timeline: Timeline = {
      clips: editClips.map(({ _k, ...rest }) => {
        void _k;
        return rest;
      }),
      background_audio_url: v.background_audio_url || undefined,
      tts_model_id: v.tts_model_id || undefined,
      burn_subtitles: !!v.burn_subtitles,
    };
    try {
      await fullVideoApi.update(editTarget.id, { name: v.name, timeline });
      message.success('已保存');
      setEditOpen(false);
      fetchList();
    } catch {
      /* ignore */
    }
  };

  // ---------------- 渲染 ----------------
  const onRender = async (fv: FullVideo) => {
    try {
      const r = await fullVideoApi.render(fv.id);
      const topic = r.topic || `full:${fv.id}`;
      setRenderTopic(topic);
      setRenderingId(fv.id);
      message.info(`已加入渲染队列 (task=${r.task_id})`);
      fetchList();
    } catch {
      /* ignore */
    }
  };

  const onRenderFromDrawer = async () => {
    if (!editTarget) return;
    await onRender(editTarget);
  };

  // ---------------- 删除 ----------------
  const onDelete = async (id: number) => {
    try {
      await fullVideoApi.delete(id);
      message.success('已删除');
      fetchList();
    } catch {
      /* ignore */
    }
  };

  // ---------------- 时间线编辑器 ----------------
  const renderClipEditor = (
    clips: ClipDraft[],
    setClips: (next: ClipDraft[]) => void,
  ) => (
    <div>
      {clips.map((clip, idx) => (
        <Card
          key={clip._k}
          size="small"
          title={`片段 #${idx + 1}`}
          style={{ marginBottom: 8 }}
          extra={
            <Button
              size="small"
              danger
              icon={<DeleteOutlined />}
              onClick={() => setClips(clips.filter((_, i) => i !== idx))}
            >
              删除
            </Button>
          }
        >
          <Space wrap size={[12, 8]} style={{ width: '100%' }}>
            <Tooltip title="短视频表中已生成片段的 ID">
              <span>
                short_video_id:{' '}
                <InputNumber
                  value={clip.short_video_id}
                  min={1}
                  style={{ width: 120 }}
                  onChange={(v) => {
                    const next = [...clips];
                    next[idx] = { ...clip, short_video_id: v ?? undefined };
                    setClips(next);
                  }}
                />
              </span>
            </Tooltip>
            <Tooltip title="或直接给定外链 URL,优先于 short_video_id">
              <span>
                url:{' '}
                <Input
                  value={clip.url}
                  placeholder="https://..."
                  style={{ width: 260 }}
                  onChange={(e) => {
                    const next = [...clips];
                    next[idx] = { ...clip, url: e.target.value };
                    setClips(next);
                  }}
                />
              </span>
            </Tooltip>
            <span>
              duration_ms:{' '}
              <InputNumber
                value={clip.duration_ms}
                min={0}
                step={100}
                style={{ width: 120 }}
                onChange={(v) => {
                  const next = [...clips];
                  next[idx] = { ...clip, duration_ms: v ?? undefined };
                  setClips(next);
                }}
              />
            </span>
            <span>
              tts_text:{' '}
              <Input
                value={clip.tts_text}
                placeholder="配音文本(可选)"
                style={{ width: 240 }}
                onChange={(e) => {
                  const next = [...clips];
                  next[idx] = { ...clip, tts_text: e.target.value };
                  setClips(next);
                }}
              />
            </span>
            <span>
              speaker:{' '}
              <Input
                value={clip.speaker}
                placeholder="角色前缀"
                style={{ width: 120 }}
                onChange={(e) => {
                  const next = [...clips];
                  next[idx] = { ...clip, speaker: e.target.value };
                  setClips(next);
                }}
              />
            </span>
          </Space>
        </Card>
      ))}
      <Button
        block
        type="dashed"
        icon={<PlusOutlined />}
        onClick={() => setClips([...clips, { _k: nextClipKey() }])}
      >
        添加片段
      </Button>
    </div>
  );

  // ---------------- 表格列 ----------------
  const columns: ColumnsType<FullVideo> = [
    { title: 'ID', dataIndex: 'id', width: 70 },
    {
      title: '名称',
      dataIndex: 'name',
      render: (n: string, r: FullVideo) => (
        <Button type="link" style={{ padding: 0 }} onClick={() => openEdit(r)}>
          {n}
        </Button>
      ),
    },
    {
      title: '项目',
      dataIndex: 'project_id',
      width: 140,
      render: (v: number) => projects.find((p) => p.id === v)?.name || `#${v}`,
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (s: string) => (
        <Tag color={statusColor[s] || 'default'}>{statusLabel[s] || s}</Tag>
      ),
    },
    {
      title: '时长',
      dataIndex: 'duration_ms',
      width: 90,
      render: (v: number) => (v ? (v / 1000).toFixed(1) + 's' : '-'),
    },
    {
      title: '渲染进度',
      dataIndex: 'render_progress',
      width: 160,
      render: (v: number, r: FullVideo) => {
        const pct = renderingId === r.id ? progressPercent : v || 0;
        const stat =
          r.status === 'failed'
            ? 'exception'
            : r.status === 'succeeded'
              ? 'success'
              : pct > 0 && pct < 100
                ? 'active'
                : 'normal';
        return <Progress percent={pct} size="small" status={stat} />;
      },
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      width: 170,
    },
    {
      title: '操作',
      key: 'op',
      width: 240,
      render: (_: unknown, r: FullVideo) => (
        <Space size={4} wrap>
          <Tooltip title="入队渲染">
            <Button
              size="small"
              type="primary"
              icon={<PlayCircleOutlined />}
              loading={renderingId === r.id}
              disabled={r.status === 'running' || r.status === 'queued'}
              onClick={() => onRender(r)}
            >
              渲染
            </Button>
          </Tooltip>
          <Button
            size="small"
            icon={<EditOutlined />}
            onClick={() => openEdit(r)}
          >
            编辑
          </Button>
          <Popconfirm title="确认删除该完整视频?" onConfirm={() => onDelete(r.id)}>
            <Button size="small" danger icon={<DeleteOutlined />}>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <Card
      title="完整视频"
      extra={
        <Space>
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
            style={{ width: 130 }}
            value={statusFilter}
            options={Object.entries(statusLabel).map(([k, v]) => ({
              label: v,
              value: k,
            }))}
            onChange={(v) => {
              setStatusFilter(v);
              setPage(1);
            }}
          />
          <Button icon={<ReloadOutlined />} onClick={fetchList}>
            刷新
          </Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            新建
          </Button>
        </Space>
      }
    >
      {renderTopic && (
        <Alert
          type={progressLast?.type === 'error' ? 'error' : 'info'}
          showIcon
          style={{ marginBottom: 12 }}
          message={
            <Space size={12} wrap>
              <span>
                {wsConnected ? '已连接进度通道' : '正在连接...'} - topic:{' '}
                <code>{renderTopic}</code>
              </span>
              <Progress
                percent={progressPercent}
                size="small"
                style={{ width: 220 }}
                status={
                  progressLast?.type === 'error'
                    ? 'exception'
                    : progressLast?.type === 'done'
                      ? 'success'
                      : 'active'
                }
              />
              <span>{progressLast?.message || '等待 worker 响应...'}</span>
            </Space>
          }
        />
      )}

      <Table<FullVideo>
        rowKey="id"
        loading={loading}
        dataSource={list}
        columns={columns}
        locale={{
          emptyText: (
            <Empty
              image={Empty.PRESENTED_IMAGE_SIMPLE}
              description="暂无完整视频,点击右上角“新建”开始"
            />
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
      />

      {/* 新建 */}
      <Modal
        open={createOpen}
        title="新建完整视频"
        width={820}
        onCancel={() => setCreateOpen(false)}
        onOk={onCreate}
        okText="创建"
        cancelText="取消"
        destroyOnClose
      >
        <Form form={createForm} layout="vertical">
          <Form.Item
            name="project_id"
            label="所属项目"
            rules={[{ required: true, message: '请选择项目' }]}
          >
            <Select
              showSearch
              optionFilterProp="label"
              options={projects.map((p) => ({ label: p.name, value: p.id }))}
              placeholder="选择项目"
            />
          </Form.Item>
          <Form.Item
            name="name"
            label="名称"
            rules={[{ required: true, max: 128 }]}
          >
            <Input placeholder="给这个完整视频起个名" />
          </Form.Item>
          <Form.Item label="时间线">{renderClipEditor(createClips, setCreateClips)}</Form.Item>
          <Space wrap style={{ width: '100%' }} size={16}>
            <Form.Item name="background_audio_url" label="背景音乐 URL" style={{ minWidth: 320 }}>
              <Input placeholder="https://... (可选)" />
            </Form.Item>
            <Form.Item name="tts_model_id" label="TTS 模型" style={{ minWidth: 220 }}>
              <Select
                allowClear
                placeholder="(可选) 用于 tts_text"
                options={audioModels.map((m) => ({
                  label: `${m.name} (${m.code})`,
                  value: m.id,
                }))}
              />
            </Form.Item>
            <Form.Item name="burn_subtitles" label="烧字幕" valuePropName="checked">
              <Switch />
            </Form.Item>
          </Space>
        </Form>
      </Modal>

      {/* 编辑 Drawer */}
      <Drawer
        open={editOpen}
        title={`编辑: ${editTarget?.name ?? ''}`}
        width={860}
        onClose={() => setEditOpen(false)}
        destroyOnClose
        extra={
          <Space>
            <Button onClick={() => setEditOpen(false)}>取消</Button>
            <Button type="primary" onClick={onSaveEdit}>
              保存
            </Button>
            <Button
              type="primary"
              icon={<PlayCircleOutlined />}
              danger
              onClick={onRenderFromDrawer}
              disabled={
                editTarget?.status === 'running' || editTarget?.status === 'queued'
              }
            >
              渲染
            </Button>
          </Space>
        }
      >
        {editTarget?.output_url ? (
          <Card size="small" title="预览" style={{ marginBottom: 12 }}>
            <video
              controls
              src={editTarget.output_url}
              poster={editTarget.cover_url || editTarget.thumb_url}
              style={{ width: '100%', maxHeight: 360, background: '#000' }}
            />
          </Card>
        ) : (
          <Card size="small" title="预览" style={{ marginBottom: 12 }}>
            <Empty
              image={Empty.PRESENTED_IMAGE_SIMPLE}
              description={
                editTarget?.status === 'running' || editTarget?.status === 'queued'
                  ? '正在渲染,请稍候...'
                  : '尚未生成视频,点击右上角“渲染”按钮触发'
              }
            />
          </Card>
        )}
        {editTarget && (
          <div style={{ marginBottom: 12 }}>
            <Tag color={statusColor[editTarget.status] || 'default'}>
              {statusLabel[editTarget.status] || editTarget.status}
            </Tag>
            <span style={{ marginLeft: 8, color: '#888' }}>
              版本 v{editTarget.version} · 时长{' '}
              {editTarget.duration_ms
                ? (editTarget.duration_ms / 1000).toFixed(1) + 's'
                : '-'}
            </span>
            {editTarget.error_msg && (
              <Alert
                type="error"
                showIcon
                style={{ marginTop: 8 }}
                message={editTarget.error_msg}
              />
            )}
          </div>
        )}
        <Form form={editForm} layout="vertical">
          <Form.Item
            name="name"
            label="名称"
            rules={[{ required: true, max: 128 }]}
          >
            <Input />
          </Form.Item>
          <Form.Item label="时间线">{renderClipEditor(editClips, setEditClips)}</Form.Item>
          <Space wrap style={{ width: '100%' }} size={16}>
            <Form.Item name="background_audio_url" label="背景音乐 URL" style={{ minWidth: 320 }}>
              <Input placeholder="https://... (可选)" />
            </Form.Item>
            <Form.Item name="tts_model_id" label="TTS 模型" style={{ minWidth: 220 }}>
              <Select
                allowClear
                placeholder="(可选) 用于 tts_text"
                options={audioModels.map((m) => ({
                  label: `${m.name} (${m.code})`,
                  value: m.id,
                }))}
              />
            </Form.Item>
            <Form.Item name="burn_subtitles" label="烧字幕" valuePropName="checked">
              <Switch />
            </Form.Item>
          </Space>
        </Form>
      </Drawer>
    </Card>
  );
}
