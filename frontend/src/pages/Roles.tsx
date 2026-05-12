import { useEffect, useMemo, useState } from 'react';
import {
  App as AntApp,
  Button,
  Card,
  Checkbox,
  Form,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Table,
  Tag,
  Typography,
} from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';
import { Permission, Role, roleApi } from '@/api/modules';

const scopeLabels: Record<string, string> = {
  self: '自己',
  dept: '本部门',
  all: '全部',
};

export default function RolesPage() {
  const { message } = AntApp.useApp();
  const [list, setList] = useState<Role[]>([]);
  const [perms, setPerms] = useState<Permission[]>([]);
  const [loading, setLoading] = useState(false);
  const [open, setOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [editing, setEditing] = useState<Role | null>(null);
  const [form] = Form.useForm();

  const fetchAll = async () => {
    setLoading(true);
    try {
      const [rs, ps] = await Promise.all([roleApi.list(), roleApi.listPermissions()]);
      setList(rs);
      setPerms(ps);
    } catch (e) {
      message.error((e as Error)?.message || '加载角色/权限失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchAll();
  }, []);

  // 按资源分组的权限点(用于穿梭/分组)
  const permGroups = useMemo(() => {
    const m = new Map<string, Permission[]>();
    perms.forEach((p) => {
      const arr = m.get(p.resource) || [];
      arr.push(p);
      m.set(p.resource, arr);
    });
    return Array.from(m.entries());
  }, [perms]);

  const openCreate = () => {
    setEditing(null);
    form.resetFields();
    form.setFieldsValue({ data_scope: 'self' });
    setOpen(true);
  };

  const openEdit = async (r: Role) => {
    try {
      const full = await roleApi.get(r.id);
      setEditing(r);
      form.setFieldsValue({
        code: full.code,
        name: full.name,
        description: full.description,
        data_scope: full.data_scope,
        status: full.status,
        permissions: full.permissions || [],
      });
      setOpen(true);
    } catch (e) {
      message.error((e as Error)?.message || '加载角色详情失败');
    }
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
        await roleApi.update(editing.id, {
          name: v.name,
          description: v.description,
          data_scope: v.data_scope,
          status: v.status,
          permissions: v.permissions,
        });
        message.success('已更新');
      } else {
        await roleApi.create({
          code: v.code,
          name: v.name,
          description: v.description,
          data_scope: v.data_scope,
          permissions: v.permissions,
        });
        message.success('已创建');
      }
      setOpen(false);
      fetchAll();
    } catch (e) {
      message.error((e as Error)?.message || '保存失败');
    } finally {
      setSubmitting(false);
    }
  };

  const onDelete = async (r: Role) => {
    if (r.is_system === 1) {
      message.warning('系统内置角色不可删除');
      return;
    }
    try {
      await roleApi.delete(r.id);
      message.success('已删除');
      fetchAll();
    } catch (e) {
      message.error((e as Error)?.message || '删除失败');
    }
  };

  return (
    <Card
      title="角色权限"
      extra={
        <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
          新建角色
        </Button>
      }
    >
      <Table
        loading={loading}
        rowKey="id"
        dataSource={list}
        pagination={false}
        columns={[
          { title: 'ID', dataIndex: 'id', width: 60 },
          { title: '编码', dataIndex: 'code', width: 140 },
          { title: '名称', dataIndex: 'name', width: 140 },
          {
            title: '数据范围',
            dataIndex: 'data_scope',
            width: 100,
            render: (s: string) => <Tag>{scopeLabels[s] || s}</Tag>,
          },
          {
            title: '类型',
            dataIndex: 'is_system',
            width: 80,
            render: (s: number) =>
              s === 1 ? <Tag color="blue">系统</Tag> : <Tag color="default">自定义</Tag>,
          },
          { title: '描述', dataIndex: 'description' },
          {
            title: '操作',
            width: 180,
            render: (_: unknown, r: Role) => (
              <Space>
                <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(r)}>
                  编辑
                </Button>
                <Popconfirm title={`确认删除 ${r.name}?`} onConfirm={() => onDelete(r)} okType="danger">
                  <Button size="small" danger icon={<DeleteOutlined />} disabled={r.is_system === 1}>
                    删除
                  </Button>
                </Popconfirm>
              </Space>
            ),
          },
        ]}
      />

      <Modal
        title={editing ? '编辑角色' : '新建角色'}
        open={open}
        width={760}
        onCancel={() => setOpen(false)}
        onOk={onSubmit}
        confirmLoading={submitting}
        destroyOnClose
      >
        <Form form={form} layout="vertical" preserve={false}>
          <Form.Item
            name="code"
            label="编码"
            rules={[{ required: !editing, message: '请输入编码' }]}
          >
            <Input disabled={!!editing} placeholder="如 producer" />
          </Form.Item>
          <Form.Item name="name" label="名称" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea rows={2} />
          </Form.Item>
          <Form.Item name="data_scope" label="数据范围" rules={[{ required: true }]}>
            <Select
              options={[
                { label: '自己', value: 'self' },
                { label: '本部门', value: 'dept' },
                { label: '全部', value: 'all' },
              ]}
            />
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
          <Form.Item name="permissions" label="权限点" valuePropName="value">
            <PermissionMatrix groups={permGroups} />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
}

interface PermissionMatrixProps {
  groups: [string, Permission[]][];
  value?: string[];
  onChange?: (codes: string[]) => void;
}
function PermissionMatrix({ groups, value = [], onChange }: PermissionMatrixProps) {
  const toggle = (code: string, checked: boolean) => {
    const next = new Set(value);
    if (checked) next.add(code);
    else next.delete(code);
    onChange?.(Array.from(next));
  };
  return (
    <div style={{ border: '1px solid #f0f0f0', padding: 12, borderRadius: 4 }}>
      {groups.map(([resource, items]) => (
        <div key={resource} style={{ marginBottom: 12 }}>
          <Typography.Text strong style={{ marginRight: 12 }}>{resource}</Typography.Text>
          {items.map((p) => (
            <Checkbox
              key={p.code}
              checked={value.includes(p.code)}
              onChange={(e) => toggle(p.code, e.target.checked)}
              style={{ marginRight: 8 }}
            >
              {p.name}
            </Checkbox>
          ))}
        </div>
      ))}
    </div>
  );
}
