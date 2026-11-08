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

/** Brand mark shared by both hero layouts and the form header. */
function LogoMark({ className = "size-16" }: { className?: string }) {
  return (
    <div
      aria-hidden="true"
      className={`flex items-center justify-center rounded-2xl bg-primary text-primary-foreground ${className}`}
    >
      <Leaf className="size-1/2" />
    </div>
  );
}

function HeroCopy({ title, body }: { title: string; body: string }) {
  return (
    <div
      aria-hidden="true"
      className="flex flex-col items-center gap-4 text-center"
    >
      <LogoMark />
      <p className="max-w-sm text-3xl font-semibold text-balance whitespace-pre-line text-primary">
        {title}
      </p>
      <p className="max-w-xs text-sm text-balance text-foreground/80">{body}</p>
    </div>
  );
}

/** Design `Login - Employee`: 500px photo panel on the left (lg and up), form centred on the right. */
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
    <div className="grid min-h-screen bg-card lg:grid-cols-[500px_1fr]">
      <div
        className="relative hidden items-center justify-center bg-cover bg-center p-10 lg:flex"
        style={{ backgroundImage: "url(/auth-hero-login.jpg)" }}
      >
        <HeroCopy
          title={t("pages.login.panelTitle")}
          body={t("pages.login.panelBody")}
        />
      </div>
      <div className="flex items-center justify-center p-6 md:p-12">
        {/* A new session means a new identity: nothing cached for the previous one may survive. The gate then
            sees `me` and redirects to the validated `next` (Frontend.md §6.4). */}
        <LoginForm
          heading={heading}
          notice={notice}
          onLoggedIn={(me) => replaceIdentity(client, me)}
        />
      </div>
    </div>
  );
}

/** Design `Sign Up - Employee`: a full-bleed photo with the form card centred on it. */
export function Register() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const heading = useAuthHeading(t("nav.register"));
  return (
    <div
      className="flex min-h-screen items-center justify-center bg-cover bg-center px-4 py-10"
      style={{ backgroundImage: "url(/auth-hero-register.jpg)" }}
    >
      <Card className="w-full max-w-md shadow-lg">
        <CardHeader className="items-center text-center">
          <LogoMark className="size-14" />
          <CardTitle className="pt-2 text-2xl">
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
