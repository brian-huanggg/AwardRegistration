import { useRef, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { Button } from "@/shared/ui/button";
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from "@/shared/ui/field";
import { Input } from "@/shared/ui/input";
import { PasswordChecklist } from "./PasswordChecklist";
import {
  mapServerError,
  precheck,
  register,
  REGISTER_FIELDS,
  type FieldProblems,
  type RegisterField,
  type RegisterResult,
} from "./register";

type Values = Record<RegisterField, string>;
const EMPTY: Values = {
  empNo: "",
  name: "",
  email: "",
  password: "",
  confirmPassword: "",
};

const FIELD_ATTRS: Record<
  RegisterField,
  {
    type?: string;
    autoComplete: string;
    maxLength?: number;
    inputMode?: "numeric";
  }
> = {
  empNo: { autoComplete: "username", inputMode: "numeric" },
  name: { autoComplete: "name" },
  email: { type: "email", autoComplete: "email" },
  password: { type: "password", autoComplete: "new-password" },
  confirmPassword: { type: "password", autoComplete: "new-password" },
};

const HINTS: Partial<Record<RegisterField, string>> = {
  empNo: "pages.register.empNoHint",
  email: "pages.register.emailHint",
};

/** The register form: format pre-check, error summary with focus, server errors mapped to fields (Frontend.md §5.6, §8). */
export function RegisterForm({
  onRegistered,
}: {
  onRegistered: (r: RegisterResult) => void;
}) {
  const { t, i18n } = useTranslation();
  const [values, setValues] = useState<Values>(EMPTY);
  const [problems, setProblems] = useState<FieldProblems>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);
  const summary = useRef<HTMLDivElement>(null);
  const form = useRef<HTMLFormElement>(null);
  const [summaryKey, setSummaryKey] = useState(0);

  const message = (code: string): string => {
    if (code === "PASSWORD_MISMATCH") return t("pages.register.mismatch");
    return i18n.exists(`errors.${code}`)
      ? t(`errors.${code}`)
      : t("errors.VALIDATION_FAILED");
  };

  const fail = (p: FieldProblems, rest: string | null) => {
    setProblems(p);
    setFormError(rest);
    setSummaryKey((k) => k + 1);
    // wait for the summary to render, then move focus to it (WCAG 3.3.1)
    queueMicrotask(() => summary.current?.focus());
  };

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (pending) return;
    const { values: body, problems: local } = precheck(values);
    if (Object.keys(local).length > 0) {
      fail(local, null);
      return;
    }
    setPending(true);
    setProblems({});
    setFormError(null);
    try {
      onRegistered(await register(body));
    } catch (err) {
      const { problems: p, rest } = mapServerError(err);
      fail(
        p,
        rest
          ? t(`errors.${rest.code}`) +
              (rest.requestId
                ? ` ${t("common.requestId", { id: rest.requestId })}`
                : "")
          : null,
      );
    } finally {
      setPending(false);
    }
  };

  const focusField =
    (f: RegisterField) => (e: { preventDefault: () => void }) => {
      e.preventDefault();
      form.current?.querySelector<HTMLInputElement>(`#${f}`)?.focus();
    };

  const listed = REGISTER_FIELDS.filter((f) => problems[f] !== undefined);
  const hasSummary = listed.length > 0 || formError !== null;

  return (
    <form ref={form} onSubmit={(e) => void onSubmit(e)} noValidate>
      <FieldGroup>
        {hasSummary && (
          <div
            key={summaryKey}
            ref={summary}
            role="alert"
            tabIndex={-1}
            className="rounded-md border border-destructive p-3 text-sm outline-none focus-visible:ring-[3px] focus-visible:ring-destructive/40"
          >
            <p className="font-medium text-destructive">
              {t("pages.register.summaryTitle")}
            </p>
            {formError !== null && <p>{formError}</p>}
            {listed.length > 0 && (
              <ul className="mt-1 list-disc pl-5">
                {listed.map((f) => (
                  <li key={f}>
                    <a
                      href={`#${f}`}
                      onClick={focusField(f)}
                      className="underline underline-offset-4"
                    >
                      {t(`fields.${f}`)}: {message(problems[f]!)}
                    </a>
                  </li>
                ))}
              </ul>
            )}
          </div>
        )}
        {REGISTER_FIELDS.map((f) => {
          const a = FIELD_ATTRS[f];
          const problem = problems[f];
          const hint = HINTS[f];
          // the password's hint is the live rule checklist (F1b-01)
          const hintId = f === "password" ? "password-rules" : hint && `${f}-hint`;
          const describedBy =
            [hintId, problem && `${f}-error`]
              .filter(Boolean)
              .join(" ") || undefined;
          return (
            <Field key={f} data-invalid={problem ? true : undefined}>
              <FieldLabel htmlFor={f}>{t(`fields.${f}`)}</FieldLabel>
              <Input
                id={f}
                name={f}
                type={a.type}
                autoComplete={a.autoComplete}
                inputMode={a.inputMode}
                required
                aria-required="true"
                aria-invalid={problem ? true : undefined}
                aria-describedby={describedBy}
                value={values[f]}
                onChange={(e) =>
                  setValues((v) => ({ ...v, [f]: e.target.value }))
                }
              />
              {hint && (
                <FieldDescription id={`${f}-hint`}>{t(hint)}</FieldDescription>
              )}
              {f === "password" && (
                <PasswordChecklist
                  id="password-rules"
                  password={values.password}
                  empNo={values.empNo}
                />
              )}
              {problem && (
                <FieldError id={`${f}-error`} role={undefined}>
                  {message(problem)}
                </FieldError>
              )}
            </Field>
          );
        })}
        <Field>
          <Button
            type="submit"
            disabled={pending}
            aria-busy={pending || undefined}
          >
            {pending
              ? t("pages.register.submitting")
              : t("pages.register.submit")}
          </Button>
          <FieldDescription className="text-center">
            {t("pages.register.haveAccount")}{" "}
            <Link to="/login" className="underline underline-offset-4">
              {t("nav.signIn")}
            </Link>
          </FieldDescription>
        </Field>
      </FieldGroup>
    </form>
  );
}
