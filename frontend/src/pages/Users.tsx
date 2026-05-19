import { useEffect, useState } from 'react';
import {
  App as AntApp,
  Button,
  Card,
  Form,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Table,
  Tag,
} from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, KeyOutlined } from '@ant-design/icons';
import { deptApi, roleApi, userApi, User, Department, Role } from '@/api/modules';

export default function UsersPage() {
  const { message } = AntApp.useApp();
  const [list, setList] = useState<User[]>([]);
  const [depts, setDepts] = useState<Department[]>([]);
  const [roles, setRoles] = useState<Role[]>([]);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [q, setQ] = useState('');

  const [open, setOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [editing, setEditing] = useState<User | null>(null);
  const [form] = Form.useForm();

  const [pwOpen, setPwOpen] = useState(false);
  const [pwSubmitting, setPwSubmitting] = useState(false);
  const [pwTarget, setPwTarget] = useState<User | null>(null);
  const [pwForm] = Form.useForm();

  const fetchList = async () => {
    setLoading(true);
    try {
      const data = await userApi.list({ page, page_size: pageSize, q });
      // 防护: data 为 undefined 时避免炸
      setList(data?.list ?? []);
      setTotal(data?.total ?? 0);
    } catch (e) {
      message.error((e as Error)?.message || '加载用户列表失败');
      setList([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchList();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, pageSize, q]);

  useEffect(() => {
    deptApi.list().then((d) => setDepts(d ?? [])).catch((e) => message.error((e as Error)?.message || '加载部门失败'));
    roleApi.list().then((r) => setRoles(r ?? [])).catch((e) => message.error((e as Error)?.message || '加载角色失败'));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const openCreate = () => {
    setEditing(null);
    form.resetFields();
    setOpen(true);
  };

  const openEdit = async (u: User) => {
    try {
      const full = await userApi.get(u.id);
      setEditing(u);
      form.setFieldsValue({
        username: full.username,
        nickname: full.nickname,
        email: full.email,
        phone: full.phone,
        dept_id: full.dept_id,
        status: full.status,
        role_ids: full.role_ids,
      });
      setOpen(true);
    } catch (e) {
      message.error((e as Error)?.message || '加载用户详情失败');
    }
  };

  const onSubmit = async () => {
    let values;
    try {
      values = await form.validateFields();
    } catch {
      return;
    }
    setSubmitting(true);
    try {
      if (editing) {
        await userApi.update(editing.id, {
          nickname: values.nickname,
          email: values.email,
          phone: values.phone,
          dept_id: values.dept_id,
          status: values.status,
          role_ids: values.role_ids,
        });
        message.success('更新成功');
      } else {
        await userApi.create(values);
        message.success('创建成功');
      }
      setOpen(false);
      fetchList();
    } catch (e) {
      message.error((e as Error)?.message || '保存失败');
    } finally {
      setSubmitting(false);
    }
  };

  const onDelete = async (id: number) => {
    try {
      await userApi.delete(id);
      message.success('已删除');
      fetchList();
    } catch (e) {
      message.error((e as Error)?.message || '删除失败');
    }
  };

  const onResetPw = (u: User) => {
    setPwTarget(u);
    pwForm.resetFields();
    setPwOpen(true);
  };

  const submitResetPw = async () => {
    let v;
    try {
      v = await pwForm.validateFields();
    } catch {
      return;
    }
    if (!pwTarget) return;
    setPwSubmitting(true);
    try {
      await userApi.resetPassword(pwTarget.id, v.new_password);
      message.success('密码已重置');
      setPwOpen(false);
    } catch (e) {
      message.error((e as Error)?.message || '重置失败');
    } finally {
      setPwSubmitting(false);
    }
  };

  const deptName = (id: number) => depts.find((d) => d.id === id)?.name || '-';

  return (
    <Card
      title="用户管理"
      extra={
        <Space>
          <Input.Search
            placeholder="搜索用户名/邮箱"
            allowClear
            style={{ width: 240 }}
            onSearch={(v) => {
              setQ(v);
              setPage(1);
            }}
          />
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            新建用户
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
          onChange: (p, ps) => {
            setPage(p);
            setPageSize(ps);
          },
        }}
        columns={[
          { title: 'ID', dataIndex: 'id', width: 60 },
          { title: '用户名', dataIndex: 'username', width: 140 },
          { title: '昵称', dataIndex: 'nickname' },
          { title: '邮箱', dataIndex: 'email' },
          { title: '部门', dataIndex: 'dept_id', render: (id: number) => deptName(id) },
          {
            title: '状态',
            dataIndex: 'status',
            width: 80,
            render: (s: number) =>
              s === 1 ? <Tag color="success">启用</Tag> : <Tag color="default">禁用</Tag>,
          },
          { title: '最后登录', dataIndex: 'last_login_at', width: 180 },
          {
            title: '操作',
            width: 220,
            render: (_: unknown, u: User) => (
              <Space>
                <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(u)}>
                  编辑
                </Button>
                <Button size="small" icon={<KeyOutlined />} onClick={() => onResetPw(u)}>
                  重置密码
                </Button>
                <Popconfirm
                  title={`确认删除 ${u.username}?`}
                  onConfirm={() => onDelete(u.id)}
                  okType="danger"
                >
                  <Button size="small" danger icon={<DeleteOutlined />} disabled={u.id === 1}>
                    删除
                  </Button>
                </Popconfirm>
              </Space>
            ),
          },
        ]}
      />

      <Modal
        title={editing ? '编辑用户' : '新建用户'}
        open={open}
        onCancel={() => setOpen(false)}
        onOk={onSubmit}
        confirmLoading={submitting}
        destroyOnClose
        width={600}
      >
        <Form form={form} layout="vertical" preserve={false}>
          <Form.Item
            name="username"
            label="用户名"
            rules={[{ required: !editing, message: '请输入用户名' }]}
          >
            <Input disabled={!!editing} />
          </Form.Item>
          {!editing && (
            <Form.Item
              name="password"
              label="密码"
              rules={[{ required: true, min: 6, message: '至少 6 位' }]}
            >
              <Input.Password autoComplete="new-password" />
            </Form.Item>
          )}
          <Form.Item name="nickname" label="昵称">
            <Input />
          </Form.Item>
          <Form.Item name="email" label="邮箱" rules={[{ type: 'email' }]}>
            <Input />
          </Form.Item>
          <Form.Item name="phone" label="手机">
            <Input />
          </Form.Item>
          <Form.Item name="dept_id" label="部门">
            <Select
              allowClear
              options={depts.map((d) => ({ label: d.name, value: d.id }))}
            />
          </Form.Item>
          <Form.Item name="role_ids" label="角色">
            <Select
              mode="multiple"
              options={roles.map((r) => ({ label: r.name, value: r.id }))}
            />
          </Form.Item>
          <Form.Item name="status" label="状态" initialValue={1}>
            <Select
              options={[
                { label: '启用', value: 1 },
                { label: '禁用', value: 2 },
              ]}
            />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={`重置密码 - ${pwTarget?.username}`}
        open={pwOpen}
        onCancel={() => setPwOpen(false)}
        onOk={submitResetPw}
        confirmLoading={pwSubmitting}
        destroyOnClose
      >
        <Form form={pwForm} layout="vertical" preserve={false}>
          <Form.Item
            name="new_password"
            label="新密码"
            rules={[{ required: true, min: 6, message: '至少 6 位' }]}
          >
            <Input.Password />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
}
