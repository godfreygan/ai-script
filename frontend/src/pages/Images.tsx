import { useEffect, useMemo, useState } from 'react';
import {
  App as AntApp,
  Button,
  Card,
  Col,
  Empty,
  Form,
  Image,
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
import { DeleteOutlined, PictureOutlined, ThunderboltOutlined } from '@ant-design/icons';
import {
  ImageItem,
  Model,
  Project,
  Style,
  imageApi,
  modelApi,
  projectApi,
  styleApi,
} from '@/api/modules';
import { useProgressWS } from '@/hooks/useProgressWS';

const statusColor: Record<number, string> = {
  0: 'default',
  1: 'processing',
  2: 'success',
  3: 'error',
};
const statusLabel: Record<number, string> = {
  0: '草稿',
  1: '生成中',
  2: '已完成',
  3: '失败',
};

export default function ImagesPage() {
  const { message } = AntApp.useApp();

  const [projects, setProjects] = useState<Project[]>([]);
  const [models, setModels] = useState<Model[]>([]);
  const [styles, setStyles] = useState<Style[]>([]);

  const [projectId, setProjectId] = useState<number | undefined>();
  const [storyboardId, setStoryboardId] = useState<number | undefined>();

  const [list, setList] = useState<ImageItem[]>([]);
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
        modelApi.list({ page_size: 200, type: 'image', enabled: 1 }),
      ]);
      setProjects(p.list);
      setModels(m.list);
    } catch (err) {
      // eslint-disable-next-line no-console
      console.error('fetch image refs failed:', err);
    }
  };

  const fetchStyles = async (pid?: number) => {
    try {
      const data = await styleApi.list(pid);
      setStyles(data);
    } catch (err) {
      // eslint-disable-next-line no-console
      console.error('fetch styles failed:', err);
      setStyles([]);
    }
  };

  const fetchList = async () => {
    setLoading(true);
    try {
      const data = await imageApi.list({
        page,
        page_size: pageSize,
        project_id: projectId,
        storyboard_id: storyboardId,
      });
      setList(data.list);
      setTotal(data.total);
    } catch (err) {
      // eslint-disable-next-line no-console
      console.error('fetch images failed:', err);
      setList([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchRefs();
    fetchStyles();
  }, []);

  useEffect(() => {
    fetchList();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, pageSize, projectId, storyboardId]);

  useEffect(() => {
    fetchStyles(projectId);
  }, [projectId]);

  useEffect(() => {
    if (!genProgress.last) return;
    const t = genProgress.last.type;
    if (t === 'done') {
      message.success(genProgress.last.message || '图片生成完成');
      fetchList();
    } else if (t === 'error') {
      message.error(genProgress.last.message || '图片生成失败');
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [genProgress.last?.type, genProgress.last?.time]);

  const onGenerate = async () => {
    const v = await genForm.validateFields();
    try {
      const r = await imageApi.generate({
        model_id: v.model_id,
        prompt: v.prompt,
        neg_prompt: v.neg_prompt,
        project_id: v.project_id,
        storyboard_id: v.storyboard_id,
        style_id: v.style_id,
      });
      setGenTopic(r.topic);
      message.success(`已入队任务 ${r.task_id}`);
    } catch (err) {
      // eslint-disable-next-line no-console
      console.error('generate image failed:', err);
    }
  };

  const onDelete = async (id: number) => {
    try {
      await imageApi.delete(id);
      message.success('已删除');
      fetchList();
    } catch (err) {
      // eslint-disable-next-line no-console
      console.error('delete image failed:', err);
    }
  };

  const openGen = () => {
    genForm.resetFields();
    genForm.setFieldsValue({
      project_id: projectId,
      storyboard_id: storyboardId,
    });
    setGenTopic(null);
    setGenOpen(true);
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
          <Tooltip title="基于提示词调用图像模型批量生成图片(可选关联分镜/项目/风格)">
            <Button
              type="primary"
              icon={<ThunderboltOutlined />}
              onClick={openGen}
            >
              生成图片
            </Button>
          </Tooltip>
        </Space>
      </Card>

      <Card
        title={
          <Space>
            <PictureOutlined />
            图片库
            {total > 0 && <Tag color="blue">{total} 张</Tag>}
          </Space>
        }
        size="small"
        loading={loading}
      >
        {list.length === 0 ? (
          <Empty description="暂无图片">
            <Button type="primary" icon={<ThunderboltOutlined />} onClick={openGen}>
              生成图片
            </Button>
          </Empty>
        ) : (
          <>
            <Row gutter={[12, 12]}>
              {list.map((img) => (
                <Col key={img.id} xs={24} sm={12} md={8} lg={6} xl={4}>
                  <Card
                    size="small"
                    cover={
                      img.url ? (
                        <Image
                          src={img.url}
                          alt={`image-${img.id}`}
                          style={{ height: 200, objectFit: 'cover' }}
                          preview={{ src: img.url }}
                        />
                      ) : (
                        <div
                          style={{
                            height: 200,
                            background: '#f5f5f5',
                            display: 'flex',
                            alignItems: 'center',
                            justifyContent: 'center',
                          }}
                        >
                          <PictureOutlined style={{ fontSize: 48, color: '#bfbfbf' }} />
                        </div>
                      )
                    }
                    actions={[
                      <Popconfirm
                        key="del"
                        title="确认删除?"
                        onConfirm={() => onDelete(img.id)}
                      >
                        <Tooltip title="删除该图片(不可恢复)">
                          <DeleteOutlined />
                        </Tooltip>
                      </Popconfirm>,
                    ]}
                  >
                    <Card.Meta
                      title={
                        <Space size={4}>
                          <span>#{img.id}</span>
                          <Tag color={statusColor[img.status] || 'default'}>
                            {statusLabel[img.status] || img.status}
                          </Tag>
                        </Space>
                      }
                      description={
                        <Typography.Paragraph
                          ellipsis={{ rows: 2 }}
                          style={{ fontSize: 12, marginBottom: 0 }}
                        >
                          {img.prompt || '-'}
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
        title="生成图片"
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
        width={640}
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
            <Form.Item name="model_id" label="图像模型" rules={[{ required: true }]}>
              <Select
                options={models.map((m) => ({
                  label: `${m.name} (${m.code})`,
                  value: m.id,
                }))}
                placeholder="选择启用中的图像模型"
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
                  <InputNumber min={1} style={{ width: '100%' }} />
                </Form.Item>
              </Col>
            </Row>
            <Form.Item name="style_id" label="风格预设(可选)">
              <Select
                allowClear
                showSearch
                optionFilterProp="label"
                options={styles.map((s) => ({
                  label: `${s.name} (${s.art_style || '-'})`,
                  value: s.id,
                }))}
              />
            </Form.Item>
            <Form.Item name="prompt" label="正向提示词" rules={[{ required: true }]}>
              <Input.TextArea rows={4} placeholder="描述画面、人物、风格..." />
            </Form.Item>
            <Form.Item name="neg_prompt" label="反向提示词">
              <Input.TextArea rows={2} placeholder="不希望出现的内容,如 blurry, low quality" />
            </Form.Item>
          </Form>
        )}
      </Modal>
    </Space>
  );
}
