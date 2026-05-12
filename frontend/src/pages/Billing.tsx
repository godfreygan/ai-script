import { useEffect, useMemo, useState } from 'react';
import {
  App as AntApp,
  Button,
  Card,
  Col,
  DatePicker,
  Form,
  InputNumber,
  Modal,
  Popconfirm,
  Progress,
  Row,
  Select,
  Space,
  Statistic,
  Switch,
  Table,
  Tabs,
  Tag,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import {
  DeleteOutlined,
  EditOutlined,
  PlusOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import dayjs, { Dayjs } from 'dayjs';
import {
  billingApi,
  deptApi,
  modelApi,
  userApi,
  type BillingDaily,
  type BillingQuota,
  type Department,
  type Model,
  type User,
} from '@/api/modules';

type ScopeType = 'user' | 'dept';
type Period = 'daily' | 'monthly' | 'yearly';
type Metric = 'calls' | 'tokens' | 'units' | 'cost';

const periodLabel: Record<string, string> = {
  daily: '按日',
  monthly: '按月',
  yearly: '按年',
};

const metricLabel: Record<string, string> = {
  calls: '调用次数',
  tokens: 'Token 数',
  units: '计费单元',
  cost: '成本(元)',
};

const scopeLabel: Record<string, string> = {
  user: '用户',
  dept: '部门',
};

function fmtCost(v: number | undefined): string {
  if (v === undefined || v === null) return '¥0.00';
  return `¥${Number(v).toFixed(2)}`;
}

export default function BillingPage() {
  const { message } = AntApp.useApp();

  // ---------------- 参考数据 ----------------
  const [users, setUsers] = useState<User[]>([]);
  const [depts, setDepts] = useState<Department[]>([]);
  const [models, setModels] = useState<Model[]>([]);

  const fetchRefs = async () => {
    try {
      const [u, d, m] = await Promise.all([
        userApi.list({ page_size: 500 }),
        deptApi.list(),
        modelApi.list({ page_size: 200 }),
      ]);
      setUsers(u.list);
      setDepts(d);
      setModels(m.list);
    } catch {
      /* api 已 toast */
    }
  };

  useEffect(() => {
    fetchRefs();
  }, []);

  const userOptions = useMemo(
    () =>
      users.map((u) => ({
        label: `${u.nickname || u.username} (#${u.id})`,
        value: u.id,
      })),
    [users],
  );
  const deptOptions = useMemo(
    () => depts.map((d) => ({ label: d.name, value: d.id })),
    [depts],
  );
  const modelOptions = useMemo(
    () => models.map((m) => ({ label: `${m.name} (${m.code})`, value: m.id })),
    [models],
  );

  const scopeIdOptions = (st: ScopeType | undefined) =>
    st === 'user' ? userOptions : st === 'dept' ? deptOptions : [];

  const renderScopeId = (st: string, id: number) => {
    if (st === 'user') {
      const u = users.find((x) => x.id === id);
      return u ? `${u.nickname || u.username}` : `#${id}`;
    }
    if (st === 'dept') {
      const d = depts.find((x) => x.id === id);
      return d ? d.name : `#${id}`;
    }
    return `#${id}`;
  };

  const renderModelName = (id: number) => {
    if (!id) return '全部模型';
    const m = models.find((x) => x.id === id);
    return m ? `${m.name}` : `#${id}`;
  };

  // ====================================================
  // Tab 1: 额度管理
  // ====================================================
  const [quotaScopeType, setQuotaScopeType] = useState<ScopeType | undefined>();
  const [quotaScopeId, setQuotaScopeId] = useState<number | undefined>();
  const [quotaList, setQuotaList] = useState<BillingQuota[]>([]);
  const [quotaLoading, setQuotaLoading] = useState(false);

  const fetchQuotas = async () => {
    setQuotaLoading(true);
    try {
      const data = await billingApi.listQuotas({
        scope_type: quotaScopeType,
        scope_id: quotaScopeId,
      });
      setQuotaList(data || []);
    } catch {
      /* ignore */
    } finally {
      setQuotaLoading(false);
    }
  };

  useEffect(() => {
    fetchQuotas();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [quotaScopeType, quotaScopeId]);

  // ---- 新建 modal ----
  const [createOpen, setCreateOpen] = useState(false);
  const [createForm] = Form.useForm<{
    scope_type: ScopeType;
    scope_id: number;
    model_id: number;
    period: Period;
    metric: Metric;
    quota_value: number;
    enabled: boolean;
  }>();
  const createScopeType = Form.useWatch('scope_type', createForm);

  const openCreate = () => {
    createForm.resetFields();
    createForm.setFieldsValue({
      period: 'monthly',
      metric: 'calls',
      model_id: 0,
      enabled: true,
      quota_value: 1000,
    });
    setCreateOpen(true);
  };

  const onCreate = async () => {
    const v = await createForm.validateFields();
    try {
      await billingApi.createQuota({
        scope_type: v.scope_type,
        scope_id: v.scope_id,
        model_id: v.model_id || 0,
        period: v.period,
        metric: v.metric,
        quota_value: v.quota_value,
        enabled: v.enabled ? 1 : 0,
      });
      message.success('已创建额度');
      setCreateOpen(false);
      fetchQuotas();
    } catch {
      /* ignore */
    }
  };

  // ---- 编辑 modal ----
  const [editOpen, setEditOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<BillingQuota | null>(null);
  const [editForm] = Form.useForm<{
    quota_value: number;
    period: Period;
    enabled: boolean;
  }>();

  const openEdit = (q: BillingQuota) => {
    setEditTarget(q);
    editForm.setFieldsValue({
      quota_value: q.quota_value,
      period: (q.period || 'monthly') as Period,
      enabled: !!q.enabled,
    });
    setEditOpen(true);
  };

  const onSaveEdit = async () => {
    if (!editTarget) return;
    const v = await editForm.validateFields();
    try {
      await billingApi.updateQuota(editTarget.id, {
        quota_value: v.quota_value,
        period: v.period,
        enabled: v.enabled ? 1 : 0,
      });
      message.success('已保存');
      setEditOpen(false);
      fetchQuotas();
    } catch {
      /* ignore */
    }
  };

  // ---- 行内 enabled 切换 ----
  const onToggleEnabled = async (q: BillingQuota, checked: boolean) => {
    try {
      await billingApi.updateQuota(q.id, { enabled: checked ? 1 : 0 });
      message.success(checked ? '已启用' : '已停用');
      fetchQuotas();
    } catch {
      /* ignore */
    }
  };

  const onDeleteQuota = async (id: number) => {
    try {
      await billingApi.deleteQuota(id);
      message.success('已删除');
      fetchQuotas();
    } catch {
      /* ignore */
    }
  };

  const quotaColumns: ColumnsType<BillingQuota> = [
    { title: 'ID', dataIndex: 'id', width: 70 },
    {
      title: '范围',
      key: 'scope',
      width: 180,
      render: (_: unknown, r: BillingQuota) => (
        <Space size={4}>
          <Tag color={r.scope_type === 'user' ? 'blue' : 'green'}>
            {scopeLabel[r.scope_type] || r.scope_type}
          </Tag>
          <span>{renderScopeId(r.scope_type, r.scope_id)}</span>
        </Space>
      ),
    },
    {
      title: '模型',
      dataIndex: 'model_id',
      width: 160,
      render: (v: number) => renderModelName(v),
    },
    {
      title: '周期',
      dataIndex: 'period',
      width: 80,
      render: (v: string) => periodLabel[v] || v,
    },
    {
      title: '指标',
      dataIndex: 'metric',
      width: 110,
      render: (v: string) => metricLabel[v] || v,
    },
    {
      title: '额度',
      dataIndex: 'quota_value',
      width: 110,
      render: (v: number, r: BillingQuota) =>
        r.metric === 'cost' ? fmtCost(v) : v.toLocaleString(),
    },
    {
      title: '已用',
      dataIndex: 'used_value',
      width: 110,
      render: (v: number, r: BillingQuota) =>
        r.metric === 'cost' ? fmtCost(v) : (v || 0).toLocaleString(),
    },
    {
      title: '使用率',
      key: 'usage',
      width: 180,
      render: (_: unknown, r: BillingQuota) => {
        const pct =
          r.quota_value > 0
            ? Math.min(100, Math.round(((r.used_value || 0) / r.quota_value) * 100))
            : 0;
        const status =
          pct >= 100 ? 'exception' : pct >= 80 ? 'active' : 'normal';
        return <Progress percent={pct} size="small" status={status} />;
      },
    },
    {
      title: '启用',
      dataIndex: 'enabled',
      width: 80,
      render: (v: number, r: BillingQuota) => (
        <Switch
          size="small"
          checked={!!v}
          onChange={(c) => onToggleEnabled(r, c)}
        />
      ),
    },
    {
      title: '操作',
      key: 'op',
      width: 160,
      render: (_: unknown, r: BillingQuota) => (
        <Space size={4}>
          <Button
            size="small"
            icon={<EditOutlined />}
            onClick={() => openEdit(r)}
          >
            编辑
          </Button>
          <Popconfirm
            title="确认删除该额度?"
            onConfirm={() => onDeleteQuota(r.id)}
          >
            <Button size="small" danger icon={<DeleteOutlined />}>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  // ====================================================
  // Tab 2: 用量统计
  // ====================================================
  const [dailyRange, setDailyRange] = useState<[Dayjs, Dayjs]>(() => [
    dayjs().subtract(29, 'day').startOf('day'),
    dayjs().endOf('day'),
  ]);
  const [dailyUserId, setDailyUserId] = useState<number | undefined>();
  const [dailyDeptId, setDailyDeptId] = useState<number | undefined>();
  const [dailyModelId, setDailyModelId] = useState<number | undefined>();
  const [dailyList, setDailyList] = useState<BillingDaily[]>([]);
  const [dailyLoading, setDailyLoading] = useState(false);

  const fetchDaily = async () => {
    setDailyLoading(true);
    try {
      const [from, to] = dailyRange;
      const data = await billingApi.listDaily({
        from: from?.format('YYYY-MM-DD'),
        to: to?.format('YYYY-MM-DD'),
        user_id: dailyUserId,
        dept_id: dailyDeptId,
        model_id: dailyModelId,
      });
      setDailyList(data || []);
    } catch {
      /* ignore */
    } finally {
      setDailyLoading(false);
    }
  };

  useEffect(() => {
    fetchDaily();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dailyRange, dailyUserId, dailyDeptId, dailyModelId]);

  // 聚合统计
  const totals = useMemo(() => {
    let calls = 0,
      tokens = 0,
      cost = 0;
    for (const r of dailyList) {
      calls += r.calls || 0;
      tokens += (r.input_tokens || 0) + (r.output_tokens || 0);
      cost += r.cost || 0;
    }
    return { calls, tokens, cost };
  }, [dailyList]);

  // 按日聚合的简易折线/Table 数据
  const byDate = useMemo(() => {
    const map = new Map<string, { date: string; calls: number; tokens: number; cost: number }>();
    for (const r of dailyList) {
      const k = r.stat_date;
      const cur = map.get(k) || { date: k, calls: 0, tokens: 0, cost: 0 };
      cur.calls += r.calls || 0;
      cur.tokens += (r.input_tokens || 0) + (r.output_tokens || 0);
      cur.cost += r.cost || 0;
      map.set(k, cur);
    }
    return Array.from(map.values()).sort((a, b) => a.date.localeCompare(b.date));
  }, [dailyList]);

  const dailyColumns: ColumnsType<BillingDaily> = [
    { title: '日期', dataIndex: 'stat_date', width: 120 },
    {
      title: '模型',
      dataIndex: 'model_id',
      width: 160,
      render: (v: number) => renderModelName(v),
    },
    {
      title: '用户',
      dataIndex: 'user_id',
      width: 140,
      render: (v: number) => (v ? renderScopeId('user', v) : '-'),
    },
    {
      title: '部门',
      dataIndex: 'dept_id',
      width: 140,
      render: (v: number) => (v ? renderScopeId('dept', v) : '-'),
    },
    {
      title: '调用',
      dataIndex: 'calls',
      width: 90,
      render: (v: number) => (v || 0).toLocaleString(),
    },
    {
      title: '入参 Token',
      dataIndex: 'input_tokens',
      width: 120,
      render: (v: number) => (v || 0).toLocaleString(),
    },
    {
      title: '出参 Token',
      dataIndex: 'output_tokens',
      width: 120,
      render: (v: number) => (v || 0).toLocaleString(),
    },
    {
      title: '计费单元',
      dataIndex: 'units',
      width: 110,
      render: (v: number) => (v || 0).toLocaleString(),
    },
    {
      title: '成本',
      dataIndex: 'cost',
      width: 110,
      render: (v: number) => fmtCost(v),
    },
  ];

  const trendColumns: ColumnsType<{
    date: string;
    calls: number;
    tokens: number;
    cost: number;
  }> = [
    { title: '日期', dataIndex: 'date', width: 120 },
    {
      title: '调用',
      dataIndex: 'calls',
      width: 110,
      render: (v: number) => v.toLocaleString(),
    },
    {
      title: 'Token 合计',
      dataIndex: 'tokens',
      width: 140,
      render: (v: number) => v.toLocaleString(),
    },
    {
      title: '成本',
      dataIndex: 'cost',
      width: 110,
      render: (v: number) => fmtCost(v),
    },
  ];

  // ====================================================
  // Render
  // ====================================================
  return (
    <Card title="计费 / 额度管理">
      <Tabs
        defaultActiveKey="quota"
        items={[
          {
            key: 'quota',
            label: '额度管理',
            children: (
              <>
                <Space wrap style={{ marginBottom: 12 }}>
                  <Select
                    allowClear
                    placeholder="范围类型"
                    style={{ width: 130 }}
                    value={quotaScopeType}
                    options={[
                      { label: '用户', value: 'user' },
                      { label: '部门', value: 'dept' },
                    ]}
                    onChange={(v) => {
                      setQuotaScopeType(v as ScopeType | undefined);
                      setQuotaScopeId(undefined);
                    }}
                  />
                  <Select
                    allowClear
                    showSearch
                    optionFilterProp="label"
                    placeholder={
                      quotaScopeType
                        ? `选择${scopeLabel[quotaScopeType]}`
                        : '先选范围类型'
                    }
                    style={{ width: 220 }}
                    value={quotaScopeId}
                    disabled={!quotaScopeType}
                    options={scopeIdOptions(quotaScopeType)}
                    onChange={(v) => setQuotaScopeId(v)}
                  />
                  <Button icon={<ReloadOutlined />} onClick={fetchQuotas}>
                    刷新
                  </Button>
                  <Button
                    type="primary"
                    icon={<PlusOutlined />}
                    onClick={openCreate}
                  >
                    新建额度
                  </Button>
                </Space>

                <Table<BillingQuota>
                  rowKey="id"
                  loading={quotaLoading}
                  dataSource={quotaList}
                  columns={quotaColumns}
                  pagination={{ pageSize: 20, showSizeChanger: true }}
                  scroll={{ x: 1200 }}
                />
              </>
            ),
          },
          {
            key: 'usage',
            label: '用量统计',
            children: (
              <>
                <Space wrap style={{ marginBottom: 12 }}>
                  <DatePicker.RangePicker
                    value={dailyRange}
                    onChange={(v) => {
                      if (v && v[0] && v[1]) {
                        setDailyRange([v[0], v[1]]);
                      }
                    }}
                    allowClear={false}
                  />
                  <Select
                    allowClear
                    showSearch
                    optionFilterProp="label"
                    placeholder="用户"
                    style={{ width: 180 }}
                    value={dailyUserId}
                    options={userOptions}
                    onChange={(v) => setDailyUserId(v)}
                  />
                  <Select
                    allowClear
                    showSearch
                    optionFilterProp="label"
                    placeholder="部门"
                    style={{ width: 160 }}
                    value={dailyDeptId}
                    options={deptOptions}
                    onChange={(v) => setDailyDeptId(v)}
                  />
                  <Select
                    allowClear
                    showSearch
                    optionFilterProp="label"
                    placeholder="模型"
                    style={{ width: 200 }}
                    value={dailyModelId}
                    options={modelOptions}
                    onChange={(v) => setDailyModelId(v)}
                  />
                  <Button icon={<ReloadOutlined />} onClick={fetchDaily}>
                    刷新
                  </Button>
                </Space>

                <Row gutter={16} style={{ marginBottom: 16 }}>
                  <Col span={8}>
                    <Card size="small">
                      <Statistic title="总调用数" value={totals.calls} />
                    </Card>
                  </Col>
                  <Col span={8}>
                    <Card size="small">
                      <Statistic title="总 Token 数" value={totals.tokens} />
                    </Card>
                  </Col>
                  <Col span={8}>
                    <Card size="small">
                      <Statistic
                        title="总成本"
                        value={totals.cost}
                        precision={2}
                        prefix="¥"
                      />
                    </Card>
                  </Col>
                </Row>

                <Card
                  size="small"
                  title="按日趋势"
                  style={{ marginBottom: 16 }}
                >
                  <Table
                    rowKey="date"
                    size="small"
                    dataSource={byDate}
                    columns={trendColumns}
                    pagination={{ pageSize: 10 }}
                  />
                </Card>

                <Card size="small" title="明细">
                  <Table<BillingDaily>
                    rowKey="id"
                    size="small"
                    loading={dailyLoading}
                    dataSource={dailyList}
                    columns={dailyColumns}
                    pagination={{ pageSize: 20, showSizeChanger: true }}
                    scroll={{ x: 1100 }}
                  />
                </Card>
              </>
            ),
          },
        ]}
      />

      {/* 新建额度 Modal */}
      <Modal
        open={createOpen}
        title="新建额度"
        onCancel={() => setCreateOpen(false)}
        onOk={onCreate}
        okText="创建"
        cancelText="取消"
        destroyOnClose
        width={520}
      >
        <Form form={createForm} layout="vertical">
          <Form.Item
            name="scope_type"
            label="范围类型"
            rules={[{ required: true, message: '请选择范围类型' }]}
          >
            <Select
              placeholder="user 或 dept"
              options={[
                { label: '用户', value: 'user' },
                { label: '部门', value: 'dept' },
              ]}
              onChange={() => createForm.setFieldValue('scope_id', undefined)}
            />
          </Form.Item>
          <Form.Item
            name="scope_id"
            label="范围对象"
            rules={[{ required: true, message: '请选择对象' }]}
          >
            <Select
              showSearch
              optionFilterProp="label"
              placeholder={
                createScopeType
                  ? `选择${scopeLabel[createScopeType]}`
                  : '先选范围类型'
              }
              disabled={!createScopeType}
              options={scopeIdOptions(createScopeType as ScopeType | undefined)}
            />
          </Form.Item>
          <Form.Item name="model_id" label="模型 (0=全部模型)">
            <Select
              showSearch
              optionFilterProp="label"
              placeholder="选择模型,留空=全部模型"
              options={[
                { label: '全部模型', value: 0 },
                ...modelOptions,
              ]}
            />
          </Form.Item>
          <Form.Item
            name="period"
            label="周期"
            rules={[{ required: true }]}
          >
            <Select
              options={[
                { label: '按月', value: 'monthly' },
                { label: '按日', value: 'daily' },
                { label: '按年', value: 'yearly' },
              ]}
            />
          </Form.Item>
          <Form.Item
            name="metric"
            label="指标"
            rules={[{ required: true }]}
          >
            <Select
              options={[
                { label: '调用次数 (calls)', value: 'calls' },
                { label: 'Token 数 (tokens)', value: 'tokens' },
                { label: '计费单元 (units)', value: 'units' },
                { label: '成本 (cost)', value: 'cost' },
              ]}
            />
          </Form.Item>
          <Form.Item
            name="quota_value"
            label="额度值"
            rules={[{ required: true, message: '请输入额度' }]}
          >
            <InputNumber min={0} step={100} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="enabled" label="启用" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>

      {/* 编辑额度 Modal */}
      <Modal
        open={editOpen}
        title={`编辑额度 #${editTarget?.id ?? ''}`}
        onCancel={() => setEditOpen(false)}
        onOk={onSaveEdit}
        okText="保存"
        cancelText="取消"
        destroyOnClose
        width={480}
      >
        <Form form={editForm} layout="vertical">
          <Form.Item
            name="quota_value"
            label="额度值"
            rules={[{ required: true }]}
          >
            <InputNumber min={0} step={100} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="period" label="周期" rules={[{ required: true }]}>
            <Select
              options={[
                { label: '按月', value: 'monthly' },
                { label: '按日', value: 'daily' },
                { label: '按年', value: 'yearly' },
              ]}
            />
          </Form.Item>
          <Form.Item name="enabled" label="启用" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
}
