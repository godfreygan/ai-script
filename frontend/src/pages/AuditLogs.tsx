import { useEffect, useState } from 'react';
import {
  App as AntApp,
  Button,
  Card,
  Drawer,
  Empty,
  Form,
  InputNumber,
  Select,
  Space,
  Table,
  Tag,
  Tooltip,
  Typography,
} from 'antd';
import { CopyOutlined, FileSearchOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import { AuditEntry, auditApi } from '@/api/modules';

const resourceTypeOptions = [
  { label: '用户', value: 'user' },
  { label: '部门', value: 'dept' },
  { label: '角色', value: 'role' },
  { label: '项目', value: 'project' },
  { label: '剧本', value: 'script' },
  { label: '模型', value: 'model' },
  { label: '完整视频', value: 'full_video' },
  { label: '发布', value: 'publish' },
  { label: '审核', value: 'review' },
  { label: '灰度开关', value: 'feature_flag' },
  { label: '计费', value: 'billing' },
];

const actionOptions = [
  { label: '创建', value: 'create' },
  { label: '更新', value: 'update' },
  { label: '删除', value: 'delete' },
  { label: '登录', value: 'login' },
  { label: '登出', value: 'logout' },
  { label: '审核', value: 'review' },
  { label: '发布', value: 'publish' },
  { label: '执行', value: 'execute' },
];

const actionColor: Record<string, string> = {
  create: 'success',
  update: 'processing',
  delete: 'error',
  login: 'blue',
  logout: 'default',
  review: 'gold',
  publish: 'purple',
  execute: 'cyan',
};

function safeStringify(v: AuditEntry['before']): string {
  if (v === null || v === undefined || v === '') return '';
  if (typeof v === 'string') {
    try {
      const parsed = JSON.parse(v);
      return JSON.stringify(parsed, null, 2);
    } catch {
      return v;
    }
  }
  try {
    return JSON.stringify(v, null, 2);
  } catch {
    return String(v);
  }
}

export default function AuditLogsPage() {
  const { message } = AntApp.useApp();

  const [filters, setFilters] = useState<{
    user_id?: number;
    resource_type?: string;
    action?: string;
  }>({});

  const [list, setList] = useState<AuditEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [loading, setLoading] = useState(false);

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [detail, setDetail] = useState<AuditEntry | null>(null);

  const fetchList = async () => {
    setLoading(true);
    try {
      const data = await auditApi.list({
        page,
        page_size: pageSize,
        user_id: filters.user_id,
        resource_type: filters.resource_type,
        action: filters.action,
      });
      setList(data.list || []);
      setTotal(data.total || 0);
    } catch {
      setList([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchList();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, pageSize, filters]);

  const copyText = async (text: string) => {
    if (!text) return;
    try {
      await navigator.clipboard.writeText(text);
      message.success('已复制');
    } catch {
      message.error('复制失败');
    }
  };

  const openDetail = (row: AuditEntry) => {
    setDetail(row);
    setDrawerOpen(true);
  };

  return (
    <Space direction="vertical" style={{ width: '100%' }} size="large">
      <Card
        size="small"
        title={
          <Space>
            <FileSearchOutlined />
            审计日志查询
          </Space>
        }
      >
        <Form layout="inline">
          <Form.Item label="用户 ID">
            <InputNumber
              min={1}
              style={{ width: 140 }}
              placeholder="user_id"
              value={filters.user_id}
              onChange={(v) => {
                setFilters((f) => ({ ...f, user_id: (v as number | null) ?? undefined }));
                setPage(1);
              }}
            />
          </Form.Item>
          <Form.Item label="资源类型">
            <Select
              allowClear
              style={{ width: 180 }}
              placeholder="全部"
              value={filters.resource_type}
              options={resourceTypeOptions}
              onChange={(v) => {
                setFilters((f) => ({ ...f, resource_type: v }));
                setPage(1);
              }}
            />
          </Form.Item>
          <Form.Item label="操作">
            <Select
              allowClear
              style={{ width: 160 }}
              placeholder="全部"
              value={filters.action}
              options={actionOptions}
              onChange={(v) => {
                setFilters((f) => ({ ...f, action: v }));
                setPage(1);
              }}
            />
          </Form.Item>
          <Form.Item label="每页">
            <Select
              style={{ width: 100 }}
              value={pageSize}
              options={[20, 50, 100].map((n) => ({ label: String(n), value: n }))}
              onChange={(v) => {
                setPageSize(v);
                setPage(1);
              }}
            />
          </Form.Item>
        </Form>
      </Card>

      <Card size="small" title="审计记录">
        <Table
          rowKey="id"
          size="small"
          loading={loading}
          dataSource={list}
          locale={{
            emptyText: (
              <Empty description="暂无审计记录,可调整左上方筛选条件" />
            ),
          }}
          onRow={(record) => ({
            onClick: () => openDetail(record),
            style: { cursor: 'pointer' },
          })}
          pagination={{
            current: page,
            pageSize,
            total,
            showSizeChanger: false,
            onChange: (p) => setPage(p),
          }}
          columns={[
            { title: 'ID', dataIndex: 'id', width: 80 },
            { title: '用户ID', dataIndex: 'user_id', width: 90 },
            {
              title: '操作',
              dataIndex: 'action',
              width: 100,
              render: (v: string) => <Tag color={actionColor[v] || 'default'}>{v}</Tag>,
            },
            {
              title: '资源',
              key: 'resource',
              width: 220,
              render: (_: unknown, r: AuditEntry) => (
                <span>
                  <Tag>{r.resource_type}</Tag>
                  {r.resource_id ? <span>#{r.resource_id}</span> : null}
                </span>
              ),
            },
            { title: 'IP', dataIndex: 'ip', width: 140 },
            {
              title: 'UA',
              dataIndex: 'ua',
              width: 180,
              ellipsis: true,
              render: (v: string) =>
                v ? (
                  <Tooltip title={v}>
                    <span>{v}</span>
                  </Tooltip>
                ) : (
                  '-'
                ),
            },
            {
              title: 'Request ID',
              dataIndex: 'request_id',
              width: 200,
              render: (v: string) =>
                v ? (
                  <Space size={4}>
                    <Typography.Text style={{ maxWidth: 140 }} ellipsis={{ tooltip: v }}>
                      {v}
                    </Typography.Text>
                    <Tooltip title="复制 Request ID">
                      <Button
                        size="small"
                        type="text"
                        icon={<CopyOutlined />}
                        onClick={(e) => {
                          e.stopPropagation();
                          copyText(v);
                        }}
                      />
                    </Tooltip>
                  </Space>
                ) : (
                  '-'
                ),
            },
            {
              title: '时间',
              dataIndex: 'created_at',
              width: 170,
              render: (v: string) => (v ? dayjs(v).format('YYYY-MM-DD HH:mm:ss') : '-'),
            },
          ]}
          scroll={{ x: 1300 }}
        />
      </Card>

      <Drawer
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        title={detail ? `审计详情 #${detail.id}` : '审计详情'}
        width={720}
        destroyOnClose
      >
        {detail && (
          <Space direction="vertical" style={{ width: '100%' }} size="middle">
            <div>
              <Tag color={actionColor[detail.action] || 'default'}>{detail.action}</Tag>
              <Tag>{detail.resource_type}</Tag>
              {detail.resource_id && <Tag>#{detail.resource_id}</Tag>}
              <Tag color="blue">user #{detail.user_id}</Tag>
              <span style={{ marginLeft: 8, color: '#888' }}>
                {detail.created_at
                  ? dayjs(detail.created_at).format('YYYY-MM-DD HH:mm:ss')
                  : '-'}
              </span>
            </div>
            <div>
              <Typography.Text type="secondary">IP:</Typography.Text> {detail.ip || '-'}
              <span style={{ marginLeft: 16 }}>
                <Typography.Text type="secondary">Request ID:</Typography.Text>{' '}
                {detail.request_id || '-'}
              </span>
            </div>
            <div>
              <Typography.Text type="secondary">UA:</Typography.Text>{' '}
              <span style={{ wordBreak: 'break-all' }}>{detail.ua || '-'}</span>
            </div>

            <div>
              <Typography.Title level={5} style={{ margin: '8px 0' }}>
                Before
              </Typography.Title>
              <pre
                style={{
                  background: '#fafafa',
                  padding: 12,
                  borderRadius: 4,
                  maxHeight: 240,
                  overflow: 'auto',
                  fontSize: 12,
                  border: '1px solid #f0f0f0',
                }}
              >
                {safeStringify(detail.before) || '(空)'}
              </pre>
            </div>
            <div>
              <Typography.Title level={5} style={{ margin: '8px 0' }}>
                After
              </Typography.Title>
              <pre
                style={{
                  background: '#fafafa',
                  padding: 12,
                  borderRadius: 4,
                  maxHeight: 240,
                  overflow: 'auto',
                  fontSize: 12,
                  border: '1px solid #f0f0f0',
                }}
              >
                {safeStringify(detail.after) || '(空)'}
              </pre>
            </div>
          </Space>
        )}
      </Drawer>
    </Space>
  );
}
