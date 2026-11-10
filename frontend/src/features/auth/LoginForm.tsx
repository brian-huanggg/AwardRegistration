import { useEffect, useRef, useState, type FormEvent, type Ref } from "react";
import { CircleCheck, Info } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { ApiError } from "@/shared/api";
import { Button } from "@/shared/ui/button";
import { Field } from "@/shared/ui/form-field";
import { cn } from "@/shared/ui/utils";
import { login, type Me } from "./login";
import { useCountdown } from "./useCountdown";

/** A notice carried in router state: account created (success) or session ended (neutral). */
export interface LoginNotice {
  kind: "registered" | "sessionEnded";
  text: string;
}

/**
 * The login form. Every credential failure is the same 401 `INVALID_CREDENTIALS` (PRD §6.5), so
 * it is one form-level message, never attached to a field. Focus moves to the message.
 */
export function LoginForm({
  onLoggedIn,
  notice,
  heading,
}: {
  onLoggedIn: (me: Me) => void;
  notice?: LoginNotice;
  heading?: Ref<HTMLHeadingElement>;
}) {
  const { t } = useTranslation();
  const [empNo, setEmpNo] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [errorKey, setErrorKey] = useState(0);
  const [pending, setPending] = useState(false);
  const alert = useRef<HTMLParagraphElement>(null);
  const countdown = useCountdown();
  const [wasThrottled, setWasThrottled] = useState(false);
  const throttled = countdown.remaining > 0;

  // after the message is in the DOM (WCAG 3.3.1)
  useEffect(() => {
    if (errorKey > 0) alert.current?.focus();
  }, [errorKey]);

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (pending || throttled) return;
    setPending(true);
    setError(null);
    setWasThrottled(false);
    try {
      onLoggedIn(await login({ empNo: empNo.trim(), password }));
    } catch (err) {
      const apiErr =
        err instanceof ApiError
          ? err
          : new ApiError({ status: 0, code: "UNKNOWN" });
      if (apiErr.status === 429 && apiErr.retryAfterSeconds !== undefined) {
        countdown.start(apiErr.retryAfterSeconds);
        setWasThrottled(true);
      }
      setError(
        t(`errors.${apiErr.code}`) +
          (apiErr.requestId
            ? ` ${t("common.requestId", { id: apiErr.requestId })}`
            : ""),
      );
      setErrorKey((k) => k + 1);
      setPassword("");
    } finally {
      setPending(false);
    }
  };

  return (
    <form
      className="flex w-full flex-col gap-[18px]"
      onSubmit={(e) => void onSubmit(e)}
    >
      {/* Notice slot above the heading, as in Pencil SxCah. Both kinds stay polite (`status`):
          they describe how the visitor arrived, not an error of this form. */}
      {notice && (
        <p
          role="status"
          data-notice={notice.kind}
          className={cn(
            "flex items-center gap-2.5 rounded-xl px-4 py-2.5 text-sm",
            notice.kind === "registered"
              ? "bg-accent-lime-tint text-brand-forest"
              : "bg-secondary text-foreground",
          )}
        >
          {notice.kind === "registered" ? (
            <CircleCheck aria-hidden="true" className="size-[18px] shrink-0" />
          ) : (
            <Info aria-hidden="true" className="size-[18px] shrink-0" />
          )}
          {notice.text}
        </p>
      )}
      {/* The design shows no "Sign in" title (the pitch above is display text), so the page's h1
          is visually hidden; it still names the page and takes focus on navigation. */}
      <h1 ref={heading} tabIndex={-1} className="sr-only">
        {t("nav.signIn")}
      </h1>
      {error !== null && (
        <p
          key={errorKey}
          ref={alert}
          role="alert"
          tabIndex={-1}
          className="rounded-xl border border-destructive bg-background px-4 py-3 text-sm text-destructive outline-none focus-visible:ring-[3px] focus-visible:ring-destructive/30"
        >
          {error}
        </p>
      )}
      {wasThrottled && !throttled && (
        <p role="status" className="text-sm">
          {t("pages.login.canRetry")}
        </p>
      )}
      {/* Side by side as in the design; stacked below 480 px. */}
      <div className="grid gap-4 min-[480px]:grid-cols-2 [&>div]:mb-0">
        <Field variant="eco" label={t("fields.empNo")}>
          {(p) => (
            <input
              {...p}
              name="empNo"
              autoComplete="username"
              inputMode="numeric"
              required
              value={empNo}
              onChange={(e) => setEmpNo(e.target.value)}
            />
          )}
        </Field>
        <Field variant="eco" label={t("fields.password")}>
          {(p) => (
            <input
              {...p}
              name="password"
              type="password"
              autoComplete="current-password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          )}
        </Field>
      </div>
      <div className="flex flex-wrap items-center gap-x-6 gap-y-3 pt-2">
        <Button
          type="submit"
          variant="lime"
          size="pill"
          className="h-[50px] px-[30px] text-sm"
          disabled={pending || throttled}
          aria-busy={pending || undefined}
        >
          {pending
            ? t("pages.login.submitting")
            : throttled
              ? t("pages.login.retryIn", { seconds: countdown.remaining })
              : t("nav.signIn")}
        </Button>
        <p className="text-sm text-muted-foreground">
          {t("pages.login.noAccount")}{" "}
          <Link
            to="/register"
            className="rounded-sm font-semibold text-brand-purple underline underline-offset-4 outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
          >
            {t("pages.login.createAccount")}
          </Link>
        </p>
      </div>
    </form>
  );
}
