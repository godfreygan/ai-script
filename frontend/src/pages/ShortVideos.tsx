import { useEffect, useMemo, useState } from 'react';
import {
  App as AntApp,
  Button,
  Card,
  Col,
  Empty,
  Form,
  Input,
  InputNumber,
  Modal,
  Pagination,
  Popconfirm,
  Progress,
  Row,
  Select,
  Space,
  Tag,
  Tooltip,
  Typography,
} from 'antd';
import {
  DeleteOutlined,
  ThunderboltOutlined,
  VideoCameraOutlined,
} from '@ant-design/icons';
import {
  ImageItem,
  Model,
  Project,
  ShortVideoItem,
  imageApi,
  modelApi,
  projectApi,
  shortVideoApi,
} from '@/api/modules';
import { useProgressWS } from '@/hooks/useProgressWS';

const statusColor: Record<string, string> = {
  pending: 'processing',
  running: 'processing',
  succeeded: 'success',
  failed: 'error',
};
const statusLabel: Record<string, string> = {
  pending: '等待中',
  running: '生成中',
  succeeded: '已完成',
  failed: '失败',
};

export default function ShortVideosPage() {
  const { message } = AntApp.useApp();

  const [projects, setProjects] = useState<Project[]>([]);
  const [models, setModels] = useState<Model[]>([]);
  const [candidateImages, setCandidateImages] = useState<ImageItem[]>([]);

  const [projectId, setProjectId] = useState<number | undefined>();
  const [storyboardId, setStoryboardId] = useState<number | undefined>();
  const [statusFilter, setStatusFilter] = useState<string | undefined>();

  const [list, setList] = useState<ShortVideoItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [total, setTotal] = useState(0);

  const [genOpen, setGenOpen] = useState(false);
  const [genForm] = Form.useForm();
  const [genTopic, setGenTopic] = useState<string | null>(null);

  const genProgress = useProgressWS(genTopic);
  const genPercent = useMemo(() => {
    if (!genProgress.last) return 0;
    return Math.round((genProgress.last.percent ?? 0) * 100);
  }, [genProgress.last]);

  const fetchRefs = async () => {
    try {
      const [p, m] = await Promise.all([
        projectApi.list({ page_size: 200 }),
        modelApi.list({ page_size: 200, type: 'video', enabled: 1 }),
      ]);
      setProjects(p.list);
      setModels(m.list);
    } catch {
      /* ignore */
    }
  };

  const fetchCandidates = async (sbid?: number, pid?: number) => {
    if (!sbid && !pid) {
      setCandidateImages([]);
      return;
    }
    try {
      const data = await imageApi.list({
        page_size: 100,
        storyboard_id: sbid,
        project_id: pid,
        status: 2,
      });
      setCandidateImages(data.list);
    } catch {
      setCandidateImages([]);
    }
  };

  const fetchList = async () => {
    setLoading(true);
    try {
      const data = await shortVideoApi.list({
        page,
        page_size: pageSize,
        project_id: projectId,
        storyboard_id: storyboardId,
        status: statusFilter,
      });
      setList(data.list);
      setTotal(data.total);
    } catch {
      setList([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchRefs();
  }, []);

  useEffect(() => {
    fetchList();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, pageSize, projectId, storyboardId, statusFilter]);

  useEffect(() => {
    if (!genProgress.last) return;
    const t = genProgress.last.type;
    if (t === 'done') {
      message.success(genProgress.last.message || '短视频生成完成');
      fetchList();
    } else if (t === 'error') {
      message.error(genProgress.last.message || '短视频生成失败');
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [genProgress.last?.type, genProgress.last?.time]);

  const onOpenGen = () => {
    genForm.resetFields();
    genForm.setFieldsValue({
      project_id: projectId,
      storyboard_id: storyboardId,
    });
    fetchCandidates(storyboardId, projectId);
    setGenTopic(null);
    setGenOpen(true);
  };

  const onGenerate = async () => {
    const v = await genForm.validateFields();
    try {
      const r = await shortVideoApi.generate({
        model_id: v.model_id,
        prompt: v.prompt,
        project_id: v.project_id,
        storyboard_id: v.storyboard_id,
        source_image_ids: v.source_image_ids,
      });
      setGenTopic(r.topic);
      message.success(`已入队任务 ${r.task_id}`);
    } catch {
      /* ignore */
    }
  };

  const onDelete = async (id: number) => {
    try {
      await shortVideoApi.delete(id);
      message.success('已删除');
      fetchList();
    } catch {
      /* ignore */
    }
  };

  return (
    <Space direction="vertical" style={{ width: '100%' }} size="large">
      <Card>
        <Space wrap>
          <Select
            allowClear
            placeholder="项目"
            style={{ width: 200 }}
            value={projectId}
            options={projects.map((p) => ({ label: p.name, value: p.id }))}
            onChange={(v) => {
              setProjectId(v);
              setPage(1);
            }}
          />
          <InputNumber
            placeholder="分镜 ID"
            style={{ width: 140 }}
            value={storyboardId}
            onChange={(v) => {
              setStoryboardId(v ?? undefined);
              setPage(1);
            }}
          />
          <Select
            allowClear
            placeholder="状态"
            style={{ width: 140 }}
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
          <Tooltip title="基于提示词调用视频模型生成短视频(可选关联分镜/参考图)">
            <Button type="primary" icon={<ThunderboltOutlined />} onClick={onOpenGen}>
              生成短视频
            </Button>
          </Tooltip>
        </Space>
      </Card>

      <Card
        title={
          <Space>
            <VideoCameraOutlined />
            短视频库
            {total > 0 && <Tag color="blue">{total} 条</Tag>}
          </Space>
        }
        size="small"
        loading={loading}
      >
        {list.length === 0 ? (
          <Empty description="暂无短视频">
            <Button type="primary" icon={<ThunderboltOutlined />} onClick={onOpenGen}>
              生成短视频
            </Button>
          </Empty>
        ) : (
          <>
            <Row gutter={[12, 12]}>
              {list.map((sv) => (
                <Col key={sv.id} xs={24} sm={12} md={8} lg={6}>
                  <Card
                    size="small"
                    cover={
                      sv.video_url ? (
                        <video
                          src={sv.video_url}
                          poster={sv.thumb_url || undefined}
                          controls
                          style={{ width: '100%', height: 220, background: '#000' }}
                        />
                      ) : (
                        <div
                          style={{
                            height: 220,
                            background: '#f5f5f5',
                            display: 'flex',
                            alignItems: 'center',
                            justifyContent: 'center',
                          }}
                        >
                          <VideoCameraOutlined style={{ fontSize: 48, color: '#bfbfbf' }} />
                        </div>
                      )
                    }
                    actions={[
                      <Popconfirm
                        key="del"
                        title="确认删除?"
                        onConfirm={() => onDelete(sv.id)}
                      >
                        <Tooltip title="删除该短视频(不可恢复)">
                          <DeleteOutlined />
                        </Tooltip>
                      </Popconfirm>,
                    ]}
                  >
                    <Card.Meta
                      title={
                        <Space size={4}>
                          <span>#{sv.id}</span>
                          <Tag color={statusColor[sv.status] || 'default'}>
                            {statusLabel[sv.status] || sv.status}
                          </Tag>
                          {sv.duration_ms > 0 && (
                            <Tag>{(sv.duration_ms / 1000).toFixed(1)}s</Tag>
                          )}
                        </Space>
                      }
                      description={
                        <Typography.Paragraph
                          ellipsis={{ rows: 2 }}
                          style={{ fontSize: 12, marginBottom: 0 }}
                        >
                          {sv.prompt || sv.error_msg || '-'}
                        </Typography.Paragraph>
                      }
                    />
                  </Card>
                </Col>
              ))}
            </Row>
            <div style={{ marginTop: 16, textAlign: 'right' }}>
              <Pagination
                current={page}
                pageSize={pageSize}
                total={total}
                showSizeChanger
                onChange={(p, ps) => {
                  setPage(p);
                  setPageSize(ps);
                }}
              />
            </div>
          </>
        )}
      </Card>

      <Modal
        open={genOpen}
        title="生成短视频"
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
        width={680}
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
            <Form.Item name="model_id" label="视频模型" rules={[{ required: true }]}>
              <Select
                options={models.map((m) => ({
                  label: `${m.name} (${m.code})`,
                  value: m.id,
                }))}
                placeholder="选择启用中的视频模型"
              />
            </Form.Item>
            <Row gutter={12}>
              <Col span={12}>
                <Form.Item name="project_id" label="项目">
                  <Select
                    allowClear
                    showSearch
                    optionFilterProp="label"
                    options={projects.map((p) => ({ label: p.name, value: p.id }))}
                  />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item name="storyboard_id" label="关联分镜 ID">
                  <InputNumber
                    min={1}
                    style={{ width: '100%' }}
                    onChange={(v) =>
                      fetchCandidates(v as number, genForm.getFieldValue('project_id'))
                    }
                  />
                </Form.Item>
              </Col>
            </Row>
            <Form.Item
              name="source_image_ids"
              label="参考图片(可选,图生视频)"
              extra={
                candidateImages.length > 0
                  ? `检测到 ${candidateImages.length} 张候选图片`
                  : '可手动输入图片 ID 数组,或先在「图片」页面生成'
              }
            >
              <Select
                mode="multiple"
                placeholder="选择已生成的图片"
                options={candidateImages.map((img) => ({
                  label: `#${img.id} - ${img.prompt?.slice(0, 30) || '-'}`,
                  value: img.id,
                }))}
              />
            </Form.Item>
            <Form.Item name="prompt" label="视频提示词">
              <Input.TextArea rows={3} placeholder="描述视频内容、运镜、节奏..." />
            </Form.Item>
          </Form>
        )}
      </Modal>
    </Space>
  );
}
