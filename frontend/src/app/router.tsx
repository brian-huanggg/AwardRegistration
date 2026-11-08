import { lazy, Suspense, useEffect, type ComponentType } from "react";
import { useTranslation } from "react-i18next";
import {
  Navigate,
  useLocation,
  useSearchParams,
  type RouteObject,
} from "react-router";
import { decide, safeNext } from "./access";
import { AppShell, PublicLayout } from "./layouts";
import { NotFound } from "./pages/NotFound";
import { Login, Register } from "./pages/auth";
import { IdeaCreate, IdeaDetail, IdeaEdit, IdeaList } from "./pages/ideas";
import { JoinRequests } from "./pages/JoinRequests";
import { ROUTES } from "./routes";
import { ApiError } from "@/shared/api";
import { leavingReason, useMe, type SessionEndedState } from "./session";

const AdminDeadline = lazy(() =>
  import("./pages/admin").then((m) => ({ default: m.AdminDeadline })),
);
const AdminStats = lazy(() =>
  import("./pages/admin").then((m) => ({ default: m.AdminStats })),
);

/** Page per route path; keys must match `ROUTES` (checked by a test). */
export const PAGES: Record<string, ComponentType> = {
  "/login": Login,
  "/register": Register,
  "/ideas": IdeaList,
  "/ideas/new": IdeaCreate,
  "/ideas/:id": IdeaDetail,
  "/ideas/:id/edit": IdeaEdit,
  "/join-requests": JoinRequests,
  "/admin/deadline": AdminDeadline,
  "/admin/stats": AdminStats,
};

/** Wraps a page with identity, the access decision (§6.3) and the right layout. `route` undefined = 404. */
function Gate({ route }: { route?: (typeof ROUTES)[number] }) {
  const { t } = useTranslation();
  const location = useLocation();
  const [params] = useSearchParams();
  const { data: me, isPending, isError, error } = useMe();
  useEffect(() => {
    if (!isError) return;
    if (error instanceof ApiError)
      console.error(
        "GET /auth/me failed",
        error.status,
        error.code,
        error.requestId,
      );
    else
      console.error(
        "GET /auth/me failed (network error or non-JSON response; is the backend / dev proxy up?)",
        error,
      );
  }, [isError, error]);

  if (isPending) return <p role="status">{t("common.loading")}</p>;
  if (isError) return <p role="alert">{t("errors.UNKNOWN")}</p>;

  const role = me?.role ?? null;
  const d = route
    ? decide(route.access, role, location)
    : ({ kind: "notFound" } as const);
  if (d.kind === "login") {
    const why = leavingReason();
    if (why === "signedOut") return <Navigate to="/login" replace />;
    const state: SessionEndedState | undefined =
      why === "sessionEnded" ? { notice: "sessionEnded" } : undefined;
    return (
      <Navigate
        to={`/login?next=${encodeURIComponent(d.next)}`}
        replace
        state={state}
      />
    );
  }
  if (d.kind === "home")
    return (
      <Navigate
        to={role ? safeNext(params.get("next"), role) : "/login"}
        replace
      />
    );

  const Page = d.kind === "allow" && route ? PAGES[route.path] : undefined;
  const page = Page ? (
    <Suspense fallback={<p role="status">{t("common.loading")}</p>}>
      <Page />
    </Suspense>
  ) : (
    <NotFound />
  );
  return me ? (
    <AppShell me={me}>{page}</AppShell>
  ) : (
    <PublicLayout>{page}</PublicLayout>
  );
}

function RootRedirect() {
  const { data: me, isPending } = useMe();
  if (isPending) return null;
  return <Navigate to={me ? "/ideas" : "/login"} replace />;
}

export const routes: RouteObject[] = [
  { path: "/", element: <RootRedirect /> },
  ...ROUTES.map((r) => ({ path: r.path, element: <Gate route={r} /> })),
  { path: "*", element: <Gate /> },
];
