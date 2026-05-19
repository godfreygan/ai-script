import { useEffect, useMemo, useState } from 'react';
import {
  App as AntApp,
  Button,
  Card,
  Col,
  ColorPicker,
  Drawer,
  Empty,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Row,
  Select,
  Slider,
  Space,
  Statistic,
  Table,
  Tag,
  Tooltip,
  Typography,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import {
  CloudUploadOutlined,
  DownloadOutlined,
  EditOutlined,
  PlayCircleOutlined,
  ReloadOutlined,
  ShareAltOutlined,
} from '@ant-design/icons';
import {
  fullVideoApi,
  publishApi,
  type FullVideo,
  type PublishItem,
} from '@/api/modules';

const { Text, Paragraph } = Typography;

const DEFAULT_WATERMARK_TEXT = '@AI短剧';
const DEFAULT_WATERMARK = {
  text: DEFAULT_WATERMARK_TEXT,
  position: 'bottom-right',
  opacity: 0.6,
};

const POSITION_OPTIONS = [
  { label: '左上', value: 'top-left' },
  { label: '右上', value: 'top-right' },
  { label: '左下', value: 'bottom-left' },
  { label: '右下', value: 'bottom-right' },
  { label: '居中', value: 'center' },
];

const STATUS_OPTIONS = [
  { label: '已发布', value: 'on' },
  { label: '已下架', value: 'off' },
];

interface WatermarkConfig {
  text?: string;
  position?: string;
  opacity?: number;
  font_size?: number;
  color?: string;
  [k: string]: unknown;
}

function parseWatermark(raw: PublishItem['watermark_config']): WatermarkConfig {
  if (!raw) return {};
  if (typeof raw === 'string') {
    try {
      const v = JSON.parse(raw);
      return v && typeof v === 'object' ? (v as WatermarkConfig) : {};
    } catch {
      return {};
    }
  }
  if (typeof raw === 'object') return raw as WatermarkConfig;
  return {};
}

function watermarkToText(raw: PublishItem['watermark_config']): string {
  const v = parseWatermark(raw);
  if (!v || !v.text) return '-';
  return String(v.text);
}

interface WatermarkFormValues {
  text: string;
  position: string;
  opacity: number;
  font_size?: number;
  color?: string;
}

export default function PublishPage() {
  const { message } = AntApp.useApp();

  // --- 列表 / 筛选 ---
  const [list, setList] = useState<PublishItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [statusFilter, setStatusFilter] = useState<string | undefined>();

  // --- 全量统计(简单本地累加) ---
  const totalPlay = useMemo(
    () => list.reduce((s, p) => s + (p.play_count || 0), 0),
    [list],
  );
  const monthCount = useMemo(() => {
    const now = new Date();
    const yy = now.getFullYear();
    const mm = now.getMonth();
    return list.filter((p) => {
      if (!p.published_at) return false;
      const d = new Date(p.published_at);
      return d.getFullYear() === yy && d.getMonth() === mm;
    }).length;
  }, [list]);

  // --- 发布 Modal ---
  const [publishOpen, setPublishOpen] = useState(false);
  const [publishForm] = Form.useForm<{
    full_video_id: number;
    watermark_json: string;
  }>();
  const [fullVideoOptions, setFullVideoOptions] = useState<FullVideo[]>([]);
  const [fvLoading, setFvLoading] = useState(false);

  // --- 水印编辑 Modal ---
  const [wmOpen, setWmOpen] = useState(false);
  const [wmTarget, setWmTarget] = useState<PublishItem | null>(null);
  const [wmForm] = Form.useForm<WatermarkFormValues>();

  // --- 预览 Drawer ---
  const [previewOpen, setPreviewOpen] = useState(false);
  const [previewItem, setPreviewItem] = useState<PublishItem | null>(null);
  const [previewVideo, setPreviewVideo] = useState<FullVideo | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);

  const fetchList = async () => {
    setLoading(true);
    try {
      const data = await publishApi.list({
        page,
        page_size: pageSize,
        status: statusFilter,
      });
      // 防护: data 为 undefined 时避免炸
      setList(data?.list ?? []);
      setTotal(data?.total ?? 0);
    } catch (err) {
      message.error((err as Error)?.message || '加载发布列表失败');
      setList([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  };

  const fetchFullVideos = async () => {
    setFvLoading(true);
    try {
      const data = await fullVideoApi.list({
        page: 1,
        page_size: 200,
        status: 'succeeded',
      });
      // 防护: data 为 undefined 时避免炸
      setFullVideoOptions(data?.list ?? []);
    } catch (err) {
      message.error((err as Error)?.message || '加载完整视频列表失败');
    } finally {
      setFvLoading(false);
    }
  };

  useEffect(() => {
    fetchList();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, pageSize, statusFilter]);

  // ---------------- 发布新视频 ----------------
  const openPublish = () => {
    publishForm.resetFields();
    publishForm.setFieldsValue({
      watermark_json: JSON.stringify(DEFAULT_WATERMARK, null, 2),
    });
    fetchFullVideos();
    setPublishOpen(true);
  };

  const onSubmitPublish = async () => {
    const v = await publishForm.validateFields();
    let wm: Record<string, unknown> | undefined;
    if (v.watermark_json && v.watermark_json.trim()) {
      try {
        const parsed = JSON.parse(v.watermark_json);
        if (parsed && typeof parsed === 'object') {
          wm = parsed as Record<string, unknown>;
        }
      } catch {
        message.error('水印 JSON 解析失败,请检查格式');
        return;
      }
    }
    try {
      await publishApi.publish({
        full_video_id: v.full_video_id,
        watermark_config: wm,
      });
      message.success('已发布');
      setPublishOpen(false);
      fetchList();
    } catch (err) {
      message.error((err as Error)?.message || '发布失败');
    }
  };

  // ---------------- 下架 / 重新发布 ----------------
  const onUnpublish = async (item: PublishItem) => {
    try {
      await publishApi.unpublish(item.full_video_id);
      message.success('已下架');
      fetchList();
    } catch (err) {
      message.error((err as Error)?.message || '下架失败');
    }
  };

  const onRepublish = async (item: PublishItem) => {
    try {
      const wm = parseWatermark(item.watermark_config);
      await publishApi.publish({
        full_video_id: item.full_video_id,
        watermark_config: Object.keys(wm).length ? wm : undefined,
      });
      message.success('已重新发布');
      fetchList();
    } catch (err) {
      message.error((err as Error)?.message || '重新发布失败');
    }
  };

  // ---------------- 水印编辑 ----------------
  const openWatermark = (item: PublishItem) => {
    setWmTarget(item);
    const wm = parseWatermark(item.watermark_config);
    wmForm.setFieldsValue({
      text: typeof wm.text === 'string' ? wm.text : DEFAULT_WATERMARK_TEXT,
      position: typeof wm.position === 'string' ? wm.position : 'bottom-right',
      opacity: typeof wm.opacity === 'number' ? wm.opacity : 0.6,
      font_size: typeof wm.font_size === 'number' ? wm.font_size : 24,
      color: typeof wm.color === 'string' ? wm.color : '#FFFFFF',
    });
    setWmOpen(true);
  };

  const onSubmitWatermark = async () => {
    if (!wmTarget) return;
    const v = await wmForm.validateFields();
    const payload: Record<string, unknown> = {
      text: v.text,
      position: v.position,
      opacity: v.opacity,
      font_size: v.font_size,
      color: v.color,
    };
    try {
      await publishApi.updateWatermark(wmTarget.full_video_id, payload);
      message.success('水印已更新');
      setWmOpen(false);
      fetchList();
    } catch (err) {
      message.error((err as Error)?.message || '更新水印失败');
    }
  };

  // ---------------- 预览 ----------------
  const openPreview = async (item: PublishItem) => {
    setPreviewItem(item);
    setPreviewVideo(null);
    setPreviewOpen(true);
    setPreviewLoading(true);
    try {
      const fv = await fullVideoApi.get(item.full_video_id);
      setPreviewVideo(fv ?? null);
      // 预览即视为播放,计数 +1(失败忽略)
      publishApi.incPlay(item.full_video_id).catch(() => {
        /* ignore */
      });
    } catch (err) {
      message.error((err as Error)?.message || '加载视频预览失败');
    } finally {
      setPreviewLoading(false);
    }
  };

  const onMockDownload = async () => {
    if (!previewItem) return;
    try {
      await publishApi.incDownload(previewItem.full_video_id);
      message.success('已记录下载');
      fetchList();
    } catch (err) {
      message.error((err as Error)?.message || '记录下载失败');
    }
  };

  const onCopyShareLink = async () => {
    if (!previewItem) return;
    const url =
      previewVideo?.output_url ||
      `${window.location.origin}/publishes/${previewItem.full_video_id}`;
    try {
      await navigator.clipboard.writeText(url);
      message.success('分享链接已复制');
    } catch {
      message.warning('剪贴板不可用: ' + url);
    }
  };

  // ---------------- 表格列 ----------------
  const columns: ColumnsType<PublishItem> = [
    { title: 'ID', dataIndex: 'id', width: 70 },
    {
      title: 'Full Video',
      dataIndex: 'full_video_id',
      width: 130,
      render: (v: number, r: PublishItem) => (
        <Button
          type="link"
          style={{ padding: 0 }}
          onClick={() => openPreview(r)}
        >
          #{v}
        </Button>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (s: string) =>
        s === 'on' ? (
          <Tag color="green">已发布</Tag>
        ) : (
          <Tag>已下架</Tag>
        ),
    },
    {
      title: '播放',
      dataIndex: 'play_count',
      width: 90,
      render: (v: number) => v ?? 0,
    },
    {
      title: '下载',
      dataIndex: 'download_count',
      width: 90,
      render: (v: number) => v ?? 0,
    },
    {
      title: '水印',
      dataIndex: 'watermark_config',
      width: 160,
      ellipsis: true,
      render: (_: unknown, r: PublishItem) => (
        <Tooltip title={watermarkToText(r.watermark_config)}>
          <span>{watermarkToText(r.watermark_config)}</span>
        </Tooltip>
      ),
    },
    {
      title: '发布时间',
      dataIndex: 'published_at',
      width: 170,
    },
    {
      title: '操作',
      key: 'op',
      width: 280,
      render: (_: unknown, r: PublishItem) => (
        <Space size={4} wrap>
          <Tooltip title="在右侧抽屉播放视频(计入一次播放)">
            <Button
              size="small"
              icon={<PlayCircleOutlined />}
              onClick={() => openPreview(r)}
            >
              预览
            </Button>
          </Tooltip>
          <Tooltip title="编辑该视频的水印配置(文字/位置/透明度/颜色)">
            <Button
              size="small"
              icon={<EditOutlined />}
              onClick={() => openWatermark(r)}
            >
              水印
            </Button>
          </Tooltip>
          {r.status === 'on' ? (
            <Popconfirm
              title="确认下架该视频?"
              onConfirm={() => onUnpublish(r)}
            >
              <Tooltip title="下架后视频不再对外可见,但可重新发布">
                <Button size="small" danger>
                  下架
                </Button>
              </Tooltip>
            </Popconfirm>
          ) : (
            <Tooltip title="重新使用当前水印配置发布该视频">
              <Button
                size="small"
                type="primary"
                onClick={() => onRepublish(r)}
              >
                重新发布
              </Button>
            </Tooltip>
          )}
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col xs={24} sm={8}>
          <Card>
            <Statistic title="当前页发布数" value={list.length} suffix={`/ 共 ${total}`} />
          </Card>
        </Col>
        <Col xs={24} sm={8}>
          <Card>
            <Statistic title="本月新发布" value={monthCount} />
          </Card>
        </Col>
        <Col xs={24} sm={8}>
          <Card>
            <Statistic title="累计播放(当前页)" value={totalPlay} />
          </Card>
        </Col>
      </Row>

      <Card
        title="发布管理"
        extra={
          <Space>
            <Select
              allowClear
              placeholder="状态"
              style={{ width: 130 }}
              value={statusFilter}
              options={STATUS_OPTIONS}
              onChange={(v) => {
                setStatusFilter(v);
                setPage(1);
              }}
            />
            <Tooltip title="刷新列表">
              <Button icon={<ReloadOutlined />} onClick={fetchList}>
                刷新
              </Button>
            </Tooltip>
            <Tooltip title="选择一个已渲染成功的完整视频发布到对外渠道">
              <Button
                type="primary"
                icon={<CloudUploadOutlined />}
                onClick={openPublish}
              >
                发布新视频
              </Button>
            </Tooltip>
          </Space>
        }
      >
        <Table<PublishItem>
          rowKey="id"
          loading={loading}
          dataSource={list}
          columns={columns}
          locale={{
            emptyText: (
              <Empty description="暂无发布记录">
                <Button
                  type="primary"
                  icon={<CloudUploadOutlined />}
                  onClick={openPublish}
                >
                  发布新视频
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
        />
      </Card>

      {/* 发布新视频 Modal */}
      <Modal
        open={publishOpen}
        title="发布新视频"
        onCancel={() => setPublishOpen(false)}
        onOk={onSubmitPublish}
        okText="发布"
        cancelText="取消"
        destroyOnClose
        width={620}
      >
        <Form form={publishForm} layout="vertical">
          <Form.Item
            name="full_video_id"
            label="选择完整视频"
            rules={[{ required: true, message: '请选择完整视频' }]}
          >
            <Select
              showSearch
              loading={fvLoading}
              placeholder="只显示渲染成功的视频"
              optionFilterProp="label"
              options={fullVideoOptions.map((fv) => ({
                label: `#${fv.id} ${fv.name}`,
                value: fv.id,
              }))}
            />
          </Form.Item>
          <Form.Item
            name="watermark_json"
            label="水印配置 (JSON,可留空)"
            extra="示例: {&quot;text&quot;:&quot;@AI短剧&quot;,&quot;position&quot;:&quot;bottom-right&quot;,&quot;opacity&quot;:0.6}"
          >
            <Input.TextArea rows={5} />
          </Form.Item>
        </Form>
      </Modal>

      {/* 水印编辑 Modal */}
      <Modal
        open={wmOpen}
        title={`编辑水印 (Video #${wmTarget?.full_video_id ?? ''})`}
        onCancel={() => setWmOpen(false)}
        onOk={onSubmitWatermark}
        okText="保存"
        cancelText="取消"
        destroyOnClose
        width={560}
      >
        <Form form={wmForm} layout="vertical">
          <Form.Item
            name="text"
            label="水印文字"
            rules={[{ required: true, message: '请输入水印文字' }]}
          >
            <Input placeholder="例如 @AI短剧" maxLength={64} />
          </Form.Item>
          <Form.Item
            name="position"
            label="位置"
            rules={[{ required: true }]}
          >
            <Select options={POSITION_OPTIONS} />
          </Form.Item>
          <Form.Item
            name="opacity"
            label="透明度"
            rules={[{ required: true }]}
          >
            <Slider min={0} max={1} step={0.1} />
          </Form.Item>
          <Form.Item name="font_size" label="字号">
            <InputNumber min={8} max={128} step={1} style={{ width: 160 }} />
          </Form.Item>
          <Form.Item
            name="color"
            label="颜色"
            getValueFromEvent={(c: unknown) => {
              if (typeof c === 'string') return c;
              if (c && typeof c === 'object' && 'toHexString' in c) {
                return (c as { toHexString: () => string }).toHexString();
              }
              return c;
            }}
          >
            <ColorPicker showText format="hex" />
          </Form.Item>
        </Form>
      </Modal>

      {/* 预览 Drawer */}
      <Drawer
        open={previewOpen}
        title={`预览发布 #${previewItem?.id ?? ''}`}
        width={640}
        onClose={() => setPreviewOpen(false)}
        destroyOnClose
        extra={
          <Space>
            <Tooltip title="记录一次下载次数(用于统计)">
              <Button
                icon={<DownloadOutlined />}
                onClick={onMockDownload}
                disabled={!previewItem}
              >
                模拟下载
              </Button>
            </Tooltip>
            <Tooltip title="复制视频外链或站内分享 URL 到剪贴板">
              <Button
                type="primary"
                icon={<ShareAltOutlined />}
                onClick={onCopyShareLink}
                disabled={!previewItem}
              >
                复制分享链接
              </Button>
            </Tooltip>
          </Space>
        }
      >
        {previewLoading ? (
          <Empty description="加载中..." />
        ) : previewVideo?.output_url ? (
          <Card size="small" title={`#${previewVideo.id} ${previewVideo.name}`} style={{ marginBottom: 12 }}>
            <video
              controls
              src={previewVideo.output_url}
              poster={previewVideo.cover_url || previewVideo.thumb_url}
              style={{ width: '100%', maxHeight: 360, background: '#000' }}
            />
          </Card>
        ) : (
          <Empty description="视频尚未生成可播放的 output_url" />
        )}

        {previewVideo && (
          <Card size="small" title="元数据" style={{ marginBottom: 12 }}>
            <Paragraph style={{ marginBottom: 4 }}>
              <Text type="secondary">时长: </Text>
              <Text>
                {previewVideo.duration_ms
                  ? (previewVideo.duration_ms / 1000).toFixed(1) + 's'
                  : '-'}
              </Text>
            </Paragraph>
            <Paragraph style={{ marginBottom: 4 }}>
              <Text type="secondary">版本: </Text>
              <Text>v{previewVideo.version}</Text>
            </Paragraph>
            <Paragraph style={{ marginBottom: 4 }}>
              <Text type="secondary">状态: </Text>
              <Tag>{previewVideo.status}</Tag>
            </Paragraph>
          </Card>
        )}

        {previewItem && (
          <Card size="small" title="发布信息">
            <Paragraph style={{ marginBottom: 4 }}>
              <Text type="secondary">发布状态: </Text>
              {previewItem.status === 'on' ? (
                <Tag color="green">已发布</Tag>
              ) : (
                <Tag>已下架</Tag>
              )}
            </Paragraph>
            <Paragraph style={{ marginBottom: 4 }}>
              <Text type="secondary">播放次数: </Text>
              <Text>{previewItem.play_count ?? 0}</Text>
            </Paragraph>
            <Paragraph style={{ marginBottom: 4 }}>
              <Text type="secondary">下载次数: </Text>
              <Text>{previewItem.download_count ?? 0}</Text>
            </Paragraph>
            <Paragraph style={{ marginBottom: 4 }}>
              <Text type="secondary">发布时间: </Text>
              <Text>{previewItem.published_at || '-'}</Text>
            </Paragraph>
            <Paragraph style={{ marginBottom: 0 }}>
              <Text type="secondary">水印: </Text>
              <Text code>
                {JSON.stringify(parseWatermark(previewItem.watermark_config))}
              </Text>
            </Paragraph>
          </Card>
        )}
      </Drawer>
    </div>
  );
}
