import { useTranslation } from 'react-i18next';
import { Navigate, useLocation, useSearchParams, type RouteObject } from 'react-router';
import { decide, safeNext } from './access';
import { AppShell, PageHeading, PublicLayout } from './layouts';
import { NotFound } from './pages/NotFound';
import { ROUTES, type RouteDef } from './routes';
import { useMe } from './session';

function Placeholder({ route }: { route: RouteDef }) {
  const { t } = useTranslation();
  return (
    <>
      <PageHeading title={t(`nav.${route.titleKey}`)} />
      <p>{t('pages.placeholder')}</p>
    </>
  );
}

/** Wraps a page with identity, the access decision (§6.3) and the right layout. `route` undefined = 404. */
function Gate({ route }: { route?: RouteDef }) {
  const { t } = useTranslation();
  const location = useLocation();
  const [params] = useSearchParams();
  const { data: me, isPending, isError } = useMe();

  if (isPending) return <p role="status">{t('common.loading')}</p>;
  if (isError) return <p role="alert">{t('errors.UNKNOWN')}</p>;

  const role = me?.role ?? null;
  const d = route ? decide(route.access, role, location) : ({ kind: 'notFound' } as const);
  if (d.kind === 'login') return <Navigate to={`/login?next=${encodeURIComponent(d.next)}`} replace />;
  if (d.kind === 'home') return <Navigate to={role ? safeNext(params.get('next'), role) : '/login'} replace />;

  const page = d.kind === 'allow' && route ? <Placeholder route={route} /> : <NotFound />;
  return me ? <AppShell me={me}>{page}</AppShell> : <PublicLayout>{page}</PublicLayout>;
}

function RootRedirect() {
  const { data: me, isPending } = useMe();
  if (isPending) return null;
  return <Navigate to={me ? '/ideas' : '/login'} replace />;
}

export const routes: RouteObject[] = [
  { path: '/', element: <RootRedirect /> },
  ...ROUTES.map((r) => ({ path: r.path, element: <Gate route={r} /> })),
  { path: '*', element: <Gate /> },
];
