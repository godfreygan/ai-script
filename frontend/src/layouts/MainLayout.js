import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { Layout, Menu, Avatar, Dropdown } from 'antd';
import { DashboardOutlined, ProjectOutlined, FileTextOutlined, ProfileOutlined, PictureOutlined, VideoCameraOutlined, PlayCircleOutlined, AuditOutlined, CloudUploadOutlined, UserOutlined, TeamOutlined, RobotOutlined, DollarOutlined, ApartmentOutlined, ClusterOutlined, LogoutOutlined, AppstoreOutlined, BgColorsOutlined, BarChartOutlined, } from '@ant-design/icons';
import { Outlet, useNavigate, useLocation } from 'react-router-dom';
import { useAuthStore } from '@/stores/auth';
const { Header, Sider, Content } = Layout;
const items = [
    { key: '/', icon: _jsx(DashboardOutlined, {}), label: '工作台' },
    { key: '/projects', icon: _jsx(ProjectOutlined, {}), label: '项目管理' },
    { key: '/scripts', icon: _jsx(FileTextOutlined, {}), label: '剧本管理' },
    { key: '/prompts', icon: _jsx(ProfileOutlined, {}), label: '分集提示词' },
    { key: '/storyboards', icon: _jsx(AppstoreOutlined, {}), label: '分镜/风格' },
    { key: '/styles', icon: _jsx(BgColorsOutlined, {}), label: '风格预设' },
    { key: '/images', icon: _jsx(PictureOutlined, {}), label: '图片' },
    { key: '/short-videos', icon: _jsx(VideoCameraOutlined, {}), label: '短视频' },
    { key: '/full-videos', icon: _jsx(PlayCircleOutlined, {}), label: '完整视频' },
    { key: '/reviews', icon: _jsx(AuditOutlined, {}), label: '审核' },
    { key: '/publish', icon: _jsx(CloudUploadOutlined, {}), label: '发布' },
    { key: '/pipelines', icon: _jsx(ApartmentOutlined, {}), label: '流水线' },
    { key: '/models', icon: _jsx(RobotOutlined, {}), label: '模型管理' },
    { key: '/billing', icon: _jsx(DollarOutlined, {}), label: '计费/额度' },
    { key: '/invocations', icon: _jsx(BarChartOutlined, {}), label: '调用日志' },
    { key: '/users', icon: _jsx(UserOutlined, {}), label: '用户管理' },
    { key: '/depts', icon: _jsx(ClusterOutlined, {}), label: '部门管理' },
    { key: '/roles', icon: _jsx(TeamOutlined, {}), label: '角色权限' },
];
export default function MainLayout() {
    const navigate = useNavigate();
    const location = useLocation();
    const { user, logout } = useAuthStore();
    return (_jsxs(Layout, { style: { minHeight: '100vh' }, children: [_jsxs(Sider, { width: 220, theme: "dark", children: [_jsx("div", { style: { color: '#fff', padding: 16, fontSize: 18, fontWeight: 600 }, children: "AI \u77ED\u5267\u5E73\u53F0" }), _jsx(Menu, { theme: "dark", mode: "inline", selectedKeys: [location.pathname], items: items, onClick: ({ key }) => navigate(key) })] }), _jsxs(Layout, { children: [_jsx(Header, { style: {
                            background: '#fff',
                            padding: '0 24px',
                            display: 'flex',
                            justifyContent: 'flex-end',
                            alignItems: 'center',
                        }, children: _jsx(Dropdown, { menu: {
                                items: [
                                    {
                                        key: 'logout',
                                        icon: _jsx(LogoutOutlined, {}),
                                        label: '退出登录',
                                        onClick: () => {
                                            logout();
                                            navigate('/login');
                                        },
                                    },
                                ],
                            }, children: _jsxs("span", { style: { cursor: 'pointer' }, children: [_jsx(Avatar, { size: "small", icon: _jsx(UserOutlined, {}) }), _jsx("span", { style: { marginLeft: 8 }, children: user?.nickname || user?.username || 'guest' })] }) }) }), _jsx(Content, { style: { margin: 16, padding: 24, background: '#fff', minHeight: 280 }, children: _jsx(Outlet, {}) })] })] }));
}
