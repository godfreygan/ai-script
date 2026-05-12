import { useEffect, useMemo, useState } from 'react';
import {
  App as AntApp,
  Button,
  Card,
  Empty,
  Form,
  Image,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Table,
  Tag,
  Tooltip,
  Upload,
} from 'antd';
import {
  BgColorsOutlined,
  DeleteOutlined,
  EditOutlined,
  PlusOutlined,
  UploadOutlined,
} from '@ant-design/icons';
import type { UploadFile } from 'antd/es/upload/interface';
import { Project, Style, projectApi, styleApi, uploadApi } from '@/api/modules';

function parseRefImages(v: Style['reference_images']): string[] {
  if (!v) return [];
  if (Array.isArray(v)) return v;
  try {
    const arr = JSON.parse(v as string);
    return Array.isArray(arr) ? arr : [];
  } catch {
    return [];
  }
}

export default function StylesPage() {
  const { message } = AntApp.useApp();

  const [projects, setProjects] = useState<Project[]>([]);
  const [list, setList] = useState<Style[]>([]);
  const [loading, setLoading] = useState(false);
  const [projectId, setProjectId] = useState<number | undefined>();

  const [editOpen, setEditOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<Style | null>(null);
  const [editForm] = Form.useForm();
  const [refFiles, setRefFiles] = useState<UploadFile[]>([]);
  const [refUrls, setRefUrls] = useState<string[]>([]);

  const isEdit = useMemo(() => editTarget !== null, [editTarget]);

  const fetchProjects = async () => {
    try {
      const p = await projectApi.list({ page_size: 200 });
      setProjects(p.list);
    } catch {
      /* ignore */
    }
  };

  const fetchList = async () => {
    setLoading(true);
    try {
      const data = await styleApi.list(projectId);
      setList(data);
    } catch {
      setList([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchProjects();
  }, []);

  useEffect(() => {
    fetchList();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [projectId]);

  const openCreate = () => {
    setEditTarget(null);
    editForm.resetFields();
    editForm.setFieldsValue({ project_id: projectId });
    setRefUrls([]);
    setRefFiles([]);
    setEditOpen(true);
  };

  const openEdit = (s: Style) => {
    setEditTarget(s);
    const urls = parseRefImages(s.reference_images);
    editForm.setFieldsValue({
      project_id: s.project_id,
      name: s.name,
      art_style: s.art_style,
      color_tone: s.color_tone,
      lighting: s.lighting,
      lora_id: s.lora_id,
      description: s.description,
    });
    setRefUrls(urls);
    setRefFiles(
      urls.map((u, i) => ({
        uid: `-${i}`,
        name: u.split('/').pop() || `ref-${i}`,
        url: u,
        status: 'done',
      })),
    );
    setEditOpen(true);
  };

  const handleUpload = async (file: File): Promise<string | null> => {
    try {
      const r = await uploadApi.upload('styles', file);
      return r.url;
    } catch (e) {
      const errMsg = e instanceof Error ? e.message : '上传失败';
      message.error(errMsg);
      return null;
    }
  };

  const onSave = async () => {
    const v = await editForm.validateFields();
    const payload = {
      ...v,
      reference_images: refUrls,
    };
    try {
      if (isEdit && editTarget) {
        await styleApi.update(editTarget.id, payload);
        message.success('已保存');
      } else {
        await styleApi.create(payload);
        message.success('已创建');
      }
      setEditOpen(false);
      fetchList();
    } catch {
      /* ignore */
    }
  };

  const onDelete = async (id: number) => {
    try {
      await styleApi.delete(id);
      message.success('已删除');
      fetchList();
    } catch {
      /* ignore */
    }
  };

  return (
    <Card
      title={
        <Space>
          <BgColorsOutlined />
          风格预设管理
        </Space>
      }
      extra={
        <Space>
          <Select
            allowClear
            placeholder="按项目筛选"
            style={{ width: 200 }}
            value={projectId}
            options={projects.map((p) => ({ label: p.name, value: p.id }))}
            onChange={(v) => setProjectId(v)}
          />
          <Tooltip title="创建新的风格预设(可绑定到项目或全局)">
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
              新建风格
            </Button>
          </Tooltip>
        </Space>
      }
    >
      <Table
        rowKey="id"
        size="small"
        loading={loading}
        dataSource={list}
        pagination={false}
        locale={{
          emptyText: (
            <Empty description="还没有风格预设">
              <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
                新建风格
              </Button>
            </Empty>
          ),
        }}
        columns={[
          { title: 'ID', dataIndex: 'id', width: 80 },
          { title: '名称', dataIndex: 'name' },
          {
            title: '项目',
            dataIndex: 'project_id',
            width: 160,
            render: (v: number) => (v ? projects.find((p) => p.id === v)?.name || `#${v}` : '-'),
          },
          { title: '画风', dataIndex: 'art_style', width: 140 },
          { title: '色调', dataIndex: 'color_tone', width: 120 },
          { title: '灯光', dataIndex: 'lighting', width: 120 },
          {
            title: '参考图',
            dataIndex: 'reference_images',
            width: 200,
            render: (v: Style['reference_images']) => {
              const urls = parseRefImages(v);
              return urls.length ? (
                <Image.PreviewGroup>
                  <Space size={4}>
                    {urls.slice(0, 3).map((u) => (
                      <Image
                        key={u}
                        src={u}
                        width={36}
                        height={36}
                        style={{ objectFit: 'cover', borderRadius: 4 }}
                      />
                    ))}
                    {urls.length > 3 && <Tag>+{urls.length - 3}</Tag>}
                  </Space>
                </Image.PreviewGroup>
              ) : (
                '-'
              );
            },
          },
          { title: 'LoRA', dataIndex: 'lora_id', width: 120 },
          {
            title: '状态',
            dataIndex: 'status',
            width: 80,
            render: (v: number) => (
              <Tag color={v === 1 ? 'success' : 'default'}>{v === 1 ? '启用' : '禁用'}</Tag>
            ),
          },
          {
            title: '操作',
            key: 'op',
            width: 160,
            render: (_: unknown, r: Style) => (
              <Space size={4}>
                <Tooltip title="编辑该风格的画风/色调/灯光/参考图">
                  <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(r)}>
                    编辑
                  </Button>
                </Tooltip>
                <Popconfirm title="确认删除该风格?" onConfirm={() => onDelete(r.id)}>
                  <Tooltip title="删除该风格(不可恢复)">
                    <Button size="small" danger icon={<DeleteOutlined />}>
                      删除
                    </Button>
                  </Tooltip>
                </Popconfirm>
              </Space>
            ),
          },
        ]}
      />

      <Modal
        open={editOpen}
        title={isEdit ? `编辑风格 - ${editTarget?.name ?? ''}` : '新建风格'}
        onCancel={() => setEditOpen(false)}
        onOk={onSave}
        width={680}
        destroyOnClose
      >
        <Form form={editForm} layout="vertical">
          <Form.Item name="project_id" label="所属项目">
            <Select
              allowClear
              showSearch
              optionFilterProp="label"
              options={projects.map((p) => ({ label: p.name, value: p.id }))}
              placeholder="留空表示全局风格"
              disabled={isEdit}
            />
          </Form.Item>
          <Form.Item name="name" label="名称" rules={[{ required: true, max: 64 }]}>
            <Input placeholder="如:都市霓虹、复古港片..." />
          </Form.Item>
          <Space wrap style={{ width: '100%' }}>
            <Form.Item name="art_style" label="画风" style={{ width: 200 }}>
              <Input placeholder="如:写实/动漫/水彩" />
            </Form.Item>
            <Form.Item name="color_tone" label="色调" style={{ width: 200 }}>
              <Input placeholder="如:冷色/暖色/高饱和" />
            </Form.Item>
            <Form.Item name="lighting" label="灯光" style={{ width: 200 }}>
              <Input placeholder="如:逆光/侧光/柔光" />
            </Form.Item>
          </Space>
          <Form.Item name="lora_id" label="LoRA / 模型增强 ID">
            <Input placeholder="可选,关联自定义 LoRA" />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea rows={2} placeholder="额外风格说明,会拼接到 image_prompt" />
          </Form.Item>
          <Form.Item label="参考图">
            <Upload
              listType="picture-card"
              fileList={refFiles}
              beforeUpload={async (file) => {
                const url = await handleUpload(file as File);
                if (url) {
                  setRefUrls((prev) => [...prev, url]);
                  setRefFiles((prev) => [
                    ...prev,
                    {
                      uid: `${Date.now()}`,
                      name: file.name,
                      url,
                      status: 'done',
                    },
                  ]);
                }
                return false;
              }}
              onRemove={(file) => {
                const i = refFiles.findIndex((f) => f.uid === file.uid);
                if (i >= 0) {
                  setRefUrls((prev) => prev.filter((_, idx) => idx !== i));
                  setRefFiles((prev) => prev.filter((_, idx) => idx !== i));
                }
                return true;
              }}
              accept="image/*"
            >
              {refFiles.length < 6 && (
                <div>
                  <UploadOutlined />
                  <div style={{ marginTop: 4 }}>上传</div>
                </div>
              )}
            </Upload>
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
}
