import { useEffect, useState } from 'react';
import { Card, Col, Row, Statistic, Table, Tag, Space, Spin } from 'antd';
import {
  ProjectOutlined,
  FileTextOutlined,
  PictureOutlined,
  VideoCameraOutlined,
  DollarCircleOutlined,
  ApiOutlined,
} from '@ant-design/icons';
import dayjs from 'dayjs';
import {
  InvocationLog,
  InvocationStats,
  invocationApi,
  projectApi,
} from '@/api/modules';

const statusColor: Record<string, string> = {
  succeeded: 'success',
  failed: 'error',
  running: 'processing',
  pending: 'default',
};

export default function DashboardPage() {
  const [loading, setLoading] = useState(false);
  const [projectCount, setProjectCount] = useState(0);
  const [todayStats, setTodayStats] = useState<InvocationStats | null>(null);
  const [scriptCalls, setScriptCalls] = useState(0);
  const [imageCalls, setImageCalls] = useState(0);
  const [videoCalls, setVideoCalls] = useState(0);
  const [recent, setRecent] = useState<InvocationLog[]>([]);

  useEffect(() => {
    const load = async () => {
      setLoading(true);
      const from = dayjs().startOf('day').toISOString();
      const to = dayjs().endOf('day').toISOString();
      try {
        const [proj, total, script, image, video, list] = await Promise.all([
          projectApi.list({ page: 1, page_size: 1 }).catch(() => ({ total: 0, list: [] })),
          invocationApi.stats({ from, to }).catch(() => null),
          invocationApi.stats({ from, to, biz_type: 'script_split' }).catch(() => null),
          invocationApi.stats({ from, to, biz_type: 'image_gen' }).catch(() => null),
          invocationApi.stats({ from, to, biz_type: 'video_gen' }).catch(() => null),
          invocationApi
            .list({ page: 1, page_size: 8 })
            .catch(() => ({ total: 0, list: [] as InvocationLog[] })),
        ]);
        setProjectCount(proj.total || 0);
        setTodayStats(total);
        setScriptCalls(script?.calls ?? 0);
        setImageCalls(image?.calls ?? 0);
        setVideoCalls(video?.calls ?? 0);
        setRecent(list.list || []);
      } finally {
        setLoading(false);
      }
    };
    load();
  }, []);

  return (
    <Spin spinning={loading}>
      <div>
        <h2 style={{ marginBottom: 16 }}>工作台</h2>
        <Row gutter={16}>
          <Col span={6}>
            <Card>
              <Statistic title="在产项目" value={projectCount} prefix={<ProjectOutlined />} />
            </Card>
          </Col>
          <Col span={6}>
            <Card>
              <Statistic
                title="今日剧本调用"
                value={scriptCalls}
                prefix={<FileTextOutlined />}
              />
            </Card>
          </Col>
          <Col span={6}>
            <Card>
              <Statistic
                title="今日图片调用"
                value={imageCalls}
                prefix={<PictureOutlined />}
              />
            </Card>
          </Col>
          <Col span={6}>
            <Card>
              <Statistic
                title="今日视频调用"
                value={videoCalls}
                prefix={<VideoCameraOutlined />}
              />
            </Card>
          </Col>
        </Row>

        <Row gutter={16} style={{ marginTop: 16 }}>
          <Col span={8}>
            <Card>
              <Statistic
                title="今日总调用"
                value={todayStats?.calls ?? 0}
                prefix={<ApiOutlined />}
              />
            </Card>
          </Col>
          <Col span={8}>
            <Card>
              <Statistic
                title="今日累计成本"
                value={todayStats?.cost ?? 0}
                precision={4}
                prefix={<DollarCircleOutlined />}
                suffix="元"
              />
            </Card>
          </Col>
          <Col span={8}>
            <Card>
              <Statistic
                title="今日 Tokens (in/out)"
                value={`${todayStats?.input_tokens ?? 0} / ${todayStats?.output_tokens ?? 0}`}
              />
            </Card>
          </Col>
        </Row>

        <Card style={{ marginTop: 16 }} title="近期模型调用">
          <Table
            size="small"
            rowKey="id"
            pagination={false}
            dataSource={recent}
            locale={{ emptyText: '暂无数据' }}
            columns={[
              { title: 'ID', dataIndex: 'id', width: 80 },
              {
                title: '业务',
                dataIndex: 'biz_type',
                width: 140,
                render: (v: string) => <Tag color="blue">{v || '-'}</Tag>,
              },
              { title: '模型', dataIndex: 'model_id', width: 100, render: (v: number) => `#${v}` },
              { title: '耗时(ms)', dataIndex: 'duration_ms', width: 100 },
              {
                title: '成本',
                dataIndex: 'cost',
                width: 100,
                render: (c: number) => (c ? `¥${c.toFixed(4)}` : '-'),
              },
              {
                title: '状态',
                dataIndex: 'status',
                width: 100,
                render: (s: string) => (
                  <Tag color={statusColor[s] || 'default'}>{s || '-'}</Tag>
                ),
              },
              {
                title: '开始时间',
                dataIndex: 'started_at',
                render: (v: string) => (v ? dayjs(v).format('YYYY-MM-DD HH:mm:ss') : '-'),
              },
            ]}
          />
          <Space style={{ marginTop: 8, color: '#999', fontSize: 12 }}>
            数据来源 invocationApi.stats / list,按今日 0 点为起点聚合
          </Space>
        </Card>
      </div>
    </Spin>
  );
}
