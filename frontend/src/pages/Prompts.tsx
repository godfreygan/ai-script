import { useEffect, useMemo, useState } from 'react';
import {
  Alert,
  App as AntApp,
  Button,
  Card,
  Col,
  Descriptions,
  Form,
  Modal,
  Progress,
  Row,
  Select,
  Space,
  Table,
  Tag,
  Tooltip,
  Typography,
} from 'antd';
import { ThunderboltOutlined } from '@ant-design/icons';
import { useSearchParams } from 'react-router-dom';
import {
  Episode,
  EpisodePrompt,
  Model,
  Script,
  modelApi,
  promptApi,
  scriptApi,
} from '@/api/modules';
import { useProgressWS } from '@/hooks/useProgressWS';

export default function PromptsPage() {
  const { message } = AntApp.useApp();
  const [search, setSearch] = useSearchParams();

  const [scripts, setScripts] = useState<Script[]>([]);
  const [models, setModels] = useState<Model[]>([]);

  const [scriptId, setScriptId] = useState<number | undefined>();
  const [episodes, setEpisodes] = useState<Episode[]>([]);
  const [episodeId, setEpisodeId] = useState<number | undefined>(
    Number(search.get('episode_id')) || undefined,
  );

  const [prompts, setPrompts] = useState<EpisodePrompt[]>([]);
  const [loading, setLoading] = useState(false);
  const [current, setCurrent] = useState<EpisodePrompt | null>(null);

  const [genOpen, setGenOpen] = useState(false);
  const [genForm] = Form.useForm();
  const [genTopic, setGenTopic] = useState<string | null>(null);

  const genProgress = useProgressWS(genTopic);
  const genPercent = useMemo(() => {
    if (!genProgress.last) return 0;
    return Math.round((genProgress.last.percent ?? 0) * 100);
  }, [genProgress.last]);

  const selectedEpisode = useMemo(
    () => episodes.find((e) => e.id === episodeId),
    [episodes, episodeId],
  );

  const fetchRefs = async () => {
    try {
      const [s, m] = await Promise.all([
        scriptApi.list({ page_size: 200 }),
        modelApi.list({ page_size: 200, type: 'text', enabled: 1 }),
      ]);
      setScripts(s.list);
      setModels(m.list);
    } catch (err) {
      // eslint-disable-next-line no-console
      console.error('fetch prompt refs failed:', err);
    }
  };

  const fetchEpisodesForScript = async (sid: number) => {
    try {
      const eps = await scriptApi.episodes(sid);
      setEpisodes(eps);
    } catch (err) {
      // eslint-disable-next-line no-console
      console.error('fetch episodes failed:', err);
      setEpisodes([]);
    }
  };

  const fetchPrompts = async (eid: number) => {
    setLoading(true);
    try {
      const [ps, cur] = await Promise.all([
        promptApi.listByEpisode(eid),
        promptApi.getCurrent(eid),
      ]);
      setPrompts(ps);
      setCurrent(cur ?? null);
    } catch (err) {
      // eslint-disable-next-line no-console
      console.error('fetch prompts failed:', err);
      setPrompts([]);
      setCurrent(null);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchRefs();
  }, []);

  // 从 URL 进来时,反向查找 script_id
  useEffect(() => {
    const urlEp = Number(search.get('episode_id'));
    if (urlEp > 0 && scripts.length > 0 && !scriptId) {
      // 加载所有 script 的分集太费,直接默认拉第一个能含该 episode 的 script
      (async () => {
        for (const sc of scripts) {
          const eps = await scriptApi.episodes(sc.id);
          if (eps.find((e) => e.id === urlEp)) {
            setScriptId(sc.id);
            setEpisodes(eps);
            setEpisodeId(urlEp);
            return;
          }
        }
      })();
    }
  }, [scripts]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (scriptId) fetchEpisodesForScript(scriptId);
    else setEpisodes([]);
  }, [scriptId]);

  useEffect(() => {
    if (episodeId) {
      fetchPrompts(episodeId);
      setSearch({ episode_id: String(episodeId) }, { replace: true });
    } else {
      setPrompts([]);
      setCurrent(null);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [episodeId]);

  useEffect(() => {
    if (!genProgress.last) return;
    const t = genProgress.last.type;
    if (t === 'done') {
      message.success(genProgress.last.message || '生成完成');
      if (episodeId) fetchPrompts(episodeId);
    } else if (t === 'error') {
      message.error(genProgress.last.message || '生成失败');
      if (episodeId) fetchPrompts(episodeId);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [genProgress.last?.type, genProgress.last?.time]);

  const closeGen = () => {
    setGenOpen(false);
    // 断开 WS 订阅 + 重置表单,避免 modal 关闭后仍占连接/残留旧 topic
    setGenTopic(null);
    genForm.resetFields();
  };

  const onGenerate = async () => {
    if (!episodeId) return;
    const v = await genForm.validateFields();
    try {
      const r = await promptApi.generate(episodeId, v);
      setGenTopic(r.topic);
      message.success(`已入队任务 ${r.task_id}`);
    } catch (err) {
      // eslint-disable-next-line no-console
      console.error('generate prompt failed:', err);
    }
  };

  const onSetCurrent = async (id: number) => {
    if (!episodeId) return;
    try {
      await promptApi.setCurrent(id, episodeId);
      message.success('已设为当前版本');
      fetchPrompts(episodeId);
    } catch (err) {
      // eslint-disable-next-line no-console
      console.error('set current prompt failed:', err);
    }
  };

  const prettyJSON = (raw: unknown): string => {
    try {
      if (typeof raw === 'string') return JSON.stringify(JSON.parse(raw), null, 2);
      return JSON.stringify(raw, null, 2);
    } catch {
      return String(raw);
    }
  };

  return (
    <Space direction="vertical" size="large" style={{ width: '100%' }}>
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
            onChange={(v) => {
              setScriptId(v);
              setEpisodeId(undefined);
            }}
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
          <Tooltip title={episodeId ? '调用大模型生成新一版分镜提示词' : '请先选择剧本与分集'}>
            <Button
              type="primary"
              icon={<ThunderboltOutlined />}
              disabled={!episodeId}
              onClick={() => {
                setGenTopic(null);
                genForm.resetFields();
                setGenOpen(true);
              }}
            >
              生成新提示词
            </Button>
          </Tooltip>
        </Space>
      </Card>

      {selectedEpisode && (
        <Card title={`第 ${selectedEpisode.ep_no} 集 · ${selectedEpisode.title}`} size="small">
          <Descriptions size="small" column={1}>
            <Descriptions.Item label="摘要">{selectedEpisode.summary}</Descriptions.Item>
            <Descriptions.Item label="原文片段">
              <Typography.Paragraph
                ellipsis={{ rows: 4, expandable: true, symbol: '展开' }}
                style={{ marginBottom: 0 }}
              >
                {selectedEpisode.raw_segment}
              </Typography.Paragraph>
            </Descriptions.Item>
          </Descriptions>
        </Card>
      )}

      <Row gutter={16}>
        <Col span={14}>
          <Card title="提示词历史" size="small">
            <Table
              rowKey="id"
              size="small"
              loading={loading}
              dataSource={prompts}
              pagination={false}
              locale={{
                emptyText: episodeId
                  ? '该分集暂无提示词,点击右上「生成新提示词」开始'
                  : '请先在上方选择剧本与分集',
              }}
              columns={[
                { title: 'ID', dataIndex: 'id', width: 80 },
                {
                  title: '模型',
                  dataIndex: 'model_id',
                  width: 200,
                  render: (id: number) => {
                    const m = models.find((x) => x.id === id);
                    return m ? `${m.name}` : `#${id}`;
                  },
                },
                {
                  title: '状态',
                  dataIndex: 'status',
                  width: 110,
                  render: (s: number, r: EpisodePrompt) => {
                    // 0 pending, 1 succeeded, 2 generating, 3 failed
                    const map: Record<number, { color: string; label: string }> = {
                      0: { color: 'default', label: '待生成' },
                      1: { color: 'success', label: '生成完成' },
                      2: { color: 'processing', label: '生成中' },
                      3: { color: 'error', label: '失败' },
                    };
                    const m = map[s] ?? { color: 'default', label: `#${s}` };
                    return (
                      <Space size={4}>
                        <Tag color={m.color}>{m.label}</Tag>
                        {current && r.id === current.id && <Tag color="success">当前</Tag>}
                      </Space>
                    );
                  },
                },
                {
                  title: '生成时间',
                  dataIndex: 'created_at',
                  width: 170,
                },
                {
                  title: '操作',
                  key: 'op',
                  width: 110,
                  render: (_: unknown, r: EpisodePrompt) =>
                    current?.id === r.id ? (
                      <Tag color="success">已锁定</Tag>
                    ) : (
                      <Tooltip title="将此版本锁定为当前生效的提示词">
                        <Button size="small" type="link" onClick={() => onSetCurrent(r.id)}>
                          设为当前
                        </Button>
                      </Tooltip>
                    ),
                },
              ]}
            />
          </Card>
        </Col>

        <Col span={10}>
          <Card title="当前提示词 JSON" size="small">
            {current ? (
              <pre
                style={{
                  background: '#fafafa',
                  padding: 12,
                  borderRadius: 4,
                  maxHeight: 480,
                  overflow: 'auto',
                  fontSize: 12,
                  margin: 0,
                }}
              >
                {prettyJSON(current.content)}
              </pre>
            ) : (
              <Typography.Text type="secondary">
                {episodeId ? '当前分集尚无提示词,请生成。' : '请先选择分集'}
              </Typography.Text>
            )}
          </Card>
        </Col>
      </Row>

      <Modal
        open={genOpen}
        title="AI 生成分镜提示词"
        onCancel={closeGen}
        footer={
          genTopic ? (
            <Button onClick={closeGen}>关闭</Button>
          ) : (
            <Space>
              <Button onClick={closeGen}>取消</Button>
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
            {genProgress.last?.type === 'error' && (
              <Alert
                type="error"
                showIcon
                style={{ marginTop: 12 }}
                message="生成失败"
                description={genProgress.last?.message || '请检查模型配置或重试'}
              />
            )}
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
              />
            </Form.Item>
          </Form>
        )}
      </Modal>
    </Space>
  );
}
