import { useEffect, useState } from 'react';
import {
  App as AntApp,
  Button,
  Card,
  Descriptions,
  Drawer,
  Form,
  Input,
  Modal,
  Select,
  Space,
  Table,
  Tabs,
  Tag,
  Timeline,
  Tooltip,
  Typography,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  PlusOutlined,
  ReloadOutlined,
  StopOutlined,
} from '@ant-design/icons';
import {
  fullVideoApi,
  reviewApi,
  type FullVideo,
  type ReviewFlow,
  type ReviewNode,
  type ReviewNodeRecord,
  type ReviewRecord,
} from '@/api/modules';
import { useAuthStore } from '@/stores/auth';

const { Text } = Typography;

const statusColor: Record<string, string> = {
  pending: 'processing',
  approved: 'success',
  rejected: 'error',
  cancelled: 'default',
};

const statusLabel: Record<string, string> = {
  pending: '待审核',
  approved: '已通过',
  rejected: '已驳回',
  cancelled: '已撤回',
};

const actionColor: Record<string, string> = {
  approve: 'green',
  reject: 'red',
  skip: 'gray',
};

const actionLabel: Record<string, string> = {
  approve: '通过',
  reject: '驳回',
  skip: '跳过',
};

const targetTypeOptions: { label: string; value: string }[] = [
  { label: '完整视频', value: 'full_video' },
];

