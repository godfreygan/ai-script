import { Fragment as _Fragment, jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { Routes, Route, Navigate } from 'react-router-dom';
import MainLayout from './layouts/MainLayout';
import LoginPage from './pages/Login';
import DashboardPage from './pages/Dashboard';
import ProjectsPage from './pages/Projects';
import ScriptsPage from './pages/Scripts';
import PromptsPage from './pages/Prompts';
import StoryboardsPage from './pages/Storyboards';
import ImagesPage from './pages/Images';
import ShortVideosPage from './pages/ShortVideos';
import FullVideosPage from './pages/FullVideos';
import ReviewsPage from './pages/Reviews';
import PublishPage from './pages/Publish';
import UsersPage from './pages/Users';
import DeptsPage from './pages/Depts';
import RolesPage from './pages/Roles';
import ModelsPage from './pages/Models';
import BillingPage from './pages/Billing';
import PipelinesPage from './pages/Pipelines';
import StylesPage from './pages/Styles';
import InvocationsPage from './pages/Invocations';
import { useAuthStore } from './stores/auth';
function PrivateRoute({ children }) {
    const token = useAuthStore((s) => s.accessToken);
    return token ? _jsx(_Fragment, { children: children }) : _jsx(Navigate, { to: "/login", replace: true });
}
export default function App() {
    return (_jsxs(Routes, { children: [_jsx(Route, { path: "/login", element: _jsx(LoginPage, {}) }), _jsxs(Route, { path: "/", element: _jsx(PrivateRoute, { children: _jsx(MainLayout, {}) }), children: [_jsx(Route, { index: true, element: _jsx(DashboardPage, {}) }), _jsx(Route, { path: "projects", element: _jsx(ProjectsPage, {}) }), _jsx(Route, { path: "scripts", element: _jsx(ScriptsPage, {}) }), _jsx(Route, { path: "prompts", element: _jsx(PromptsPage, {}) }), _jsx(Route, { path: "storyboards", element: _jsx(StoryboardsPage, {}) }), _jsx(Route, { path: "styles", element: _jsx(StylesPage, {}) }), _jsx(Route, { path: "images", element: _jsx(ImagesPage, {}) }), _jsx(Route, { path: "short-videos", element: _jsx(ShortVideosPage, {}) }), _jsx(Route, { path: "full-videos", element: _jsx(FullVideosPage, {}) }), _jsx(Route, { path: "reviews", element: _jsx(ReviewsPage, {}) }), _jsx(Route, { path: "publish", element: _jsx(PublishPage, {}) }), _jsx(Route, { path: "users", element: _jsx(UsersPage, {}) }), _jsx(Route, { path: "depts", element: _jsx(DeptsPage, {}) }), _jsx(Route, { path: "roles", element: _jsx(RolesPage, {}) }), _jsx(Route, { path: "models", element: _jsx(ModelsPage, {}) }), _jsx(Route, { path: "billing", element: _jsx(BillingPage, {}) }), _jsx(Route, { path: "pipelines", element: _jsx(PipelinesPage, {}) }), _jsx(Route, { path: "invocations", element: _jsx(InvocationsPage, {}) })] }), _jsx(Route, { path: "*", element: _jsx(Navigate, { to: "/", replace: true }) })] }));
}
