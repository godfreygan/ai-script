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
import FeatureFlagsPage from './pages/FeatureFlags';
import AuditLogsPage from './pages/AuditLogs';
import { useAuthStore } from './stores/auth';

function PrivateRoute({ children }: { children: React.ReactNode }) {
  const token = useAuthStore((s) => s.accessToken);
  return token ? <>{children}</> : <Navigate to="/login" replace />;
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route
        path="/"
        element={
          <PrivateRoute>
            <MainLayout />
          </PrivateRoute>
        }
      >
        <Route index element={<DashboardPage />} />
        <Route path="projects" element={<ProjectsPage />} />
        <Route path="scripts" element={<ScriptsPage />} />
        <Route path="prompts" element={<PromptsPage />} />
        <Route path="storyboards" element={<StoryboardsPage />} />
        <Route path="styles" element={<StylesPage />} />
        <Route path="images" element={<ImagesPage />} />
        <Route path="short-videos" element={<ShortVideosPage />} />
        <Route path="full-videos" element={<FullVideosPage />} />
        <Route path="reviews" element={<ReviewsPage />} />
        <Route path="publish" element={<PublishPage />} />
        <Route path="users" element={<UsersPage />} />
        <Route path="depts" element={<DeptsPage />} />
        <Route path="roles" element={<RolesPage />} />
        <Route path="models" element={<ModelsPage />} />
        <Route path="billing" element={<BillingPage />} />
        <Route path="pipelines" element={<PipelinesPage />} />
        <Route path="invocations" element={<InvocationsPage />} />
        <Route path="feature-flags" element={<FeatureFlagsPage />} />
        <Route path="audit-logs" element={<AuditLogsPage />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
