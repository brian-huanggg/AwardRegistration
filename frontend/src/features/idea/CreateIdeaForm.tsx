import { useEffect, useRef, useState, type FormEvent, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/shared/ui/button";
import { CopyButton } from "@/shared/ui/copy-button";
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from "@/shared/ui/field";
import { Input } from "@/shared/ui/input";
import { NativeSelect } from "@/shared/ui/native-select";
import { Textarea } from "@/shared/ui/textarea";
import { SLIDE_EXTS } from "@/shared/slides";
import { cleanName, cleanSummary, countCodePoints } from "@/shared/text";
import {
  CATEGORIES,
  createIdea,
  DIRECTIONS,
  expectedBaseFor,
  IDEA_FIELDS,
  mapServerError,
  NAME_MAX,
  ORGS,
  precheck,
  SUMMARY_MAX,
  type FieldProblems,
  type IdeaDetail,
  type IdeaField,
  type IdeaText,
  type Leader,
} from "./create";

const EMPTY: IdeaText = { category: "", org: "", direction: "", name: "", summary: "" };

const OPTIONS = {
  category: { list: CATEGORIES, key: "enums.category" },
  org: { list: ORGS, key: "enums.org" },
  direction: { list: DIRECTIONS, key: "enums.direction" },
} as const;

/**
 * The create form (Frontend.md §5.5, §5.6, §8): code-point counters, the expected
 * filename with copy buttons, a pre-check in the server's order, an error summary
 * that takes focus, and server errors mapped to fields. The file is never renamed
 * (OI-21): the user names it.
 */
export function CreateIdeaForm({
  leader,
  canSubmit,
  onCreated,
  onSubmissionRejected,
  onOutcomeUnknown,
}: {
  leader: Leader;
  /** False outside the submission window; the server still decides (Frontend.md §6.3 rule 6). */
  canSubmit: boolean;
  onCreated: (idea: IdeaDetail) => void;
  /** A 403 SUBMISSION_* answer: the caller refetches the deadline (Frontend.md §5.3). */
  onSubmissionRejected?: () => void;
  /** The create may have succeeded although no answer arrived (Frontend.md §5.2, F12): the caller refreshes the list. */
  onOutcomeUnknown?: () => void;
}) {
  const { t, i18n } = useTranslation();
  const [values, setValues] = useState<IdeaText>(EMPTY);
  const [files, setFiles] = useState<File[]>([]);
  const [problems, setProblems] = useState<FieldProblems>({});
  const [serverExpected, setServerExpected] = useState<string | undefined>();
  const [formError, setFormError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);
  const [progress, setProgress] = useState<number | null>(null);
  // An earlier attempt ended without a usable answer; a later 409 may be its success (F12).
  const [earlierUncertain, setEarlierUncertain] = useState(false);
  const [summaryKey, setSummaryKey] = useState(0);
  const summary = useRef<HTMLDivElement>(null);
  // WCAG 3.3.1: the error summary takes focus once it is on screen (it is re-keyed on every failed submit).
  useEffect(() => {
    if (summaryKey > 0) summary.current?.focus();
  }, [summaryKey]);
  const form = useRef<HTMLFormElement>(null);

  const expectedBase = expectedBaseFor(values, leader);
  const fileExt = files[0]?.name.split(".").pop()?.toLowerCase();

  const message = (f: IdeaField, code: string): string => {
    const text = i18n.exists(`errors.${code}`) ? t(`errors.${code}`) : t("errors.VALIDATION_FAILED");
    if (f !== "slides" || code !== "FILENAME_MISMATCH") return text;
    const expected =
      serverExpected ??
      (expectedBase && `${expectedBase}.${fileExt === "pptx" ? "pptx" : "pdf"}`);
    return expected ? `${text} ${t("pages.ideaForm.expectedIs", { name: expected })}` : text;
  };

  const fail = (p: FieldProblems, rest: string | null) => {
    setProblems(p);
    setFormError(rest);
    setSummaryKey((k) => k + 1);
  };

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (pending || !canSubmit) return;
    setServerExpected(undefined);
    const { values: body, problems: local } = await precheck(values, files, leader);
    if (Object.keys(local).length > 0) {
      fail(local, null);
      return;
    }
    setPending(true);
    setProgress(0);
    setProblems({});
    setFormError(null);
    try {
      onCreated(await createIdea(body, files[0]!, (loaded, total) => setProgress(Math.round((loaded / total) * 100))));
    } catch (err) {
      const { problems: p, rest, expected } = mapServerError(err);
      setServerExpected(expected);
      if (rest?.code === "SUBMISSION_CLOSED" || rest?.code === "SUBMISSION_NOT_OPEN") onSubmissionRejected?.();
      // 503 SERVICE_UNAVAILABLE or a lost answer: the commit may have happened. Do not claim failure.
      const unknown = rest !== null && (rest.code === "SERVICE_UNAVAILABLE" || rest.status === 0);
      if (unknown) {
        setEarlierUncertain(true);
        onOutcomeUnknown?.();
      }
      const id = rest?.requestId ? ` ${t("common.requestId", { id: rest.requestId })}` : "";
      const duplicate = p.name === "IDEA_DUPLICATE" && earlierUncertain;
      fail(
        p,
        unknown
          ? t("pages.ideaCreate.uncertain") + id
          : duplicate
            ? t("pages.ideaCreate.duplicateAfterUncertain")
            : rest
              ? t(`errors.${rest.code}`) + id
              : null,
      );
    } finally {
      setPending(false);
      setProgress(null);
    }
  };

  const focusField = (f: IdeaField) => (e: { preventDefault: () => void }) => {
    e.preventDefault();
    form.current?.querySelector<HTMLElement>(`#${f}`)?.focus();
  };

  const set = (f: keyof IdeaText) => (e: { target: { value: string } }) =>
    setValues((v) => ({ ...v, [f]: e.target.value }));

  /** Common ARIA wiring of a control; `extra` ids (hint, counter) come first. */
  const aria = (f: IdeaField, ...extra: string[]) => {
    const problem = problems[f];
    return {
      id: f,
      name: f,
      required: true,
      "aria-required": true as const,
      "aria-invalid": problem ? true : undefined,
      "aria-describedby": [...extra, problem && `${f}-error`].filter(Boolean).join(" ") || undefined,
    };
  };

  const wrap = (f: IdeaField, control: ReactNode, below?: ReactNode) => (
    <Field key={f} data-invalid={problems[f] ? true : undefined}>
      <FieldLabel htmlFor={f}>{t(`fields.${f === "name" ? "ideaName" : f}`)}</FieldLabel>
      {control}
      {below}
      {problems[f] && (
        <FieldError id={`${f}-error`} role={undefined}>
          {message(f, problems[f])}
        </FieldError>
      )}
    </Field>
  );

  const counter = (f: "name" | "summary", n: number, max: number) => (
    <FieldDescription id={`${f}-count`} className={n > max ? "text-destructive" : undefined}>
      {t("pages.ideaForm.count", { n, max })}
    </FieldDescription>
  );

  const listed = IDEA_FIELDS.filter((f) => problems[f] !== undefined);
  const hasSummary = listed.length > 0 || formError !== null;
  const nameCount = countCodePoints(cleanName(values.name));
  const summaryCount = countCodePoints(cleanSummary(values.summary));

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
            <p className="font-medium text-destructive">{t("pages.register.summaryTitle")}</p>
            {formError !== null && <p>{formError}</p>}
            {listed.length > 0 && (
              <ul className="mt-1 list-disc pl-5">
                {listed.map((f) => (
                  <li key={f}>
                    <a href={`#${f}`} onClick={focusField(f)} className="underline underline-offset-4">
                      {t(`fields.${f === "name" ? "ideaName" : f}`)}: {message(f, problems[f]!)}
                    </a>
                  </li>
                ))}
              </ul>
            )}
          </div>
        )}

        <div className="flex flex-col gap-1 text-sm">
          <p className="font-medium">{t("fields.leader")}</p>
          <p>
            {leader.name} ({leader.empNo})
          </p>
          <p className="text-muted-foreground">{t("pages.ideaForm.leaderHint")}</p>
        </div>

        {(["category", "org", "direction"] as const).map((f) =>
          wrap(
            f,
            <NativeSelect {...aria(f)} value={values[f]} onChange={set(f)}>
              <option value="">{t("pages.ideaForm.choose")}</option>
              {OPTIONS[f].list.map((code) => (
                <option key={code} value={code}>
                  {t(`${OPTIONS[f].key}.${code}`)}
                </option>
              ))}
            </NativeSelect>,
          ),
        )}

        {wrap(
          "name",
          <Input {...aria("name", "name-count", "name-hint")} autoComplete="off" value={values.name} onChange={set("name")} />,
          <>
            {counter("name", nameCount, NAME_MAX)}
            <FieldDescription id="name-hint">{t("pages.ideaForm.nameHint")}</FieldDescription>
          </>,
        )}

        {wrap(
          "summary",
          <Textarea {...aria("summary", "summary-count")} rows={6} value={values.summary} onChange={set("summary")} />,
          counter("summary", summaryCount, SUMMARY_MAX),
        )}

        <section aria-labelledby="expected-title" className="rounded-md border bg-muted/40 p-3 text-sm">
          <h2 id="expected-title" className="font-medium">
            {t("pages.ideaForm.expectedTitle")}
          </h2>
          {expectedBase === null ? (
            <p className="mt-1 text-muted-foreground">{t("pages.ideaForm.expectedPending")}</p>
          ) : (
            <>
              <p className="mt-1 text-muted-foreground">{t("pages.ideaForm.expectedHint")}</p>
              <ul className="mt-2 flex flex-col gap-2">
                {SLIDE_EXTS.map((ext) => {
                  const name = `${expectedBase}.${ext}`;
                  return (
                    <li key={ext} className="flex items-center gap-2">
                      <code className="min-w-0 flex-1 break-all rounded bg-background px-2 py-1">{name}</code>
                      <CopyButton
                        value={name}
                        label={t("pages.ideaForm.copy", { ext: ext.toUpperCase() })}
                        copiedText={t("pages.ideaForm.copied")}
                        failedText={t("pages.ideaForm.copyFailed")}
                      />
                    </li>
                  );
                })}
              </ul>
            </>
          )}
        </section>

        {wrap(
          "slides",
          <Input
            {...aria("slides", "slides-hint")}
            type="file"
            accept=".pdf,.pptx,application/pdf,application/vnd.openxmlformats-officedocument.presentationml.presentation"
            onChange={(e) => setFiles(Array.from(e.target.files ?? []))}
          />,
          <FieldDescription id="slides-hint">{t("pages.ideaForm.slidesHint")}</FieldDescription>,
        )}

        <Field>
          {progress !== null && (
            <div className="flex items-center gap-2 text-sm">
              <progress
                className="h-2 flex-1"
                max={100}
                value={progress}
                aria-label={t("pages.ideaCreate.uploading")}
              />
              <span aria-hidden="true">{progress}%</span>
            </div>
          )}
          {!canSubmit && <FieldDescription>{t("pages.ideaCreate.closedHint")}</FieldDescription>}
          <Button type="submit" disabled={pending || !canSubmit} aria-busy={pending || undefined}>
            {pending ? t("pages.ideaCreate.submitting") : t("pages.ideaCreate.submit")}
          </Button>
        </Field>
      </FieldGroup>
    </form>
  );
}
