import { useEffect, useState } from 'react';
import {
  App as AntApp,
  Button,
  Card,
  Drawer,
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
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  TeamOutlined,
  UserAddOutlined,
} from '@ant-design/icons';
import {
  deptApi,
  Department,
  Project,
  ProjectMember,
  projectApi,
  User,
  userApi,
} from '@/api/modules';

const statusMap: Record<number, { color: string; label: string }> = {
  0: { color: 'default', label: '草稿' },
  1: { color: 'processing', label: '进行中' },
  2: { color: 'success', label: '已完成' },
  3: { color: 'error', label: '已归档' },
};

const roleInProjectOptions = [
  { label: '负责人 (owner)', value: 'owner' },
  { label: '编辑 (editor)', value: 'editor' },
  { label: '审核 (reviewer)', value: 'reviewer' },
  { label: '只读 (viewer)', value: 'viewer' },
];

export default function ProjectsPage() {
  const { message } = AntApp.useApp();
  const [list, setList] = useState<Project[]>([]);
  const [depts, setDepts] = useState<Department[]>([]);
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [q, setQ] = useState('');
  const [statusFilter, setStatusFilter] = useState<number | undefined>();

  const [open, setOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [editing, setEditing] = useState<Project | null>(null);
  const [form] = Form.useForm();

  const [memberDrawerOpen, setMemberDrawerOpen] = useState(false);
  const [memberTarget, setMemberTarget] = useState<Project | null>(null);
  const [members, setMembers] = useState<ProjectMember[]>([]);
  const [memberLoading, setMemberLoading] = useState(false);
  const [memberSubmitting, setMemberSubmitting] = useState(false);
  const [addMemberForm] = Form.useForm();

  const fetchList = async () => {
    setLoading(true);
    try {
      const data = await projectApi.list({
        page,
        page_size: pageSize,
        q,
        status: statusFilter,
      });
      setList(data.list);
      setTotal(data.total);
    } catch (e) {
      message.error((e as Error)?.message || '加载项目列表失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchList();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, pageSize, statusFilter, q]);

  useEffect(() => {
    deptApi.list().then(setDepts).catch((e) => message.error((e as Error)?.message || '加载部门失败'));
    userApi
      .list({ page: 1, page_size: 200 })
      .then((p) => setUsers(p.list))
      .catch((e) => message.error((e as Error)?.message || '加载用户失败'));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const userName = (id: number) =>
    users.find((u) => u.id === id)?.nickname ||
    users.find((u) => u.id === id)?.username ||
    `#${id}`;
  const deptName = (id: number) => depts.find((d) => d.id === id)?.name || '-';

  const openCreate = () => {
    setEditing(null);
    form.resetFields();
    setOpen(true);
  };

  const openEdit = (p: Project) => {
    setEditing(p);
    form.setFieldsValue({
      code: p.code,
      name: p.name,
      description: p.description,
      status: p.status,
      dept_id: p.dept_id || undefined,
      cover_url: p.cover_url,
    });
    setOpen(true);
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
        await projectApi.update(editing.id, {
          name: v.name,
          description: v.description,
          status: v.status,
          cover_url: v.cover_url,
        });
        message.success('已更新');
      } else {
        await projectApi.create({
          code: v.code,
          name: v.name,
          description: v.description,
          dept_id: v.dept_id,
          cover_url: v.cover_url,
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

  const onDelete = async (p: Project) => {
    try {
      await projectApi.delete(p.id);
      message.success('已删除');
      fetchList();
    } catch (e) {
      message.error((e as Error)?.message || '删除失败');
    }
  };

  const openMemberDrawer = async (p: Project) => {
    setMemberTarget(p);
    setMemberDrawerOpen(true);
    addMemberForm.resetFields();
    setMemberLoading(true);
    try {
      const data = await projectApi.listMembers(p.id);
      setMembers(data);
    } catch (e) {
      message.error((e as Error)?.message || '加载成员失败');
    } finally {
      setMemberLoading(false);
    }
  };

  const refreshMembers = async () => {
    if (!memberTarget) return;
    setMemberLoading(true);
    try {
      const data = await projectApi.listMembers(memberTarget.id);
      setMembers(data);
    } catch (e) {
      message.error((e as Error)?.message || '刷新成员失败');
    } finally {
      setMemberLoading(false);
    }
  };

  const onAddMember = async (v: { user_id: number; role_in_project: string }) => {
    if (!memberTarget) return;
    setMemberSubmitting(true);
    try {
      await projectApi.addMember(memberTarget.id, v.user_id, v.role_in_project);
      message.success('成员已添加');
      addMemberForm.resetFields();
      refreshMembers();
    } catch (e) {
      message.error((e as Error)?.message || '添加失败');
    } finally {
      setMemberSubmitting(false);
    }
  };

  const onRemoveMember = async (uid: number) => {
    if (!memberTarget) return;
    try {
      await projectApi.removeMember(memberTarget.id, uid);
      message.success('已移除');
      refreshMembers();
    } catch (e) {
      message.error((e as Error)?.message || '移除失败');
    }
  };

  return (
    <Card
      title="项目管理"
      extra={
        <Space>
          <Select
            allowClear
            placeholder="状态筛选"
            style={{ width: 140 }}
            value={statusFilter}
            onChange={(v) => {
              setStatusFilter(v);
              setPage(1);
            }}
            options={Object.entries(statusMap).map(([k, m]) => ({
              label: m.label,
              value: Number(k),
            }))}
          />
          <Input.Search
            placeholder="搜索 code / name"
            allowClear
            style={{ width: 240 }}
            onSearch={(v) => {
              setQ(v);
              setPage(1);
            }}
          />
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            新建项目
          </Button>
        </Space>
      }
    >
      <Table
        loading={loading}
        rowKey="id"
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
          { title: 'ID', dataIndex: 'id', width: 60 },
          { title: '编码', dataIndex: 'code', width: 140 },
          { title: '名称', dataIndex: 'name' },
          {
            title: '部门',
            dataIndex: 'dept_id',
            width: 140,
            render: (id: number) => deptName(id),
          },
          {
            title: '负责人',
            dataIndex: 'owner_id',
            width: 120,
            render: (id: number) => userName(id),
          },
          {
            title: '状态',
            dataIndex: 'status',
            width: 100,
            render: (s: number) => {
              const m = statusMap[s] || statusMap[0];
              return <Tag color={m.color}>{m.label}</Tag>;
            },
          },
          { title: '创建时间', dataIndex: 'created_at', width: 180 },
          {
            title: '操作',
            width: 260,
            render: (_: unknown, r: Project) => (
              <Space>
                <Button size="small" icon={<TeamOutlined />} onClick={() => openMemberDrawer(r)}>
                  成员
                </Button>
                <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(r)}>
                  编辑
                </Button>
                <Popconfirm
                  title={`确认删除 ${r.name}?`}
                  onConfirm={() => onDelete(r)}
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
        title={editing ? `编辑项目 - ${editing.name}` : '新建项目'}
        open={open}
        onCancel={() => setOpen(false)}
        onOk={onSubmit}
        confirmLoading={submitting}
        destroyOnClose
        width={620}
      >
        <Form form={form} layout="vertical" preserve={false}>
          <Form.Item
            name="code"
            label="项目编码"
            rules={[{ required: !editing, message: '请输入编码' }]}
          >
            <Input disabled={!!editing} placeholder="如 P2026001" />
          </Form.Item>
          <Form.Item name="name" label="项目名称" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea rows={3} />
          </Form.Item>
          <Form.Item name="dept_id" label="所属部门">
            <Select
              allowClear
              disabled={!!editing}
              options={depts.map((d) => ({ label: d.name, value: d.id }))}
            />
          </Form.Item>
          {editing && (
            <Form.Item name="status" label="状态">
              <Select
                options={Object.entries(statusMap).map(([k, m]) => ({
                  label: m.label,
                  value: Number(k),
                }))}
              />
            </Form.Item>
          )}
          <Form.Item name="cover_url" label="封面 URL">
            <Input placeholder="https://..." />
          </Form.Item>
        </Form>
      </Modal>

      <Drawer
        title={memberTarget ? `成员管理 - ${memberTarget.name}` : '成员管理'}
        open={memberDrawerOpen}
        onClose={() => setMemberDrawerOpen(false)}
        width={520}
        destroyOnClose
      >
        <Card size="small" title="添加成员" style={{ marginBottom: 16 }}>
          <Form form={addMemberForm} layout="inline" onFinish={onAddMember}>
            <Form.Item name="user_id" rules={[{ required: true, message: '选择用户' }]}>
              <Select
                showSearch
                placeholder="选择用户"
                style={{ width: 200 }}
                optionFilterProp="label"
                options={users.map((u) => ({
                  label: `${u.nickname || u.username} (${u.username})`,
                  value: u.id,
                }))}
              />
            </Form.Item>
            <Form.Item
              name="role_in_project"
              initialValue="editor"
              rules={[{ required: true }]}
            >
              <Select style={{ width: 130 }} options={roleInProjectOptions} />
            </Form.Item>
            <Form.Item>
              <Button
                type="primary"
                htmlType="submit"
                icon={<UserAddOutlined />}
                loading={memberSubmitting}
              >
                添加
              </Button>
            </Form.Item>
          </Form>
        </Card>
        <Typography.Title level={5} style={{ marginTop: 0 }}>
          当前成员
        </Typography.Title>
        <Table
          size="small"
          loading={memberLoading}
          rowKey="id"
          dataSource={members}
          pagination={false}
          columns={[
            {
              title: '用户',
              dataIndex: 'user_id',
              render: (id: number) => userName(id),
            },
            {
              title: '角色',
              dataIndex: 'role_in_project',
              width: 110,
              render: (s: string) => (
                <Tag color="blue">
                  {roleInProjectOptions.find((o) => o.value === s)?.label || s}
                </Tag>
              ),
            },
            {
              title: '操作',
              width: 80,
              render: (_: unknown, m: ProjectMember) => (
                <Popconfirm
                  title="确认移除该成员?"
                  onConfirm={() => onRemoveMember(m.user_id)}
                  okType="danger"
                >
                  <Button size="small" danger icon={<DeleteOutlined />} />
                </Popconfirm>
              ),
            },
          ]}
        />
      </Drawer>
    </Card>
  );
}