export default function ReviewsPage() {
  const { message } = AntApp.useApp();
  const user = useAuthStore((s) => s.user);
  const currentUid = user?.id;

  const [activeTab, setActiveTab] = useState<'records' | 'flows'>('records');

  // ---------------- 审核记录 ----------------
  const [records, setRecords] = useState<ReviewRecord[]>([]);
  const [recordsLoading, setRecordsLoading] = useState(false);
  const [recordsTotal, setRecordsTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [statusFilter, setStatusFilter] = useState<string | undefined>();

  // ---------------- 审核流配置 ----------------
  const [flows, setFlows] = useState<ReviewFlow[]>([]);
  const [flowsLoading, setFlowsLoading] = useState(false);
  const [flowNodesMap, setFlowNodesMap] = useState<Record<number, ReviewNode[]>>({});
  const [flowNodesLoading, setFlowNodesLoading] = useState<Record<number, boolean>>({});
  const [expandedFlowKeys, setExpandedFlowKeys] = useState<number[]>([]);

  // ---------------- 参考数据 ----------------
  const [videos, setVideos] = useState<FullVideo[]>([]);

  // ---------------- Drawer (记录详情) ----------------
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [drawerRecord, setDrawerRecord] = useState<ReviewRecord | null>(null);
  const [drawerActions, setDrawerActions] = useState<ReviewNodeRecord[]>([]);
  const [drawerLoading, setDrawerLoading] = useState(false);
  const [drawerFlowNodes, setDrawerFlowNodes] = useState<ReviewNode[]>([]);
  const [actComment, setActComment] = useState('');
  const [acting, setActing] = useState(false);

  // ---------------- 提交审核 Modal ----------------
  const [submitOpen, setSubmitOpen] = useState(false);
  const [submitForm] = Form.useForm<{
    target_type: string;
    target_id: number;
    flow_id?: number;
    note?: string;
  }>();
  const [submitting, setSubmitting] = useState(false);

  // ---------------- 拉数据 ----------------
  const fetchRecords = async () => {
    setRecordsLoading(true);
    try {
      const data = await reviewApi.listRecords({
        page,
        page_size: pageSize,
        status: statusFilter,
      });
      // 防护: data 为 undefined 时避免炸
      setRecords(data?.list ?? []);
      setRecordsTotal(data?.total ?? 0);
    } catch (err) {
      message.error((err as Error)?.message || '加载审核记录失败');
      setRecords([]);
      setRecordsTotal(0);
    } finally {
      setRecordsLoading(false);
    }
  };

  const fetchFlows = async () => {
    setFlowsLoading(true);
    try {
      const data = await reviewApi.listFlows();
      setFlows(data || []);
    } catch (err) {
      message.error((err as Error)?.message || '加载审核流失败');
      setFlows([]);
    } finally {
      setFlowsLoading(false);
    }
  };

  const fetchFlowNodes = async (flowId: number) => {
    setFlowNodesLoading((prev) => ({ ...prev, [flowId]: true }));
    try {
      const nodes = await reviewApi.listNodes(flowId);
      setFlowNodesMap((prev) => ({ ...prev, [flowId]: nodes || [] }));
    } catch (err) {
      message.error((err as Error)?.message || '加载审核流节点失败');
    } finally {
      setFlowNodesLoading((prev) => ({ ...prev, [flowId]: false }));
    }
  };

  const fetchVideos = async () => {
    try {
      const data = await fullVideoApi.list({ page_size: 200 });
      // 防护: data 为 undefined 时避免炸
      setVideos(data?.list ?? []);
    } catch (err) {
      message.error((err as Error)?.message || '加载视频列表失败');
      setVideos([]);
    }
  };

  useEffect(() => {
    fetchVideos();
    fetchFlows();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (activeTab === 'records') {
      fetchRecords();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeTab, page, pageSize, statusFilter]);

  // ---------------- 工具函数 ----------------
  const videoNameById = (id: number): string => {
    const v = videos.find((x) => x.id === id);
    return v ? v.name : `#${id}`;
  };

  const renderTargetCell = (targetType: string, targetId: number) => {
    if (targetType === 'full_video') {
      return (
        <Tooltip title={`点击查看视频 #${targetId}`}>
          <a href={`/full-videos?id=${targetId}`}>{videoNameById(targetId)}</a>
        </Tooltip>
      );
    }
    return <Text>{`${targetType} #${targetId}`}</Text>;
  };

  // ---------------- 打开 Drawer ----------------
  const openDrawer = async (rec: ReviewRecord) => {
    setDrawerRecord(rec);
    setDrawerOpen(true);
    setDrawerActions([]);
    setDrawerFlowNodes([]);
    setActComment('');
    setDrawerLoading(true);
    try {
      const [actions, nodes] = await Promise.all([
        reviewApi.listActions(rec.id),
        rec.flow_id ? reviewApi.listNodes(rec.flow_id) : Promise.resolve<ReviewNode[]>([]),
      ]);
      setDrawerActions(actions || []);
      setDrawerFlowNodes(nodes || []);
    } catch (err) {
      message.error((err as Error)?.message || '加载审核详情失败');
    } finally {
      setDrawerLoading(false);
    }
  };

  const refreshDrawer = async () => {
    if (!drawerRecord) return;
    setDrawerLoading(true);
    try {
      const [rec, actions] = await Promise.all([
        reviewApi.getRecord(drawerRecord.id),
        reviewApi.listActions(drawerRecord.id),
      ]);
      setDrawerRecord(rec ?? null);
      setDrawerActions(actions || []);
    } catch (err) {
      message.error((err as Error)?.message || '刷新审核详情失败');
    } finally {
      setDrawerLoading(false);
    }
  };

  // ---------------- 审核操作 ----------------
  const onAct = async (action: 'approve' | 'reject' | 'skip') => {
    if (!drawerRecord) return;
    setActing(true);
    try {
      await reviewApi.act(drawerRecord.id, {
        action,
        comment: actComment || undefined,
      });
      message.success(`${actionLabel[action]}成功`);
      setActComment('');
      await refreshDrawer();
      fetchRecords();
    } catch (err) {
      message.error((err as Error)?.message || '操作失败');
    } finally {
      setActing(false);
    }
  };

  const onCancel = async () => {
    if (!drawerRecord) return;
    setActing(true);
    try {
      await reviewApi.cancel(drawerRecord.id);
      message.success('已撤回');
      await refreshDrawer();
      fetchRecords();
    } catch (err) {
      message.error((err as Error)?.message || '撤回失败');
    } finally {
      setActing(false);
    }
  };

  // ---------------- 提交审核 ----------------
  const openSubmit = () => {
    submitForm.resetFields();
    submitForm.setFieldsValue({ target_type: 'full_video' });
    setSubmitOpen(true);
  };

  const onSubmit = async () => {
    const v = await submitForm.validateFields();
    setSubmitting(true);
    try {
      await reviewApi.submit({
        target_type: v.target_type,
        target_id: v.target_id,
        flow_id: v.flow_id || undefined,
        note: v.note || undefined,
      });
      message.success('已提交审核');
      setSubmitOpen(false);
      fetchRecords();
    } catch (err) {
      message.error((err as Error)?.message || '提交审核失败');
    } finally {
      setSubmitting(false);
    }
  };

  // ---------------- 当前节点 (用于判断 skip 是否启用) ----------------
  const currentNodeOfDrawer = (): ReviewNode | undefined => {
    if (!drawerRecord) return undefined;
    return drawerFlowNodes.find((n) => n.step_no === drawerRecord.current_step);
  };

  const canSkip = (): boolean => {
    const node = currentNodeOfDrawer();
    return !!node && node.allow_timeout_pass === 1;
  };

  const canCancel = (): boolean => {
    if (!drawerRecord) return false;
    if (drawerRecord.status !== 'pending') return false;
    if (!currentUid) return false;
    return drawerRecord.submitted_by === currentUid;
  };

  // ---------------- 表格列: 审核记录 ----------------
  const recordColumns: ColumnsType<ReviewRecord> = [
    { title: 'ID', dataIndex: 'id', width: 70 },
    {
      title: '目标类型',
      dataIndex: 'target_type',
      width: 110,
      render: (v: string) =>
        v === 'full_video' ? <Tag color="blue">完整视频</Tag> : <Tag>{v}</Tag>,
    },
    {
      title: '目标',
      dataIndex: 'target_id',
      width: 200,
      render: (id: number, r: ReviewRecord) => renderTargetCell(r.target_type, id),
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
      title: '当前步骤',
      dataIndex: 'current_step',
      width: 90,
      render: (v: number) => `第 ${v} 步`,
    },
    {
      title: '提交人',
      dataIndex: 'submitted_by',
      width: 90,
      render: (v: number) => `#${v}`,
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      width: 170,
    },
    {
      title: '操作',
      key: 'op',
      width: 100,
      render: (_: unknown, r: ReviewRecord) => (
        <Button type="link" size="small" onClick={() => openDrawer(r)}>
          详情
        </Button>
      ),
    },
  ];

  // ---------------- 表格列: 审核流 ----------------
  const flowColumns: ColumnsType<ReviewFlow> = [
    { title: 'ID', dataIndex: 'id', width: 70 },
    { title: '名称', dataIndex: 'name' },
    {
      title: '目标类型',
      dataIndex: 'target_type',
      width: 140,
      render: (v: string) =>
        v === 'full_video' ? <Tag color="blue">完整视频</Tag> : <Tag>{v}</Tag>,
    },
    {
      title: '默认',
      dataIndex: 'is_default',
      width: 80,
      render: (v: number) => (v === 1 ? <Tag color="gold">默认</Tag> : '-'),
    },
    {
      title: '启用',
      dataIndex: 'enabled',
      width: 80,
      render: (v: number) =>
        v === 1 ? <Tag color="success">启用</Tag> : <Tag>停用</Tag>,
    },
  ];

  // ---------------- Render ----------------
  return (
    <Card
      title="审核管理"
      extra={
        <Space>
          {activeTab === 'records' && (
            <>
              <Select
                allowClear
                placeholder="状态过滤"
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
              <Button
                icon={<ReloadOutlined />}
                onClick={() => fetchRecords()}
              >
                刷新
              </Button>
              <Button
                type="primary"
                icon={<PlusOutlined />}
                onClick={openSubmit}
              >
                提交审核
              </Button>
            </>
          )}
          {activeTab === 'flows' && (
            <Button icon={<ReloadOutlined />} onClick={() => fetchFlows()}>
              刷新
            </Button>
          )}
        </Space>
      }
    >
      <Tabs
        activeKey={activeTab}
        onChange={(k) => setActiveTab(k as 'records' | 'flows')}
        items={[
          {
            key: 'records',
            label: '审核记录',
            children: (
              <Table<ReviewRecord>
                rowKey="id"
                loading={recordsLoading}
                dataSource={records}
                columns={recordColumns}
                locale={{ emptyText: '暂无审核记录' }}
                pagination={{
                  current: page,
                  pageSize,
                  total: recordsTotal,
                  showSizeChanger: true,
                  onChange: (p, ps) => {
                    setPage(p);
                    setPageSize(ps);
                  },
                }}
              />
            ),
          },
          {
            key: 'flows',
            label: '审核流配置',
            children: (
              <Table<ReviewFlow>
                rowKey="id"
                loading={flowsLoading}
                dataSource={flows}
                columns={flowColumns}
                locale={{ emptyText: '暂无审核流' }}
                pagination={false}
                expandable={{
                  expandedRowKeys: expandedFlowKeys,
                  onExpand: (expanded, record) => {
                    if (expanded) {
                      setExpandedFlowKeys((prev) => [...prev, record.id]);
                      if (!flowNodesMap[record.id]) {
                        fetchFlowNodes(record.id);
                      }
                    } else {
                      setExpandedFlowKeys((prev) =>
                        prev.filter((k) => k !== record.id),
                      );
                    }
                  },
                  expandedRowRender: (record) => {
                    const nodes = flowNodesMap[record.id];
                    const loading = !!flowNodesLoading[record.id];
                    return (
                      <Table<ReviewNode>
                        rowKey="id"
                        size="small"
                        loading={loading}
                        pagination={false}
                        dataSource={nodes || []}
                        locale={{ emptyText: '暂无节点' }}
                        columns={[
                          { title: '步骤', dataIndex: 'step_no', width: 80 },
                          { title: '名称', dataIndex: 'name' },
                          {
                            title: '审批人类型',
                            dataIndex: 'approver_type',
                            width: 140,
                          },
                          {
                            title: '审批人值',
                            dataIndex: 'approver_value',
                            width: 160,
                          },
                          {
                            title: '允许超时通过',
                            dataIndex: 'allow_timeout_pass',
                            width: 130,
                            render: (v: number) =>
                              v === 1 ? (
                                <Tag color="success">是</Tag>
                              ) : (
                                <Tag>否</Tag>
                              ),
                          },
                          {
                            title: '超时(h)',
                            dataIndex: 'timeout_hours',
                            width: 100,
                          },
                        ]}
                      />
                    );
                  },
                }}
              />
            ),
          },
        ]}
      />

      {/* 详情 Drawer */}
      <Drawer
        open={drawerOpen}
        width={620}
        title={
          drawerRecord
            ? `审核记录 #${drawerRecord.id}`
            : '审核记录详情'
        }
        onClose={() => setDrawerOpen(false)}
        destroyOnClose
        extra={
          canCancel() && (
            <Button danger icon={<StopOutlined />} loading={acting} onClick={onCancel}>
              撤回
            </Button>
          )
        }
      >
        {drawerRecord && (
          <>
            <Descriptions
              size="small"
              column={1}
              bordered
              style={{ marginBottom: 16 }}
            >
              <Descriptions.Item label="目标类型">
                {drawerRecord.target_type}
              </Descriptions.Item>
              <Descriptions.Item label="目标">
                {renderTargetCell(drawerRecord.target_type, drawerRecord.target_id)}
              </Descriptions.Item>
              <Descriptions.Item label="状态">
                <Tag color={statusColor[drawerRecord.status] || 'default'}>
                  {statusLabel[drawerRecord.status] || drawerRecord.status}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="当前步骤">
                第 {drawerRecord.current_step} 步
                {currentNodeOfDrawer() && (
                  <span style={{ marginLeft: 8, color: '#888' }}>
                    ({currentNodeOfDrawer()?.name})
                  </span>
                )}
              </Descriptions.Item>
              <Descriptions.Item label="提交人">
                #{drawerRecord.submitted_by}
              </Descriptions.Item>
              <Descriptions.Item label="创建时间">
                {drawerRecord.created_at}
              </Descriptions.Item>
              {drawerRecord.finished_at && (
                <Descriptions.Item label="完成时间">
                  {drawerRecord.finished_at}
                </Descriptions.Item>
              )}
            </Descriptions>

            <Card
              size="small"
              title="操作历史"
              loading={drawerLoading}
              style={{ marginBottom: 16 }}
            >
              {drawerActions.length === 0 ? (
                <Text type="secondary">暂无操作记录</Text>
              ) : (
                <Timeline
                  items={drawerActions.map((a) => ({
                    color: actionColor[a.action] || 'gray',
                    children: (
                      <div>
                        <Space size={8}>
                          <Tag color={actionColor[a.action] || 'default'}>
                            {actionLabel[a.action] || a.action}
                          </Tag>
                          <Text>第 {a.step_no} 步</Text>
                          <Text type="secondary">审批人 #{a.approver_id}</Text>
                          <Text type="secondary">{a.acted_at}</Text>
                        </Space>
                        {a.comment && (
                          <div style={{ marginTop: 4, color: '#555' }}>
                            备注: {a.comment}
                          </div>
                        )}
                      </div>
                    ),
                  }))}
                />
              )}
            </Card>

            {drawerRecord.status === 'pending' && (
              <Card size="small" title="审批操作">
                <Input.TextArea
                  rows={3}
                  placeholder="备注 (可选)"
                  value={actComment}
                  onChange={(e) => setActComment(e.target.value)}
                  style={{ marginBottom: 12 }}
                />
                <Space>
                  <Button
                    type="primary"
                    icon={<CheckCircleOutlined />}
                    loading={acting}
                    onClick={() => onAct('approve')}
                  >
                    通过
                  </Button>
                  <Button
                    danger
                    icon={<CloseCircleOutlined />}
                    loading={acting}
                    onClick={() => onAct('reject')}
                  >
                    驳回
                  </Button>
                  <Tooltip
                    title={
                      canSkip()
                        ? '跳过当前节点 (允许超时通过)'
                        : '当前节点不允许跳过'
                    }
                  >
                    <Button
                      disabled={!canSkip()}
                      loading={acting}
                      onClick={() => onAct('skip')}
                    >
                      跳过
                    </Button>
                  </Tooltip>
                </Space>
              </Card>
            )}
          </>
        )}
      </Drawer>

      {/* 提交审核 Modal */}
      <Modal
        open={submitOpen}
        title="提交审核"
        onCancel={() => setSubmitOpen(false)}
        onOk={onSubmit}
        confirmLoading={submitting}
        okText="提交"
        cancelText="取消"
        destroyOnClose
      >
        <Form form={submitForm} layout="vertical">
          <Form.Item
            name="target_type"
            label="目标类型"
            rules={[{ required: true, message: '请选择目标类型' }]}
          >
            <Select options={targetTypeOptions} placeholder="选择目标类型" />
          </Form.Item>
          <Form.Item
            name="target_id"
            label="目标视频"
            rules={[{ required: true, message: '请选择目标视频' }]}
          >
            <Select
              showSearch
              optionFilterProp="label"
              placeholder="选择完整视频"
              options={videos.map((v) => ({
                label: `${v.name} (#${v.id})`,
                value: v.id,
              }))}
            />
          </Form.Item>
          <Form.Item name="flow_id" label="审核流 (可选, 不填使用默认)">
            <Select
              allowClear
              placeholder="使用默认审核流"
              options={flows
                .filter((f) => f.enabled === 1)
                .map((f) => ({
                  label: `${f.name}${f.is_default === 1 ? ' (默认)' : ''}`,
                  value: f.id,
                }))}
            />
          </Form.Item>
          <Form.Item name="note" label="备注">
            <Input.TextArea rows={3} placeholder="提交说明 (可选)" />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
}
