import { useEffect, useRef, useState, type FormEvent, type Ref } from "react";
import { Leaf } from "lucide-react";
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
  notice?: string;
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
      className="flex w-full max-w-sm flex-col gap-6"
      onSubmit={(e) => void onSubmit(e)}
    >
      <FieldGroup>
        <div className="flex flex-col items-center gap-1 text-center">
          <div
            aria-hidden="true"
            className="mb-2 flex size-14 items-center justify-center rounded-2xl bg-primary text-primary-foreground"
          >
            <Leaf className="size-7" />
          </div>
          <h1
            ref={heading}
            tabIndex={-1}
            className="text-2xl font-bold outline-none"
          >
            {t("pages.login.title")}
          </h1>
          <p className="text-sm text-balance text-muted-foreground">
            {t("pages.login.description")}
          </p>
        </div>
        {notice && (
          <p
            role="status"
            className="rounded-md border border-primary p-3 text-sm"
          >
            {notice}
          </p>
        )}
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
            <Link to="/register" className="underline underline-offset-4">
              {t("nav.register")}
            </Link>
          </FieldDescription>
        </Field>
      </FieldGroup>
    </form>
  );
}
