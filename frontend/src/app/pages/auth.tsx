import { useEffect, useRef, type ReactNode } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { useLocation, useNavigate } from "react-router";
import { LoginForm, type LoginNotice } from "@/features/auth/LoginForm";
import { RegisterForm } from "@/features/auth/RegisterForm";
import { cn } from "@/shared/ui/utils";
import {
  replaceIdentity,
  type RegisteredState,
  type SessionEndedState,
} from "../session";
import { LanguageSwitch } from "../AppShell";
import {
  BrandEyebrow,
  headlineClass,
  LoginBrandPanel,
} from "./LoginBrandPanel";

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
 * Pencil `SxCah` (Eco — Login): brand column left, illustration right from 1024 px; below that a
 * single column. `/register` has no frame of its own and shares this layout.
 */
function AuthLayout({ children }: { children: ReactNode }) {
  return (
    <LoginBrandPanel languageSwitch={<LanguageSwitch />}>
      {children}
    </LoginBrandPanel>
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
      {/* The pitch is display text; the page's h1 ("Sign in") sits in the form, above the fields. */}
      <div className="flex flex-col gap-3">
        <BrandEyebrow />
        <p className={cn(headlineClass, "whitespace-pre-line")}>
          {t("pages.login.brand.headline")}
        </p>
        <p className="text-base text-muted-foreground">
          {t("pages.login.brand.subtitle")}
        </p>
      </div>
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

/** No design frame: the Login layout with the register form in the brand column (UI-02). */
export function Register() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const heading = useAuthHeading(t("nav.register"));
  return (
    <AuthLayout>
      <div className="flex flex-col gap-3">
        <BrandEyebrow />
        <h1
          ref={heading}
          tabIndex={-1}
          className={cn(headlineClass, "outline-none")}
        >
          {t("pages.register.title")}
        </h1>
        <p className="text-base text-muted-foreground">
          {t("pages.register.description")}
        </p>
      </div>
      <RegisterForm
        onRegistered={() => {
          const state: RegisteredState = { notice: "registered" };
          void navigate("/login", { replace: true, state });
        }}
      />
    </AuthLayout>
  );
}
