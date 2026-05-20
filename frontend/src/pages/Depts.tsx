import { useEffect, useMemo, useState } from 'react';
import {
  App as AntApp,
  Button,
  Card,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Space,
  Table,
  Tag,
  Typography,
} from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';
import { deptApi, Department } from '@/api/modules';

interface TreeDept extends Department {
  children?: TreeDept[];
}

function buildTree(list: Department[]): TreeDept[] {
  const map = new Map<number, TreeDept>();
  list.forEach((d) => map.set(d.id, { ...d, children: [] }));
  const roots: TreeDept[] = [];
  map.forEach((node) => {
    if (node.parent_id && map.has(node.parent_id)) {
      map.get(node.parent_id)!.children!.push(node);
    } else {
      roots.push(node);
    }
  });
  const sortRecursive = (arr: TreeDept[]) => {
    arr.sort((a, b) => (a.sort ?? 0) - (b.sort ?? 0) || a.id - b.id);
    arr.forEach((n) => {
      if (n.children && n.children.length) sortRecursive(n.children);
      else delete n.children;
    });
  };
  sortRecursive(roots);
  return roots;
}

export default function DeptsPage() {
  const { message } = AntApp.useApp();
  const [list, setList] = useState<Department[]>([]);
  const [loading, setLoading] = useState(false);
  const [open, setOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [editing, setEditing] = useState<Department | null>(null);
  const [form] = Form.useForm();
  const [formKey, setFormKey] = useState('create');
  const [formInitialValues, setFormInitialValues] = useState<Record<string, unknown>>({ sort: 0 });

  const fetchList = async () => {
    setLoading(true);
    try {
      const data = await deptApi.list();
      // 防护: data 为 undefined 时避免炸
      setList(data ?? []);
    } catch (e) {
      message.error((e as Error)?.message || '加载部门失败');
      setList([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchList();
  }, []);

  const tree = useMemo(() => buildTree(list), [list]);

  const openCreate = (parent?: Department) => {
    setEditing(null);
    setFormKey(`create-${Date.now()}`);
    const vals: Record<string, unknown> = { sort: 0 };
    if (parent) vals.parent_id = parent.id;
    setFormInitialValues(vals);
    setOpen(true);
  };

  const openEdit = (d: Department) => {
    setEditing(d);
    setFormKey(`edit-${d.id}`);
    setFormInitialValues({
      name: d.name,
      parent_id: d.parent_id || undefined,
      sort: d.sort,
      status: d.status,
    });
    setOpen(true);
  };

  const handleModalAfterOpenChange = (visible: boolean) => {
    if (!visible) return;
    requestAnimationFrame(() => {
      form.resetFields();
      form.setFieldsValue(formInitialValues);
    });
  };

  const onSubmit = async () => {
    let v;
    try {
      v = await form.validateFields();
    } catch {
      return;
    }
    setSubmitting(true);
    try {
      if (editing) {
        await deptApi.update(editing.id, {
          name: v.name,
          sort: v.sort,
          status: v.status,
        });
        message.success('已更新');
      } else {
        await deptApi.create({
          name: v.name,
          parent_id: v.parent_id || 0,
          sort: v.sort || 0,
        });
        message.success('已创建');
      }
      setOpen(false);
      fetchList();
    } catch (e) {
      message.error((e as Error)?.message || '保存失败');
    } finally {
      setSubmitting(false);
    }
  };

  const onDelete = async (d: Department) => {
    try {
      await deptApi.delete(d.id);
      message.success('已删除');
      fetchList();
    } catch (e) {
      message.error((e as Error)?.message || '删除失败,请确认无下属用户/项目');
    }
  };

  return (
    <Card
      title="部门管理"
      extra={
        <Button type="primary" icon={<PlusOutlined />} onClick={() => openCreate()}>
          新建部门
        </Button>
      }
    >
      <Typography.Paragraph type="secondary" style={{ marginTop: 0 }}>
        部门用于划分数据范围 (data_scope=dept)。删除部门前请确认无下属用户/项目。
      </Typography.Paragraph>
      <Table<TreeDept>
        loading={loading}
        rowKey="id"
        dataSource={tree}
        pagination={false}
        defaultExpandAllRows
        columns={[
          { title: 'ID', dataIndex: 'id', width: 60 },
          { title: '名称', dataIndex: 'name' },
          { title: '路径', dataIndex: 'path', width: 200 },
          { title: '排序', dataIndex: 'sort', width: 80 },
          {
            title: '状态',
            dataIndex: 'status',
            width: 90,
            render: (s: number) =>
              s === 1 ? <Tag color="success">启用</Tag> : <Tag>禁用</Tag>,
          },
          {
            title: '操作',
            width: 260,
            render: (_: unknown, d: TreeDept) => (
              <Space>
                <Button size="small" icon={<PlusOutlined />} onClick={() => openCreate(d)}>
                  子部门
                </Button>
                <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(d)}>
                  编辑
                </Button>
                <Popconfirm
                  title={`确认删除 ${d.name}?`}
                  onConfirm={() => onDelete(d)}
                  okType="danger"
                >
                  <Button size="small" danger icon={<DeleteOutlined />}>
                    删除
                  </Button>
                </Popconfirm>
              </Space>
            ),
          },
        ]}
      />

      <Modal
        title={editing ? `编辑部门 - ${editing.name}` : '新建部门'}
        open={open}
        onCancel={() => setOpen(false)}
        onOk={onSubmit}
        confirmLoading={submitting}
        destroyOnClose
        afterOpenChange={handleModalAfterOpenChange}
        width={520}
      >
        <Form key={formKey} form={form} layout="vertical" preserve={false}>
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input />
          </Form.Item>
          <Form.Item name="parent_id" label="上级部门">
            <Select
              allowClear
              disabled={!!editing}
              placeholder="选择上级部门(留空为根部门)"
              options={list
                .filter((d) => !editing || d.id !== editing.id)
                .map((d) => ({
                  label: `${d.name} (id:${d.id})`,
                  value: d.id,
                }))}
            />
          </Form.Item>
          <Form.Item name="sort" label="排序" tooltip="数字越小越靠前">
            <InputNumber min={0} max={9999} style={{ width: '100%' }} />
          </Form.Item>
          {editing && (
            <Form.Item name="status" label="状态">
              <Select
                options={[
                  { label: '启用', value: 1 },
                  { label: '禁用', value: 2 },
                ]}
              />
            </Form.Item>
          )}
        </Form>
      </Modal>
    </Card>
  );
}
