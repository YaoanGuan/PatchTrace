import { lazy, Suspense, type ReactNode } from 'react';
import { createBrowserRouter, Navigate } from 'react-router-dom';
import { AppShell } from '../components/layout/AppShell';

const IdentityPage = lazy(
  () => import('../features/identity/pages/IdentityPage'),
);
const MarketplacePage = lazy(
  () => import('../features/marketplace/pages/MarketplacePage'),
);
const TaskDetailPage = lazy(
  () => import('../features/marketplace/pages/TaskDetailPage'),
);
const AuditPage = lazy(() => import('../features/audit/pages/AuditPage'));
const AuditDetailPage = lazy(
  () => import('../features/audit/pages/AuditDetailPage'),
);
const OnboardingPage = lazy(
  () => import('../features/onboarding/pages/OnboardingPage'),
);
const NotFoundPage = lazy(() => import('./pages/NotFoundPage'));

const withSuspense = (node: ReactNode) => (
  <Suspense
    fallback={
      <div className="flex h-48 items-center justify-center text-sm text-slate-400">
        页面加载中…
      </div>
    }
  >
    {node}
  </Suspense>
);

export const router = createBrowserRouter([
  {
    path: '/',
    element: <AppShell />,
    children: [
      { index: true, element: <Navigate to="/marketplace" replace /> },
      { path: 'identity', element: withSuspense(<IdentityPage />) },
      { path: 'marketplace', element: withSuspense(<MarketplacePage />) },
      {
        path: 'marketplace/tasks/:taskId',
        element: withSuspense(<TaskDetailPage />),
      },
      { path: 'audit', element: withSuspense(<AuditPage />) },
      { path: 'audit/:taskId', element: withSuspense(<AuditDetailPage />) },
      { path: 'onboarding', element: withSuspense(<OnboardingPage />) },
      { path: '*', element: withSuspense(<NotFoundPage />) },
    ],
  },
]);
