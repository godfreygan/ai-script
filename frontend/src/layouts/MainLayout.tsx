import { Layout, Menu, Avatar, Dropdown } from 'antd';
import {
  DashboardOutlined,
  ProjectOutlined,
  FileTextOutlined,
  ProfileOutlined,
  PictureOutlined,
  VideoCameraOutlined,
  PlayCircleOutlined,
  AuditOutlined,
  CloudUploadOutlined,
  UserOutlined,
  TeamOutlined,
  RobotOutlined,
  DollarOutlined,
  ApartmentOutlined,
  ClusterOutlined,
  LogoutOutlined,
  AppstoreOutlined,
  BgColorsOutlined,
  BarChartOutlined,
  ExperimentOutlined,
  FileSearchOutlined,
} from '@ant-design/icons';
import { Outlet, useNavigate, useLocation } from 'react-router-dom';
import { useAuthStore } from '@/stores/auth';

const { Header, Sider, Content } = Layout;

const items = [
  { key: '/', icon: <DashboardOutlined />, label: '工作台' },
  { key: '/projects', icon: <ProjectOutlined />, label: '项目管理' },
  { key: '/scripts', icon: <FileTextOutlined />, label: '剧本管理' },
  { key: '/prompts', icon: <ProfileOutlined />, label: '分集提示词' },
  { key: '/storyboards', icon: <AppstoreOutlined />, label: '分镜/风格' },
  { key: '/styles', icon: <BgColorsOutlined />, label: '风格预设' },
  { key: '/images', icon: <PictureOutlined />, label: '图片' },
  { key: '/short-videos', icon: <VideoCameraOutlined />, label: '短视频' },
  { key: '/full-videos', icon: <PlayCircleOutlined />, label: '完整视频' },
  { key: '/reviews', icon: <AuditOutlined />, label: '审核' },
  { key: '/publish', icon: <CloudUploadOutlined />, label: '发布' },
  { key: '/pipelines', icon: <ApartmentOutlined />, label: '流水线' },
  { key: '/models', icon: <RobotOutlined />, label: '模型管理' },
  { key: '/billing', icon: <DollarOutlined />, label: '计费/额度' },
  { key: '/invocations', icon: <BarChartOutlined />, label: '调用日志' },
  { key: '/users', icon: <UserOutlined />, label: '用户管理' },
  { key: '/depts', icon: <ClusterOutlined />, label: '部门管理' },
  { key: '/roles', icon: <TeamOutlined />, label: '角色权限' },
  { key: '/feature-flags', icon: <ExperimentOutlined />, label: '灰度开关' },
  { key: '/audit-logs', icon: <FileSearchOutlined />, label: '审计日志' },
];

export default function MainLayout() {
  const navigate = useNavigate();
  const location = useLocation();
  const { user, logout } = useAuthStore();

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider width={220} theme="dark">
        <div style={{ color: '#fff', padding: 16, fontSize: 18, fontWeight: 600 }}>
          AI 短剧平台
        </div>
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[location.pathname]}
          items={items}
          onClick={({ key }) => navigate(key)}
        />
      </Sider>
      <Layout>
        <Header
          style={{
            background: '#fff',
            padding: '0 24px',
            display: 'flex',
            justifyContent: 'flex-end',
            alignItems: 'center',
          }}
        >
          <Dropdown
            menu={{
              items: [
                {
                  key: 'logout',
                  icon: <LogoutOutlined />,
                  label: '退出登录',
                  onClick: () => {
                    logout();
                    navigate('/login');
                  },
                },
              ],
            }}
          >
            <span style={{ cursor: 'pointer' }}>
              <Avatar size="small" icon={<UserOutlined />} />
              <span style={{ marginLeft: 8 }}>{user?.nickname || user?.username || 'guest'}</span>
            </span>
          </Dropdown>
        </Header>
        <Content style={{ margin: 16, padding: 24, background: '#fff', minHeight: 280 }}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
}
