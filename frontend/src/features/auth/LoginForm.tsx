import { useEffect, useRef, useState, type FormEvent, type Ref } from "react";
import { CircleCheck, Info } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { ApiError } from "@/shared/api";
import { Button } from "@/shared/ui/button";
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/shared/ui/field";
import { Input } from "@/shared/ui/input";
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
      className="flex w-full max-w-[380px] flex-col gap-6"
      onSubmit={(e) => void onSubmit(e)}
    >
      <FieldGroup>
        {/* Notice slot above the heading, as in Pencil ZziSC. Both kinds stay polite (`status`):
            they describe how the visitor arrived, not an error of this form. */}
        {notice && (
          <p
            role="status"
            data-notice={notice.kind}
            className={
              notice.kind === "registered"
                ? "flex items-center gap-2.5 rounded-lg border border-success/25 bg-success/10 p-3 text-[13px] text-success"
                : "flex items-center gap-2.5 rounded-lg border bg-muted p-3 text-[13px] text-foreground"
            }
          >
            {notice.kind === "registered" ? (
              <CircleCheck aria-hidden="true" className="size-4 shrink-0" />
            ) : (
              <Info aria-hidden="true" className="size-4 shrink-0" />
            )}
            {notice.text}
          </p>
        )}
        <div className="flex flex-col gap-1.5">
          <h1
            ref={heading}
            tabIndex={-1}
            className="text-[28px] leading-tight font-bold tracking-tight outline-none"
          >
            {t("nav.signIn")}
          </h1>
          <p className="text-sm text-muted-foreground">
            {t("pages.login.description")}
          </p>
        </div>
        {error !== null && (
          <p
            key={errorKey}
            ref={alert}
            role="alert"
            tabIndex={-1}
            className="rounded-md border border-destructive p-3 text-sm text-destructive outline-none"
          >
            {error}
          </p>
        )}
        {wasThrottled && !throttled && (
          <p role="status" className="text-sm">
            {t("pages.login.canRetry")}
          </p>
        )}
        <Field>
          <FieldLabel htmlFor="empNo">{t("fields.empNo")}</FieldLabel>
          <Input
            id="empNo"
            name="empNo"
            autoComplete="username"
            inputMode="numeric"
            required
            value={empNo}
            onChange={(e) => setEmpNo(e.target.value)}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor="password">{t("fields.password")}</FieldLabel>
          <Input
            id="password"
            name="password"
            type="password"
            autoComplete="current-password"
            required
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </Field>
        <Field>
          <Button
            type="submit"
            size="lg"
            disabled={pending || throttled}
            aria-busy={pending || undefined}
          >
            {pending
              ? t("pages.login.submitting")
              : throttled
                ? t("pages.login.retryIn", { seconds: countdown.remaining })
                : t("nav.signIn")}
          </Button>
          <FieldDescription className="text-center">
            {t("pages.login.noAccount")}{" "}
            <Link
              to="/register"
              className="font-semibold text-primary underline underline-offset-4"
            >
              {t("pages.login.createAccount")}
            </Link>
          </FieldDescription>
        </Field>
      </FieldGroup>
    </form>
  );
}
