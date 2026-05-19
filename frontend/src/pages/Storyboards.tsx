import { useEffect, useMemo, useState } from 'react';
import {
  App as AntApp,
  Button,
  Card,
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
  AppstoreOutlined,
  BgColorsOutlined,
  DeleteOutlined,
  EditOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';
import {
  Episode,
  Model,
  Script,
  Storyboard,
  Style,
  modelApi,
  scriptApi,
  storyboardApi,
  styleApi,
} from '@/api/modules';
import { useProgressWS } from '@/hooks/useProgressWS';

function parseCharacters(v: Storyboard['characters']): string[] {
  if (!v) return [];
  if (Array.isArray(v)) return v;
  try {
    const arr = JSON.parse(v as string);
    return Array.isArray(arr) ? arr : [];
  } catch {
    return [];
  }
}

export default function StoryboardsPage() {
  const { message } = AntApp.useApp();

  const [scripts, setScripts] = useState<Script[]>([]);
  const [models, setModels] = useState<Model[]>([]);
  const [styles, setStyles] = useState<Style[]>([]);

  const [scriptId, setScriptId] = useState<number | undefined>();
  const [episodes, setEpisodes] = useState<Episode[]>([]);
  const [episodeId, setEpisodeId] = useState<number | undefined>();

  const [list, setList] = useState<Storyboard[]>([]);
  const [loading, setLoading] = useState(false);

  const [editOpen, setEditOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<Storyboard | null>(null);
  const [editForm] = Form.useForm();

  const [genOpen, setGenOpen] = useState(false);
  const [genForm] = Form.useForm();
  const [genTopic, setGenTopic] = useState<string | null>(null);

  const [styleOpen, setStyleOpen] = useState(false);
  const [styleTarget, setStyleTarget] = useState<Storyboard | null>(null);
  const [styleForm] = Form.useForm();

  const genProgress = useProgressWS(genTopic);
  const genPercent = useMemo(() => {
    if (!genProgress.last) return 0;
    return Math.round((genProgress.last.percent ?? 0) * 100);
  }, [genProgress.last]);

  const selectedScript = useMemo(
    () => scripts.find((s) => s.id === scriptId),
    [scripts, scriptId],
  );

  const fetchRefs = async () => {
    try {
      const [s, m] = await Promise.all([
        scriptApi.list({ page_size: 200 }),
        modelApi.list({ page_size: 200, type: 'text', enabled: 1 }),
      ]);
      // 防护: 接口返回 undefined 时避免炸
      setScripts(s?.list ?? []);
      setModels(m?.list ?? []);
    } catch (err) {
      message.error((err as Error)?.message || '加载参考数据失败');
    }
  };

  const fetchEpisodes = async (sid: number) => {
    try {
      const eps = await scriptApi.episodes(sid);
      // 防护: 接口返回 undefined 时避免炸
      setEpisodes(eps ?? []);
    } catch (err) {
      message.error((err as Error)?.message || '加载分集失败');
      setEpisodes([]);
    }
  };

  const fetchStyles = async (pid?: number) => {
    try {
      const data = await styleApi.list(pid);
      // 防护: 接口返回 undefined 时避免炸
      setStyles(data ?? []);
    } catch (err) {
      message.error((err as Error)?.message || '加载风格列表失败');
      setStyles([]);
    }
  };

  const fetchList = async (eid: number) => {
    setLoading(true);
    try {
      const data = await storyboardApi.listByEpisode(eid);
      // 防护: 接口返回 undefined 时避免炸
      setList(data ?? []);
    } catch (err) {
      message.error((err as Error)?.message || '加载分镜列表失败');
      setList([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchRefs();
  }, []);

  useEffect(() => {
    if (scriptId) {
      fetchEpisodes(scriptId);
      const sc = scripts.find((s) => s.id === scriptId);
      fetchStyles(sc?.project_id);
    } else {
      setEpisodes([]);
      setStyles([]);
    }
    setEpisodeId(undefined);
    setList([]);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [scriptId]);

  useEffect(() => {
    if (episodeId) fetchList(episodeId);
    else setList([]);
  }, [episodeId]);

  useEffect(() => {
    if (!genProgress.last) return;
    const t = genProgress.last.type;
    if (t === 'done') {
      message.success(genProgress.last.message || '分镜生成完成');
      if (episodeId) fetchList(episodeId);
    } else if (t === 'error') {
      message.error(genProgress.last.message || '分镜生成失败');
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [genProgress.last?.type, genProgress.last?.time]);

  const openEdit = (sb: Storyboard) => {
    setEditTarget(sb);
    editForm.setFieldsValue({
      shot_type: sb.shot_type,
      camera_motion: sb.camera_motion,
      scene_desc: sb.scene_desc,
      characters: parseCharacters(sb.characters),
      action: sb.action,
      dialogue: sb.dialogue,
      duration_sec: sb.duration_sec,
      notes: sb.notes,
    });
    setEditOpen(true);
  };

  const onSaveEdit = async () => {
    if (!editTarget) return;
    const v = await editForm.validateFields();
    try {
      await storyboardApi.update(editTarget.id, v);
      message.success('已保存');
      setEditOpen(false);
      if (episodeId) fetchList(episodeId);
    } catch (err) {
      message.error((err as Error)?.message || '保存失败');
    }
  };

  const onDelete = async (id: number) => {
    try {
      await storyboardApi.delete(id);
      message.success('已删除');
      if (episodeId) fetchList(episodeId);
    } catch (err) {
      message.error((err as Error)?.message || '删除失败');
    }
  };

  const onGenerate = async () => {
    if (!episodeId) return;
    const v = await genForm.validateFields();
    try {
      const r = await storyboardApi.generate(episodeId, v);
      setGenTopic(r?.topic ?? null);
      message.success(`已入队任务 ${r?.task_id}`);
    } catch (err) {
      message.error((err as Error)?.message || '生成失败');
    }
  };

  const openStyle = (sb: Storyboard) => {
    setStyleTarget(sb);
    styleForm.resetFields();
    setStyleOpen(true);
  };

  const onApplyStyle = async () => {
    if (!styleTarget) return;
    const v = await styleForm.validateFields();
    try {
      await storyboardApi.applyStyle(styleTarget.id, v.style_id);
      message.success('已应用风格');
      setStyleOpen(false);
      if (episodeId) fetchList(episodeId);
    } catch (err) {
      message.error((err as Error)?.message || '应用风格失败');
    }
  };

  return (
    <Space direction="vertical" style={{ width: '100%' }} size="large">
      <Card>
        <Space wrap>
          <Select
            showSearch
            allowClear
            placeholder="选择剧本"
            style={{ width: 280 }}
            value={scriptId}
            optionFilterProp="label"
            options={scripts.map((s) => ({ label: s.name, value: s.id }))}
            onChange={(v) => setScriptId(v)}
          />
          <Select
            showSearch
            allowClear
            placeholder="选择分集"
            style={{ width: 360 }}
            value={episodeId}
            optionFilterProp="label"
            options={episodes.map((e) => ({
              label: `${e.ep_no}. ${e.title}`,
              value: e.id,
            }))}
            onChange={(v) => setEpisodeId(v)}
            disabled={!scriptId}
          />
          <Tooltip title={episodeId ? '基于该分集当前提示词的 shots[] 段调用大模型批量生成分镜' : '请先选择剧本与分集'}>
            <Button
              type="primary"
              icon={<ThunderboltOutlined />}
              disabled={!episodeId}
              onClick={() => {
                genForm.resetFields();
                setGenTopic(null);
                setGenOpen(true);
              }}
            >
              AI 生成分镜
            </Button>
          </Tooltip>
          {selectedScript && (
            <Typography.Text type="secondary">
              项目: {selectedScript.project_id ? `#${selectedScript.project_id}` : '-'}
            </Typography.Text>
          )}
        </Space>
      </Card>

      <Card
        title={
          <Space>
            <AppstoreOutlined />
            分镜列表
            {list.length > 0 && <Tag color="blue">{list.length} 条</Tag>}
          </Space>
        }
        size="small"
      >
        <Table
          rowKey="id"
          size="small"
          loading={loading}
          dataSource={list}
          pagination={false}
          scroll={{ x: 1400 }}
          locale={{
            emptyText: episodeId ? (
              <Empty description="该分集暂无分镜">
                <Button
                  type="primary"
                  icon={<ThunderboltOutlined />}
                  onClick={() => {
                    genForm.resetFields();
                    setGenTopic(null);
                    setGenOpen(true);
                  }}
                >
                  AI 生成分镜
                </Button>
              </Empty>
            ) : (
              '请先在上方选择剧本与分集'
            ),
          }}
          columns={[
            { title: '#', dataIndex: 'shot_no', width: 56 },
            {
              title: '景别',
              dataIndex: 'shot_type',
              width: 90,
              render: (v: string) => <Tag>{v || '-'}</Tag>,
            },
            {
              title: '运镜',
              dataIndex: 'camera_motion',
              width: 90,
              render: (v: string) => <Tag color="cyan">{v || 'static'}</Tag>,
            },
            {
              title: '场景描述',
              dataIndex: 'scene_desc',
              ellipsis: true,
              render: (s: string) => <Tooltip title={s}>{s}</Tooltip>,
            },
            {
              title: '角色',
              dataIndex: 'characters',
              width: 160,
              render: (v: Storyboard['characters']) => {
                const arr = parseCharacters(v);
                return arr.length ? (
                  <Space size={2} wrap>
                    {arr.map((c) => (
                      <Tag key={c}>{c}</Tag>
                    ))}
                  </Space>
                ) : (
                  '-'
                );
              },
            },
            {
              title: '动作',
              dataIndex: 'action',
              ellipsis: true,
              render: (s: string) => <Tooltip title={s}>{s}</Tooltip>,
            },
            {
              title: '台词',
              dataIndex: 'dialogue',
              ellipsis: true,
              width: 160,
              render: (s: string) => <Tooltip title={s}>{s}</Tooltip>,
            },
            { title: '时长(秒)', dataIndex: 'duration_sec', width: 90 },
            {
              title: '操作',
              key: 'op',
              fixed: 'right',
              width: 220,
              render: (_: unknown, r: Storyboard) => (
                <Space size={4} wrap>
                  <Tooltip title="编辑该分镜的镜头/描述/台词">
                    <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(r)}>
                      编辑
                    </Button>
                  </Tooltip>
                  <Tooltip title="为此分镜叠加风格关键词到 image_prompt">
                    <Button
                      size="small"
                      icon={<BgColorsOutlined />}
                      onClick={() => openStyle(r)}
                    >
                      应用风格
                    </Button>
                  </Tooltip>
                  <Popconfirm title="确认删除该分镜?" onConfirm={() => onDelete(r.id)}>
                    <Tooltip title="删除该分镜(不可恢复)">
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
      </Card>

      <Modal
        open={editOpen}
        title={`编辑分镜 #${editTarget?.shot_no ?? ''}`}
        onCancel={() => setEditOpen(false)}
        onOk={onSaveEdit}
        width={720}
        destroyOnClose
      >
        <Form form={editForm} layout="vertical">
          <Space wrap style={{ width: '100%' }}>
            <Form.Item name="shot_type" label="景别" style={{ width: 200 }}>
              <Select
                options={['wide', 'medium', 'close', 'extreme_close'].map((v) => ({
                  label: v,
                  value: v,
                }))}
              />
            </Form.Item>
            <Form.Item name="camera_motion" label="运镜" style={{ width: 200 }}>
              <Select
                options={['static', 'push', 'pull', 'pan', 'tilt', 'tracking'].map((v) => ({
                  label: v,
                  value: v,
                }))}
              />
            </Form.Item>
            <Form.Item name="duration_sec" label="时长(秒)" style={{ width: 140 }}>
              <InputNumber min={1} max={120} style={{ width: '100%' }} />
            </Form.Item>
          </Space>
          <Form.Item name="scene_desc" label="场景描述">
            <Input.TextArea rows={3} />
          </Form.Item>
          <Form.Item name="characters" label="角色">
            <Select mode="tags" placeholder="按回车添加角色" />
          </Form.Item>
          <Form.Item name="action" label="动作">
            <Input.TextArea rows={2} />
          </Form.Item>
          <Form.Item name="dialogue" label="台词">
            <Input.TextArea rows={2} />
          </Form.Item>
          <Form.Item name="notes" label="备注">
            <Input.TextArea rows={2} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        open={genOpen}
        title="AI 生成分镜"
        onCancel={() => setGenOpen(false)}
        footer={
          genTopic ? (
            <Button onClick={() => setGenOpen(false)}>关闭</Button>
          ) : (
            <Space>
              <Button onClick={() => setGenOpen(false)}>取消</Button>
              <Button type="primary" icon={<ThunderboltOutlined />} onClick={onGenerate}>
                开始生成
              </Button>
            </Space>
          )
        }
        width={520}
        destroyOnClose
      >
        {genTopic ? (
          <div>
            <Progress
              percent={genPercent}
              status={
                genProgress.last?.type === 'error'
                  ? 'exception'
                  : genProgress.last?.type === 'done'
                  ? 'success'
                  : 'active'
              }
            />
            <Typography.Paragraph type="secondary" style={{ marginTop: 12 }}>
              {genProgress.connected ? '已连接进度通道' : '正在连接...'}
            </Typography.Paragraph>
            <Typography.Paragraph>
              <Typography.Text strong>最新消息: </Typography.Text>
              {genProgress.last?.message || '等待 worker 响应...'}
            </Typography.Paragraph>
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              topic: {genTopic}
            </Typography.Text>
          </div>
        ) : (
          <Form form={genForm} layout="vertical">
            <Form.Item name="model_id" label="文本模型" rules={[{ required: true }]}>
              <Select
                options={models.map((m) => ({
                  label: `${m.name} (${m.code})`,
                  value: m.id,
                }))}
                placeholder="选择启用中的 LLM"
              />
            </Form.Item>
            <Typography.Paragraph type="secondary" style={{ fontSize: 12, marginBottom: 0 }}>
              系统会读取该分集当前提示词中的 shots[] 段,覆盖式生成分镜。
            </Typography.Paragraph>
          </Form>
        )}
      </Modal>

      <Modal
        open={styleOpen}
        title={`应用风格 - 分镜 #${styleTarget?.shot_no ?? ''}`}
        onCancel={() => setStyleOpen(false)}
        onOk={onApplyStyle}
        width={480}
        destroyOnClose
      >
        <Form form={styleForm} layout="vertical">
          <Form.Item name="style_id" label="风格预设" rules={[{ required: true }]}>
            <Select
              showSearch
              optionFilterProp="label"
              options={styles.map((s) => ({
                label: `${s.name} (${s.art_style || '-'})`,
                value: s.id,
              }))}
              placeholder="选择该项目下的风格"
            />
          </Form.Item>
          <Typography.Paragraph type="secondary" style={{ fontSize: 12, marginBottom: 0 }}>
            应用后该分镜的 image_prompt 会自动追加风格关键词。
          </Typography.Paragraph>
        </Form>
      </Modal>
    </Space>
  );
}
