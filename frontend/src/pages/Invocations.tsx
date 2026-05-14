import { useEffect, useState } from 'react';
import {
  Card,
  Col,
  DatePicker,
  Form,
  Row,
  Select,
  Space,
  Statistic,
  Table,
  Tag,
} from 'antd';
import { BarChartOutlined } from '@ant-design/icons';
import dayjs, { Dayjs } from 'dayjs';
import {
  InvocationLog,
  InvocationStats,
  Model,
  Project,
  User,
  invocationApi,
  modelApi,
  projectApi,
  userApi,
} from '@/api/modules';

const { RangePicker } = DatePicker;

const bizTypeOptions = [
  { label: '剧本拆分', value: 'script_split' },
  { label: '提示词生成', value: 'prompt_gen' },
  { label: '分镜生成', value: 'storyboard_gen' },
  { label: '图片生成', value: 'image_gen' },
  { label: '视频生成', value: 'video_gen' },
];

const statusColor: Record<string, string> = {
  succeeded: 'success',
  failed: 'error',
  running: 'processing',
  pending: 'default',
};

export default function InvocationsPage() {
  const [projects, setProjects] = useState<Project[]>([]);
  const [models, setModels] = useState<Model[]>([]);
  const [users, setUsers] = useState<User[]>([]);

  const [filters, setFilters] = useState<{
    user_id?: number;
    project_id?: number;
    model_id?: number;
    biz_type?: string;
    status?: string;
    range?: [Dayjs, Dayjs] | null;
  }>({});

  const [list, setList] = useState<InvocationLog[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [loading, setLoading] = useState(false);

  const [stats, setStats] = useState<InvocationStats | null>(null);

  const fetchRefs = async () => {
    try {
      const [p, m, u] = await Promise.all([
        projectApi.list({ page_size: 200 }),
        modelApi.list({ page_size: 200 }),
        userApi.list({ page_size: 200 }),
      ]);
      setProjects(p.list);
      setModels(m.list);
      setUsers(u.list);
    } catch (err) {
      // eslint-disable-next-line no-console
      console.error('fetch invocation refs failed:', err);
    }
  };

  const buildQuery = () => {
    const q: Record<string, unknown> = {
      user_id: filters.user_id,
      project_id: filters.project_id,
      model_id: filters.model_id,
      biz_type: filters.biz_type,
      status: filters.status,
    };
    if (filters.range && filters.range[0] && filters.range[1]) {
      q.from = filters.range[0].toISOString();
      q.to = filters.range[1].toISOString();
    }
    return q;
  };

  const fetchList = async () => {
    setLoading(true);
    try {
      const q = buildQuery();
      const data = await invocationApi.list({
        ...q,
        page,
        page_size: pageSize,
      });
      setList(data.list);
      setTotal(data.total);
    } catch (err) {
      // eslint-disable-next-line no-console
      console.error('fetch invocations failed:', err);
      setList([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  };

  const fetchStats = async () => {
    try {
      const s = await invocationApi.stats(buildQuery());
      setStats(s);
    } catch (err) {
      // eslint-disable-next-line no-console
      console.error('fetch invocation stats failed:', err);
      setStats(null);
    }
  };

  useEffect(() => {
    fetchRefs();
  }, []);

  useEffect(() => {
    fetchList();
    fetchStats();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, pageSize, filters]);

  return (
    <Space direction="vertical" style={{ width: '100%' }} size="large">
      <Card
        title={
          <Space>
            <BarChartOutlined />
            模型调用统计
          </Space>
        }
        size="small"
      >
        <Row gutter={16}>
          <Col span={5}>
            <Statistic title="调用次数" value={stats?.calls ?? 0} />
          </Col>
          <Col span={5}>
            <Statistic title="输入 Tokens" value={stats?.input_tokens ?? 0} />
          </Col>
          <Col span={5}>
            <Statistic title="输出 Tokens" value={stats?.output_tokens ?? 0} />
          </Col>
          <Col span={5}>
            <Statistic title="单元数" value={stats?.units ?? 0} />
          </Col>
          <Col span={4}>
            <Statistic
              title="累计成本"
              value={stats?.cost ?? 0}
              precision={4}
              prefix="¥"
            />
          </Col>
        </Row>
      </Card>

      <Card size="small">
        <Form layout="inline">
          <Form.Item label="用户">
            <Select
              allowClear
              showSearch
              optionFilterProp="label"
              style={{ width: 180 }}
              placeholder="按用户筛选"
              value={filters.user_id}
              options={users.map((u) => ({
                label: u.nickname || u.username,
                value: u.id,
              }))}
              onChange={(v) => {
                setFilters((f) => ({ ...f, user_id: v }));
                setPage(1);
              }}
            />
          </Form.Item>
          <Form.Item label="项目">
            <Select
              allowClear
              showSearch
              optionFilterProp="label"
              style={{ width: 180 }}
              placeholder="按项目筛选"
              value={filters.project_id}
              options={projects.map((p) => ({ label: p.name, value: p.id }))}
              onChange={(v) => {
                setFilters((f) => ({ ...f, project_id: v }));
                setPage(1);
              }}
            />
          </Form.Item>
          <Form.Item label="模型">
            <Select
              allowClear
              showSearch
              optionFilterProp="label"
              style={{ width: 200 }}
              placeholder="按模型筛选"
              value={filters.model_id}
              options={models.map((m) => ({
                label: `${m.name} (${m.code})`,
                value: m.id,
              }))}
              onChange={(v) => {
                setFilters((f) => ({ ...f, model_id: v }));
                setPage(1);
              }}
            />
          </Form.Item>
          <Form.Item label="业务类型">
            <Select
              allowClear
              style={{ width: 140 }}
              placeholder="业务"
              value={filters.biz_type}
              options={bizTypeOptions}
              onChange={(v) => {
                setFilters((f) => ({ ...f, biz_type: v }));
                setPage(1);
              }}
            />
          </Form.Item>
          <Form.Item label="状态">
            <Select
              allowClear
              style={{ width: 120 }}
              placeholder="状态"
              value={filters.status}
              options={Object.keys(statusColor).map((k) => ({ label: k, value: k }))}
              onChange={(v) => {
                setFilters((f) => ({ ...f, status: v }));
                setPage(1);
              }}
            />
          </Form.Item>
          <Form.Item label="时间范围">
            <RangePicker
              showTime
              value={filters.range as [Dayjs, Dayjs] | undefined}
              onChange={(v) => {
                setFilters((f) => ({ ...f, range: v as [Dayjs, Dayjs] | null }));
                setPage(1);
              }}
            />
          </Form.Item>
        </Form>
      </Card>

      <Card size="small" title="调用日志">
        <Table
          rowKey="id"
          size="small"
          loading={loading}
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
            { title: 'ID', dataIndex: 'id', width: 80 },
            {
              title: '业务',
              dataIndex: 'biz_type',
              width: 120,
              render: (v: string) => <Tag color="blue">{v}</Tag>,
            },
            {
              title: '业务引用',
              dataIndex: 'biz_ref',
              width: 140,
              ellipsis: true,
            },
            {
              title: '模型',
              dataIndex: 'model_id',
              width: 180,
              render: (id: number) => {
                const m = models.find((x) => x.id === id);
                return m ? `${m.name} (${m.code})` : `#${id}`;
              },
            },
            {
              title: '用户',
              dataIndex: 'user_id',
              width: 120,
              render: (id: number) => {
                const u = users.find((x) => x.id === id);
                return u ? u.nickname || u.username : `#${id}`;
              },
            },
            {
              title: '项目',
              dataIndex: 'project_id',
              width: 140,
              render: (id: number) =>
                id ? projects.find((p) => p.id === id)?.name || `#${id}` : '-',
            },
            { title: '输入 Tokens', dataIndex: 'input_tokens', width: 100 },
            { title: '输出 Tokens', dataIndex: 'output_tokens', width: 100 },
            { title: '单元', dataIndex: 'units', width: 60 },
            {
              title: '耗时(ms)',
              dataIndex: 'duration_ms',
              width: 100,
            },
            {
              title: '成本',
              dataIndex: 'cost',
              width: 100,
              render: (c: number) => (c ? `¥${c.toFixed(4)}` : '-'),
            },
            {
              title: '状态',
              dataIndex: 'status',
              width: 90,
              render: (s: string) => (
                <Tag color={statusColor[s] || 'default'}>{s}</Tag>
              ),
            },
            {
              title: '错误码',
              dataIndex: 'error_code',
              width: 140,
              ellipsis: true,
            },
            {
              title: '开始时间',
              dataIndex: 'started_at',
              width: 170,
              render: (v: string) => (v ? dayjs(v).format('YYYY-MM-DD HH:mm:ss') : '-'),
            },
          ]}
          scroll={{ x: 1800 }}
        />
      </Card>
    </Space>
  );
}
