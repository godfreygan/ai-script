import { useCallback, useEffect, useMemo, useState } from 'react';
import ReactFlow, {
  Background,
  Controls,
  MiniMap,
  ReactFlowProvider,
  Node,
  Edge,
  addEdge,
  Connection,
  useNodesState,
  useEdgesState,
} from 'reactflow';
import 'reactflow/dist/style.css';
import {
  App as AntApp,
  Button,
  Card,
  Col,
  Drawer,
  Empty,
  Form,
  Input,
  List,
  Modal,
  Progress,
  Row,
  Select,
  Space,
  Spin,
  Table,
  Tag,
  Timeline,
  Typography,
} from 'antd';
import {
  HistoryOutlined,
  PlayCircleOutlined,
  PlusOutlined,
  ReloadOutlined,
  SaveOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import {
  modelApi,
  pipelineApi,
  projectApi,
  type Model,
  type Pipeline,
  type PipelineRun,
  type StepRun,
  type Project,
} from '@/api/modules';
import { useProgressWS } from '@/hooks/useProgressWS';

const NODE_TYPES = [
  { value: 'script.split', label: '剧本拆分' },
  { value: 'prompt.generate', label: '提示词生成' },
  { value: 'storyboard.generate', label: '分镜生成' },
  { value: 'style.apply', label: '风格套用' },
  { value: 'image.generate', label: '图片生成' },
  { value: 'image.upload', label: '图片上传' },
  { value: 'video.generate', label: '短视频生成' },
  { value: 'audio.tts', label: '配音 TTS' },
  { value: 'video.compose', label: '视频合成' },
  { value: 'review.submit', label: '提交审核' },
  { value: 'human.approve', label: '人工节点' },
];

const NODE_TYPE_LABELS: Record<string, string> = NODE_TYPES.reduce(
  (acc, cur) => ({ ...acc, [cur.value]: cur.label }),
  {} as Record<string, string>,
);

/** 节点类型对应的模型 type 筛选；无映射时展示全部已启用模型 */
const NODE_TYPE_MODEL_TYPE: Record<string, string | undefined> = {
  'script.split': 'text',
  'prompt.generate': 'text',
  'storyboard.generate': 'text',
  'style.apply': 'text',
  'image.generate': 'image',
  'image.upload': 'image',
  'video.generate': 'video',
  'video.compose': 'video',
  'audio.tts': 'audio',
};

function filterModelsForNodeType(nodeType: string | undefined, models: Model[]): Model[] {
  const enabled = models.filter((m) => m.enabled === 1);
  if (!nodeType) return enabled;
  const want = NODE_TYPE_MODEL_TYPE[nodeType];
  if (!want) return enabled;
  return enabled.filter((m) => m.type === want);
}

function modelSelectLabel(m: Model): string {
  return `${m.name} (${m.code})`;
}

function nodeFlowLabel(type: string, modelId: number | undefined, modelList: Model[]): string {
  const base = NODE_TYPE_LABELS[type] || type;
  if (!modelId) return base;
  const m = modelList.find((x) => x.id === modelId);
  return m ? `${base} · ${m.name}` : `${base} · #${modelId}`;
}

type DAGNode = {
  id: string;
  type: string;
  model_id?: number;
  params?: Record<string, unknown>;
  position?: { x: number; y: number };
};

type DAGEdge = {
  from: string;
  to: string;
  mapping?: Record<string, string>;
};

type DAG = {
  nodes: DAGNode[];
  edges: DAGEdge[];
};

const EMPTY_DAG: DAG = { nodes: [], edges: [] };

function decodeDAG(dag: Pipeline['dag']): DAG {
  if (!dag) return { ...EMPTY_DAG };
  try {
    const obj = typeof dag === 'string' ? JSON.parse(dag) : (dag as unknown);
    if (obj && typeof obj === 'object') {
      const o = obj as Partial<DAG>;
      return {
        nodes: Array.isArray(o.nodes) ? (o.nodes as DAGNode[]) : [],
        edges: Array.isArray(o.edges) ? (o.edges as DAGEdge[]) : [],
      };
    }
  } catch (err) {
    // eslint-disable-next-line no-console
    console.error('parse DAG failed:', err);
  }
  return { ...EMPTY_DAG };
}

function dagToFlow(dag: DAG): { nodes: Node[]; edges: Edge[] } {
  const nodes: Node[] = (dag.nodes || []).map((n, idx) => ({
    id: n.id,
    position: n.position || { x: 60 + idx * 220, y: 80 + (idx % 2) * 40 },
    data: {
      label: NODE_TYPE_LABELS[n.type] || n.type,
      type: n.type,
      model_id: n.model_id,
      params: n.params || {},
    },
    type: 'default',
  }));
  const edges: Edge[] = (dag.edges || []).map((e, idx) => ({
    id: `e-${e.from}-${e.to}-${idx}`,
    source: e.from,
    target: e.to,
    data: { mapping: e.mapping || {} },
  }));
  return { nodes, edges };
}

function flowToDAG(nodes: Node[], edges: Edge[]): DAG {
  return {
    nodes: nodes.map((n) => ({
      id: n.id,
      type: (n.data?.type as string) || 'unknown',
      model_id: n.data?.model_id as number | undefined,
      params: (n.data?.params as Record<string, unknown>) || {},
      position: n.position,
    })),
    edges: edges.map((e) => ({
      from: e.source,
      to: e.target,
      mapping: (e.data?.mapping as Record<string, string>) || {},
    })),
  };
}

const runStatusColor: Record<string, string> = {
  pending: 'default',
  running: 'processing',
  succeeded: 'success',
  success: 'success',
  done: 'success',
  failed: 'error',
  error: 'error',
  canceled: 'warning',
  cancelled: 'warning',
};

function statusTag(status?: string) {
  if (!status) return <Tag>未知</Tag>;
  return <Tag color={runStatusColor[status] || 'default'}>{status}</Tag>;
}

export default function PipelinesPage() {
  const { message, modal } = AntApp.useApp();

  // ============ 列表 ============
  const [pipelines, setPipelines] = useState<Pipeline[]>([]);
  const [listLoading, setListLoading] = useState(false);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [selectedPipeline, setSelectedPipeline] = useState<Pipeline | null>(null);
  const [pipelineLoading, setPipelineLoading] = useState(false);

  // ============ 项目下拉 ============
  const [projects, setProjects] = useState<Project[]>([]);

  // ============ 模型列表（节点配置下拉） ============
  const [models, setModels] = useState<Model[]>([]);
  const [modelsLoading, setModelsLoading] = useState(false);

  // ============ DAG 画布 ============
  type NodeData = {
    label?: string;
    type?: string;
    model_id?: number;
    params?: Record<string, unknown>;
  };
  const [nodes, setNodes, onNodesChange] = useNodesState<NodeData>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState([]);

  // ============ 节点编辑 ============
  const [editorOpen, setEditorOpen] = useState(false);
  const [editorTarget, setEditorTarget] = useState<Node | null>(null);
  const [editorForm] = Form.useForm();
  const editorNodeType = Form.useWatch('type', editorForm);

  // ============ 新建 pipeline ============
  const [createOpen, setCreateOpen] = useState(false);
  const [createForm] = Form.useForm();

  // ============ 添加节点 ============
  const [addNodeOpen, setAddNodeOpen] = useState(false);
  const [addNodeForm] = Form.useForm();

  // ============ 运行 ============
  const [runOpen, setRunOpen] = useState(false);
  const [runForm] = Form.useForm();

  // ============ 历史 Drawer ============
  const [historyOpen, setHistoryOpen] = useState(false);
  const [runs, setRuns] = useState<PipelineRun[]>([]);
  const [runsLoading, setRunsLoading] = useState(false);
  const [activeRun, setActiveRun] = useState<PipelineRun | null>(null);
  const [activeSteps, setActiveSteps] = useState<StepRun[]>([]);
  const [activeStepsLoading, setActiveStepsLoading] = useState(false);

  // ============ WS topic ============
  const [runTopic, setRunTopic] = useState<string | null>(null);
  const [activeRunId, setActiveRunId] = useState<number | null>(null);
  const { last, connected } = useProgressWS(runTopic);

  const percent = useMemo(() => {
    if (!last || typeof last.percent !== 'number') return 0;
    return Math.min(100, Math.round(last.percent * 100));
  }, [last]);

  // ============ 加载列表 ============
  const refreshList = useCallback(async () => {
    setListLoading(true);
    try {
      const page = await pipelineApi.list({ page: 1, page_size: 200 });
      // 防护: 接口返回 undefined 时避免炸
      setPipelines(page?.list ?? []);
      if (!selectedId && page?.list && page.list.length > 0) {
        setSelectedId(page.list[0].id);
      }
    } catch (e) {
      message.error((e as Error).message || '加载流水线失败');
    } finally {
      setListLoading(false);
    }
  }, [message, selectedId]);

  const fetchModels = useCallback(async () => {
    setModelsLoading(true);
    try {
      const page = await modelApi.list({ page: 1, page_size: 500, enabled: 1 });
      // 防护: 接口返回 undefined 时避免炸
      setModels(page?.list ?? []);
    } catch (e) {
      message.error((e as Error).message || '加载模型列表失败');
    } finally {
      setModelsLoading(false);
    }
  }, [message]);

  useEffect(() => {
    refreshList();
    projectApi
      .list({ page: 1, page_size: 100 })
      .then((p) => setProjects(p?.list ?? []))
      .catch(() => setProjects([]));
    fetchModels();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const editorModelOptions = useMemo(() => {
    const list = filterModelsForNodeType(editorNodeType, models);
    return list.map((m) => ({
      value: m.id,
      label: modelSelectLabel(m),
    }));
  }, [editorNodeType, models]);

  // ============ 加载选中详情 ============
  useEffect(() => {
    if (!selectedId) {
      setSelectedPipeline(null);
      setNodes([]);
      setEdges([]);
      return;
    }
    setPipelineLoading(true);
    pipelineApi
      .get(selectedId)
      .then((p) => {
        setSelectedPipeline(p);
        const dag = decodeDAG(p.dag);
        const flow = dagToFlow(dag);
        setNodes(flow.nodes);
        setEdges(flow.edges);
      })
      .catch((e) => message.error((e as Error).message || '加载流水线详情失败'))
      .finally(() => setPipelineLoading(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedId]);

  // ============ WS 完成 / 失败 ============
  useEffect(() => {
    if (!last) return;
    if (last.type === 'done') {
      message.success(last.message || '流水线执行完成');
      setRunTopic(null);
      if (activeRunId) {
        refreshSteps(activeRunId);
      }
      refreshRuns();
    } else if (last.type === 'error') {
      message.error(last.message || '流水线执行失败');
      setRunTopic(null);
      if (activeRunId) {
        refreshSteps(activeRunId);
      }
      refreshRuns();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [last]);

  // ============ DAG 操作 ============
  const onConnect = useCallback(
    (c: Connection) => setEdges((eds) => addEdge({ ...c, data: { mapping: {} } }, eds)),
    [setEdges],
  );

  useEffect(() => {
    if (!editorOpen || !editorTarget) return;
    const params = (editorTarget.data?.params as Record<string, unknown>) || {};
    editorForm.setFieldsValue({
      type: editorTarget.data?.type,
      model_id: editorTarget.data?.model_id,
      params: Object.keys(params).length > 0 ? JSON.stringify(params, null, 2) : '',
    });
  }, [editorOpen, editorTarget, editorForm]);

  const onNodeClick = (_: unknown, n: Node) => {
    setEditorTarget(n);
    setEditorOpen(true);
  };

  const submitEditor = async () => {
    try {
      const values = await editorForm.validateFields();
      let params: Record<string, unknown> = {};
      if (values.params && String(values.params).trim()) {
        try {
          params = JSON.parse(values.params);
        } catch {
          message.error('参数必须是合法 JSON');
          return;
        }
      }
      const targetId = editorTarget?.id;
      const modelId = values.model_id ? Number(values.model_id) : undefined;
      setNodes((ns) =>
        ns.map((n) =>
          n.id === targetId
            ? {
                ...n,
                data: {
                  ...n.data,
                  type: values.type,
                  label: nodeFlowLabel(values.type, modelId, models),
                  model_id: modelId,
                  params,
                },
              }
            : n,
        ),
      );
      setEditorOpen(false);
    } catch {
      /* 表单校验失败 */
    }
  };

  const submitAddNode = async () => {
    try {
      const values = await addNodeForm.validateFields();
      const id = `n${Date.now().toString(36)}`;
      const idx = nodes.length;
      const newNode: Node = {
        id,
        position: { x: 60 + idx * 200, y: 80 + (idx % 3) * 90 },
        data: {
          type: values.type,
          label: NODE_TYPE_LABELS[values.type] || values.type,
          model_id: undefined,
          params: {},
        },
        type: 'default',
      };
      setNodes((ns) => [...ns, newNode]);
      setAddNodeOpen(false);
      addNodeForm.resetFields();
    } catch {
      /* skip */
    }
  };

  // ============ 新建 pipeline ============
  const submitCreate = async () => {
    try {
      const values = await createForm.validateFields();
      const p = await pipelineApi.create({
        project_id: Number(values.project_id),
        name: values.name,
        description: values.description || '',
        dag: { nodes: [], edges: [] },
      });
      message.success('已创建');
      setCreateOpen(false);
      createForm.resetFields();
      await refreshList();
      setSelectedId(p?.id ?? null);
    } catch (e) {
      if ((e as { errorFields?: unknown }).errorFields) return;
      message.error((e as Error).message || '创建失败');
    }
  };

  // ============ 保存 ============
  const onSave = async () => {
    if (!selectedId) {
      message.warning('请先选择流水线');
      return;
    }
    const dag = flowToDAG(nodes, edges);
    try {
      await pipelineApi.update(selectedId, { dag });
      message.success('已保存');
      const p = await pipelineApi.get(selectedId);
      setSelectedPipeline(p);
    } catch (e) {
      message.error((e as Error).message || '保存失败');
    }
  };

  // ============ 运行 ============
  const openRun = () => {
    if (!selectedId) {
      message.warning('请先选择流水线');
      return;
    }
    runForm.setFieldsValue({ input: '' });
    setRunOpen(true);
  };

  const submitRun = async () => {
    if (!selectedId) return;
    try {
      const values = await runForm.validateFields();
      let input: Record<string, unknown> = {};
      if (values.input && String(values.input).trim()) {
        try {
          input = JSON.parse(values.input);
        } catch {
          message.error('input 必须是合法 JSON');
          return;
        }
      }
      const res = await pipelineApi.run(selectedId, { input });
      // 防护: res 为 undefined 时避免炸
      const runId = res?.run_id;
      if (!runId) {
        message.error('运行返回异常,未获取到 run_id');
        return;
      }
      message.success(`已提交,run_id=${runId}`);
      setRunOpen(false);
      setActiveRunId(Number(runId));
      setRunTopic(`pipeline:${runId}`);
      setHistoryOpen(true);
      await refreshRuns();
      await refreshSteps(Number(runId));
    } catch (e) {
      if ((e as { errorFields?: unknown }).errorFields) return;
      message.error((e as Error).message || '运行失败');
    }
  };

  // ============ 历史 ============
  const refreshRuns = useCallback(async () => {
    if (!selectedId) return;
    setRunsLoading(true);
    try {
      const page = await pipelineApi.listRuns(selectedId, { page: 1, page_size: 50 });
      // 防护: 接口返回 undefined 时避免炸
      setRuns(page?.list ?? []);
    } catch (e) {
      message.error((e as Error).message || '加载运行记录失败');
    } finally {
      setRunsLoading(false);
    }
  }, [selectedId, message]);

  const refreshSteps = useCallback(
    async (runId: number) => {
      setActiveStepsLoading(true);
      try {
        const [run, steps] = await Promise.all([
          pipelineApi.getRun(runId),
          pipelineApi.listSteps(runId),
        ]);
        setActiveRun(run ?? null);
        // 防护: steps 为 undefined 时避免炸
        setActiveSteps(steps ?? []);
      } catch (e) {
        message.error((e as Error).message || '加载步骤失败');
      } finally {
        setActiveStepsLoading(false);
      }
    },
    [message],
  );

  const openHistory = async () => {
    if (!selectedId) {
      message.warning('请先选择流水线');
      return;
    }
    setHistoryOpen(true);
    await refreshRuns();
  };

  const viewRun = async (run: PipelineRun) => {
    setActiveRunId(run.id);
    await refreshSteps(run.id);
    if (run.status === 'running' || run.status === 'pending') {
      setRunTopic(`pipeline:${run.id}`);
    }
  };

  // ============ 删除 pipeline ============
  const onDelete = (p: Pipeline) => {
    modal.confirm({
      title: `删除流水线"${p.name}"?`,
      okType: 'danger',
      onOk: async () => {
        try {
          await pipelineApi.delete(p.id);
          message.success('已删除');
          if (selectedId === p.id) setSelectedId(null);
          refreshList();
        } catch (e) {
          message.error((e as Error).message || '删除失败');
        }
      },
    });
  };

  const runColumns: ColumnsType<PipelineRun> = [
    { title: 'ID', dataIndex: 'id', width: 70 },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (s: string) => statusTag(s),
    },
    {
      title: '开始',
      dataIndex: 'started_at',
      width: 160,
      render: (v?: string) => v || '-',
    },
    {
      title: '结束',
      dataIndex: 'ended_at',
      width: 160,
      render: (v?: string) => v || '-',
    },
    {
      title: '错误',
      dataIndex: 'error_msg',
      ellipsis: true,
      render: (v?: string) => v || '-',
    },
    {
      title: '操作',
      width: 120,
      render: (_: unknown, r: PipelineRun) => (
        <Button size="small" type="link" onClick={() => viewRun(r)}>
          查看
        </Button>
      ),
    },
  ];

  return (
    <Card
      title="流水线编排"
      extra={
        <Space>
          <Button icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
            新建
          </Button>
          <Button icon={<SaveOutlined />} onClick={onSave} disabled={!selectedId}>
            保存
          </Button>
          <Button
            type="primary"
            icon={<PlayCircleOutlined />}
            onClick={openRun}
            disabled={!selectedId}
          >
            运行
          </Button>
          <Button icon={<HistoryOutlined />} onClick={openHistory} disabled={!selectedId}>
            历史
          </Button>
        </Space>
      }
      bodyStyle={{ padding: 0, height: 'calc(100vh - 220px)' }}
    >
      <Row style={{ height: '100%' }} wrap={false}>
        <Col
          flex="260px"
          style={{
            borderRight: '1px solid #f0f0f0',
            height: '100%',
            overflow: 'auto',
            background: '#fafafa',
          }}
        >
          <div style={{ padding: '8px 12px', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <Typography.Text strong>流水线列表</Typography.Text>
            <Button size="small" type="text" icon={<ReloadOutlined />} onClick={refreshList} />
          </div>
          <List
            loading={listLoading}
            dataSource={pipelines}
            locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无流水线" /> }}
            renderItem={(p) => (
              <List.Item
                style={{
                  padding: '8px 12px',
                  cursor: 'pointer',
                  background: selectedId === p.id ? '#e6f4ff' : undefined,
                  borderLeft: selectedId === p.id ? '3px solid #1677ff' : '3px solid transparent',
                }}
                onClick={() => setSelectedId(p.id)}
                actions={[
                  <a
                    key="del"
                    onClick={(e) => {
                      e.stopPropagation();
                      onDelete(p);
                    }}
                  >
                    删除
                  </a>,
                ]}
              >
                <List.Item.Meta
                  title={
                    <span>
                      {p.name}{' '}
                      {p.enabled === 0 && <Tag color="default">禁用</Tag>}
                      {p.is_template === 1 && <Tag color="purple">模板</Tag>}
                    </span>
                  }
                  description={
                    <Typography.Text type="secondary" ellipsis style={{ fontSize: 12 }}>
                      #{p.id} · project={p.project_id}
                    </Typography.Text>
                  }
                />
              </List.Item>
            )}
          />
        </Col>

        <Col flex="auto" style={{ position: 'relative', height: '100%' }}>
          {pipelineLoading && (
            <div
              style={{
                position: 'absolute',
                top: 12,
                left: 12,
                zIndex: 10,
                background: 'rgba(255,255,255,0.8)',
                padding: '4px 12px',
                borderRadius: 4,
              }}
            >
              <Spin size="small" /> 加载中...
            </div>
          )}
          {selectedId ? (
            <>
              <div
                style={{
                  position: 'absolute',
                  top: 12,
                  right: 12,
                  zIndex: 10,
                  display: 'flex',
                  gap: 8,
                }}
              >
                <Button size="small" icon={<PlusOutlined />} onClick={() => setAddNodeOpen(true)}>
                  添加节点
                </Button>
              </div>
              {runTopic && (
                <div
                  style={{
                    position: 'absolute',
                    top: 12,
                    left: '50%',
                    transform: 'translateX(-50%)',
                    zIndex: 10,
                    background: '#fff',
                    border: '1px solid #d9d9d9',
                    padding: '6px 14px',
                    borderRadius: 4,
                    minWidth: 320,
                    boxShadow: '0 2px 8px rgba(0,0,0,0.08)',
                  }}
                >
                  <Space>
                    <Tag color={connected ? 'green' : 'red'}>{connected ? '已连接' : '未连接'}</Tag>
                    <Typography.Text style={{ fontSize: 12 }}>{runTopic}</Typography.Text>
                  </Space>
                  <Progress percent={percent} size="small" status={last?.type === 'error' ? 'exception' : undefined} />
                  {last?.message && (
                    <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                      {last.message}
                    </Typography.Text>
                  )}
                </div>
              )}
              <ReactFlowProvider>
                <ReactFlow
                  nodes={nodes}
                  edges={edges}
                  onNodesChange={onNodesChange}
                  onEdgesChange={onEdgesChange}
                  onConnect={onConnect}
                  onNodeClick={onNodeClick}
                  fitView
                >
                  <Background />
                  <MiniMap />
                  <Controls />
                </ReactFlow>
              </ReactFlowProvider>
            </>
          ) : (
            <div style={{ paddingTop: 80, textAlign: 'center' }}>
              <Empty description="请选择左侧流水线或新建一个" />
            </div>
          )}
        </Col>
      </Row>

      {/* 节点配置 Drawer */}
      <Drawer
        title={`节点配置 ${editorTarget?.id || ''}`}
        open={editorOpen}
        onClose={() => setEditorOpen(false)}
        width={420}
        destroyOnClose
        footer={
          <Space style={{ float: 'right' }}>
            <Button onClick={() => setEditorOpen(false)}>取消</Button>
            <Button type="primary" onClick={submitEditor}>
              确定
            </Button>
          </Space>
        }
      >
        <Form layout="vertical" form={editorForm}>
          <Form.Item name="type" label="节点类型" rules={[{ required: true, message: '请选择类型' }]}>
            <Select
              options={NODE_TYPES}
              onChange={() => {
                editorForm.setFieldValue('model_id', undefined);
              }}
            />
          </Form.Item>
          <Form.Item
            name="model_id"
            label="模型"
            tooltip={
              editorNodeType && NODE_TYPE_MODEL_TYPE[editorNodeType]
                ? `仅显示类型为 ${NODE_TYPE_MODEL_TYPE[editorNodeType]} 且已启用的模型`
                : '显示全部已启用模型；审核/人工节点通常无需模型'
            }
          >
            <Select
              allowClear
              showSearch
              placeholder="选择模型（选填）"
              loading={modelsLoading}
              options={editorModelOptions}
              notFoundContent={modelsLoading ? <Spin size="small" /> : '暂无可用模型，请先在模型管理中注册'}
              filterOption={(input, opt) =>
                String(opt?.label || '').toLowerCase().includes(input.toLowerCase())
              }
            />
          </Form.Item>
          <Form.Item name="params" label="参数 (JSON)">
            <Input.TextArea rows={8} placeholder='{"width":1024,"height":1024}' />
          </Form.Item>
        </Form>
      </Drawer>

      {/* 新建 pipeline Modal */}
      <Modal
        title="新建流水线"
        open={createOpen}
        onCancel={() => setCreateOpen(false)}
        onOk={submitCreate}
        destroyOnClose
      >
        <Form layout="vertical" form={createForm}>
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input placeholder="如: 默认短剧流水线" />
          </Form.Item>
          <Form.Item
            name="project_id"
            label="所属项目"
            rules={[{ required: true, message: '请选择项目' }]}
          >
            <Select
              showSearch
              placeholder="选择项目"
              options={projects.map((p) => ({ value: p.id, label: `${p.name} (#${p.id})` }))}
              filterOption={(input, opt) =>
                String(opt?.label || '').toLowerCase().includes(input.toLowerCase())
              }
            />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea rows={2} />
          </Form.Item>
        </Form>
      </Modal>

      {/* 添加节点 Modal */}
      <Modal
        title="添加节点"
        open={addNodeOpen}
        onCancel={() => setAddNodeOpen(false)}
        onOk={submitAddNode}
        destroyOnClose
      >
        <Form layout="vertical" form={addNodeForm}>
          <Form.Item
            name="type"
            label="节点类型"
            rules={[{ required: true, message: '请选择类型' }]}
          >
            <Select options={NODE_TYPES} />
          </Form.Item>
        </Form>
      </Modal>

      {/* 运行 Modal */}
      <Modal
        title="运行流水线"
        open={runOpen}
        onCancel={() => setRunOpen(false)}
        onOk={submitRun}
        destroyOnClose
      >
        <Form layout="vertical" form={runForm}>
          <Form.Item name="input" label="输入 (JSON, 可空)">
            <Input.TextArea
              rows={6}
              placeholder='{"script_id":1, "project_id":1}'
            />
          </Form.Item>
        </Form>
      </Modal>

      {/* 历史 Drawer */}
      <Drawer
        title={`运行历史 ${selectedPipeline ? `· ${selectedPipeline.name}` : ''}`}
        open={historyOpen}
        onClose={() => setHistoryOpen(false)}
        width={860}
        extra={
          <Button size="small" icon={<ReloadOutlined />} onClick={refreshRuns}>
            刷新
          </Button>
        }
      >
        {runTopic && (
          <Card size="small" style={{ marginBottom: 12 }}>
            <Space direction="vertical" style={{ width: '100%' }}>
              <Space>
                <Tag color={connected ? 'green' : 'red'}>{connected ? '已连接' : '未连接'}</Tag>
                <Typography.Text>{runTopic}</Typography.Text>
              </Space>
              <Progress
                percent={percent}
                status={last?.type === 'error' ? 'exception' : undefined}
              />
              {last?.message && (
                <Typography.Text type="secondary">{last.message}</Typography.Text>
              )}
            </Space>
          </Card>
        )}

        <Table
          rowKey="id"
          size="small"
          loading={runsLoading}
          columns={runColumns}
          dataSource={runs}
          pagination={{ pageSize: 10 }}
        />

        {activeRun && (
          <Card
            size="small"
            title={
              <Space>
                <span>Run #{activeRun.id}</span>
                {statusTag(activeRun.status)}
              </Space>
            }
            style={{ marginTop: 16 }}
            extra={
              <Button
                size="small"
                icon={<ReloadOutlined />}
                onClick={() => refreshSteps(activeRun.id)}
              >
                刷新步骤
              </Button>
            }
          >
            {activeStepsLoading ? (
              <Spin />
            ) : activeSteps.length === 0 ? (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无步骤" />
            ) : (
              <Timeline
                items={activeSteps.map((s) => ({
                  color:
                    s.status === 'succeeded' || s.status === 'success' || s.status === 'done'
                      ? 'green'
                      : s.status === 'failed' || s.status === 'error'
                        ? 'red'
                        : s.status === 'running'
                          ? 'blue'
                          : 'gray',
                  children: (
                    <Space direction="vertical" size={2}>
                      <Space>
                        <Typography.Text strong>{s.node_id}</Typography.Text>
                        <Tag>{NODE_TYPE_LABELS[s.node_type] || s.node_type}</Tag>
                        {statusTag(s.status)}
                        {s.attempt > 1 && <Tag color="orange">重试 {s.attempt}</Tag>}
                      </Space>
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        {s.started_at || '-'} ~ {s.ended_at || '-'}
                      </Typography.Text>
                      {s.error_msg && (
                        <Typography.Text type="danger" style={{ fontSize: 12 }}>
                          {s.error_msg}
                        </Typography.Text>
                      )}
                    </Space>
                  ),
                }))}
              />
            )}
          </Card>
        )}
      </Drawer>
    </Card>
  );
}
