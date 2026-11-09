import { useEffect, useRef, type ReactNode } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { useLocation, useNavigate } from "react-router";
import { LoginForm, type LoginNotice } from "@/features/auth/LoginForm";
import { RegisterForm } from "@/features/auth/RegisterForm";
import {
  replaceIdentity,
  type RegisteredState,
  type SessionEndedState,
} from "../session";
import { LanguageSwitch } from "../AppShell";
import { LoginBrandPanel } from "./LoginBrandPanel";

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

/**
 * Pencil `ZziSC`: brand panel left, form right, 50 / 50 from 1024 px. Below that the panel stacks
 * above the form in its short form. `/register` has no frame of its own and shares this layout.
 */
function AuthLayout({ children }: { children: ReactNode }) {
  return (
    <div className="grid min-h-screen bg-background lg:grid-cols-2">
      <LoginBrandPanel />
      <div className="relative flex flex-col items-center justify-center gap-4 px-4 pt-16 pb-10 sm:px-6 md:px-12 lg:py-12">
        <div className="absolute top-4 right-4 lg:top-6 lg:right-6">
          <LanguageSwitch />
        </div>
        {children}
      </div>
    </div>
  );
}

export function Login() {
  const { t } = useTranslation();
  const heading = useAuthHeading(t("nav.signIn"));
  const client = useQueryClient();
  const state = useLocation().state as Partial<
    RegisteredState | SessionEndedState
  > | null;
  const notice: LoginNotice | undefined =
    state?.notice === "registered"
      ? { kind: "registered", text: t("pages.register.success") }
      : state?.notice === "sessionEnded"
        ? { kind: "sessionEnded", text: t("pages.login.sessionEnded") }
        : undefined;
  return (
    <AuthLayout>
      {/* A new session means a new identity: nothing cached for the previous one may survive. The gate then
          sees `me` and redirects to the validated `next` (Frontend.md §6.4). */}
      <LoginForm
        heading={heading}
        notice={notice}
        onLoggedIn={(me) => replaceIdentity(client, me)}
      />
    </AuthLayout>
  );
}

/** No design frame: the Login layout with the register form on the right (UI-02). */
export function Register() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const heading = useAuthHeading(t("nav.register"));
  return (
    <AuthLayout>
      <div className="flex w-full max-w-md flex-col gap-6">
        <div className="flex flex-col gap-1.5">
          <h1
            ref={heading}
            tabIndex={-1}
            className="text-[28px] leading-tight font-bold tracking-tight outline-none"
          >
            {t("pages.register.title")}
          </h1>
          <p className="text-sm text-muted-foreground">
            {t("pages.register.description")}
          </p>
        </div>
        <RegisterForm
          onRegistered={() => {
            const state: RegisteredState = { notice: "registered" };
            void navigate("/login", { replace: true, state });
          }}
        />
      </div>
    </AuthLayout>
  );
}
