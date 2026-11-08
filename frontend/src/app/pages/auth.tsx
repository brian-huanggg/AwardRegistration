import { useEffect, useRef } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Leaf } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useLocation, useNavigate } from "react-router";
import { LoginForm } from "@/features/auth/LoginForm";
import { RegisterForm } from "@/features/auth/RegisterForm";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/shared/ui/card";
import {
  replaceIdentity,
  type RegisteredState,
  type SessionEndedState,
} from "../session";

/** One h1 per page; focus moves to it after a route change and the title updates (Frontend.md §8). */
function useAuthHeading(title: string) {
  const { t } = useTranslation();
  const ref = useRef<HTMLHeadingElement>(null);
  useEffect(() => {
    document.title = `${title} · ${t("app.name")}`;
    ref.current?.focus();
  }, [title, t]);
  return ref;
}

/** shadcn `login-02`: two columns, form on the left, brand panel on the right (hidden below lg). */
export function Login() {
  const { t } = useTranslation();
  const heading = useAuthHeading(t("nav.signIn"));
  const client = useQueryClient();
  const state = useLocation().state as Partial<
    RegisteredState | SessionEndedState
  > | null;
  const notice =
    state?.notice === "registered"
      ? t("pages.register.success")
      : state?.notice === "sessionEnded"
        ? t("pages.login.sessionEnded")
        : undefined;
  return (
    <div className="grid overflow-hidden rounded-xl border lg:grid-cols-2">
      <div className="flex items-center justify-center p-6 md:p-10">
        {/* A new session means a new identity: nothing cached for the previous one may survive. The gate then
            sees `me` and redirects to the validated `next` (Frontend.md §6.4). */}
        <LoginForm
          heading={heading}
          notice={notice}
          onLoggedIn={(me) => replaceIdentity(client, me)}
        />
      </div>
      <div
        aria-hidden="true"
        className="hidden flex-col items-center justify-center gap-3 bg-muted p-10 text-center lg:flex"
      >
        <div className="flex size-12 items-center justify-center rounded-md bg-primary text-primary-foreground">
          <Leaf className="size-6" />
        </div>
        <p className="text-xl font-semibold">{t("pages.login.panelTitle")}</p>
        <p className="max-w-xs text-sm text-balance text-muted-foreground">
          {t("pages.login.panelBody")}
        </p>
      </div>
    </div>
  );
}

/** shadcn `signup-01`: a single centred card. */
export function Register() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const heading = useAuthHeading(t("nav.register"));
  return (
    <div className="mx-auto w-full max-w-sm">
      <Card>
        <CardHeader>
          <CardTitle>
            <h1 ref={heading} tabIndex={-1} className="outline-none">
              {t("pages.register.title")}
            </h1>
          </CardTitle>
          <CardDescription>{t("pages.register.description")}</CardDescription>
        </CardHeader>
        <CardContent>
          <RegisterForm
            onRegistered={() => {
              const state: RegisteredState = { notice: "registered" };
              void navigate("/login", { replace: true, state });
            }}
          />
        </CardContent>
      </Card>
    </div>
  );
}
