import { useEffect, useRef, useState, type FormEvent, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import {
  CircleAlertIcon,
  CircleCheckIcon,
  FileTextIcon,
  SendIcon,
  SparklesIcon,
  TriangleAlertIcon,
  TrophyIcon,
  UploadIcon,
} from "lucide-react";
import { Button, buttonVariants } from "@/shared/ui/button";
import { CopyButton } from "@/shared/ui/copy-button";
import { Field, FieldDescription, FieldError, FieldLabel } from "@/shared/ui/field";
import { Input } from "@/shared/ui/input";
import { NativeSelect } from "@/shared/ui/native-select";
import { Textarea } from "@/shared/ui/textarea";
import { cn } from "@/shared/ui/utils";
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
  precheckText,
  SUMMARY_MAX,
  updateIdea,
  type FieldProblems,
  type IdeaDetail,
  type IdeaField,
  type IdeaText,
  type Leader,
} from "./create";
import { fileStatus, formatSize, MAX_SLIDES_MIB } from "./fileStatus";
import { FormSection } from "./FormSection";

const EMPTY: IdeaText = { category: "", org: "", direction: "", name: "", summary: "" };

const OPTIONS = {
  org: { list: ORGS, key: "enums.org" },
  direction: { list: DIRECTIONS, key: "enums.direction" },
} as const;

const CATEGORY_ICON = { COOL_IDEA: SparklesIcon, OUTSTANDING_IMPACT: TrophyIcon } as const;

/** A counter is read out only near its limit, not on every keystroke (UI-05). */
const nearLimit = (n: number, max: number) => n >= Math.floor(max * 0.9);

/**
 * The Idea form (Frontend.md §5.5, §5.6, §8), shared by create and edit: code-point counters,
 * a pre-check in the server's order, an error summary that takes focus, and server errors
 * mapped to fields. Create adds the expected filename with copy buttons and the slide file
 * (never renamed, OI-21: the user names it). Edit sends the five text fields only (PUT).
 * Layout: three `FormSection` cards, Pencil frame `UfPqk` (docs/UI-refactor/UI-05).
 */
export function IdeaForm({
  leader,
  edit,
  canSubmit,
  onSaved,
  onCancel,
  onSubmissionRejected,
  onOutcomeUnknown,
}: {
  leader: Leader;
  /** Present = edit mode: starts from this Idea, has no slide file, submits with PUT. */
  edit?: IdeaDetail;
  /** False outside the submission window; the server still decides (Frontend.md §6.3 rule 6). */
  canSubmit: boolean;
  onSaved: (idea: IdeaDetail) => void;
  /** Where Cancel goes (a link styled as a button), rendered before the submit button. */
  onCancel?: ReactNode;
  /** A 403 SUBMISSION_* answer: the caller refetches the deadline (Frontend.md §5.3). */
  onSubmissionRejected?: () => void;
  /** The create may have succeeded although no answer arrived (Frontend.md §5.2, F12): the caller refreshes the list. */
  onOutcomeUnknown?: () => void;
}) {
  const { t, i18n } = useTranslation();
  const [values, setValues] = useState<IdeaText>(
    edit ? { category: edit.category, org: edit.org, direction: edit.direction, name: edit.name, summary: edit.summary } : EMPTY,
  );
  const [files, setFiles] = useState<File[]>([]);
  const [problems, setProblems] = useState<FieldProblems>({});
  const [serverExpected, setServerExpected] = useState<string | undefined>();
  const [formError, setFormError] = useState<string | null>(null);
  // "Result uncertain" texts (Frontend.md §5.2): an alert above the actions, not an error summary entry.
  const [notice, setNotice] = useState<string | null>(null);
  const [pending, setPending] = useState(false);
  const [progress, setProgress] = useState<number | null>(null);
  // An earlier attempt ended without a usable answer; a later 409 may be its success (F12).
  const [earlierUncertain, setEarlierUncertain] = useState(false);
  const [summaryKey, setSummaryKey] = useState(0);
  const summary = useRef<HTMLDivElement>(null);
  const noticeRef = useRef<HTMLDivElement>(null);
  // WCAG 3.3.1: the error summary (or, without one, the notice) takes focus once it is on screen (re-keyed on every failed submit).
  useEffect(() => {
    if (summaryKey > 0) (summary.current ?? noticeRef.current)?.focus();
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

  const fail = (p: FieldProblems, rest: string | null, note: string | null = null) => {
    setProblems(p);
    setFormError(rest);
    setNotice(note);
    setSummaryKey((k) => k + 1);
  };

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (pending || !canSubmit) return;
    setServerExpected(undefined);
    const { values: body, problems: local } = edit ? precheckText(values, leader) : await precheck(values, files, leader);
    if (Object.keys(local).length > 0) {
      fail(local, null);
      return;
    }
    setPending(true);
    setProgress(0);
    setProblems({});
    setFormError(null);
    setNotice(null);
    try {
      onSaved(
        edit
          ? await updateIdea(edit.id, body)
          : await createIdea(body, files[0]!, (loaded, total) => setProgress(Math.round((loaded / total) * 100))),
      );
    } catch (err) {
      const { problems: p, rest, expected } = mapServerError(err);
      setServerExpected(expected);
      if (rest?.code === "SUBMISSION_CLOSED" || rest?.code === "SUBMISSION_NOT_OPEN") onSubmissionRejected?.();
      // 503 SERVICE_UNAVAILABLE or a lost answer: the commit may have happened. Do not claim failure.
      // (A repeated PUT is harmless, so edit shows the plain error and the user simply saves again.)
      const unknown = !edit && rest !== null && (rest.code === "SERVICE_UNAVAILABLE" || rest.status === 0);
      if (unknown) {
        setEarlierUncertain(true);
        onOutcomeUnknown?.();
      }
      const id = rest?.requestId ? ` ${t("common.requestId", { id: rest.requestId })}` : "";
      const duplicate = p.name === "IDEA_DUPLICATE" && earlierUncertain;
      fail(
        p,
        !unknown && !duplicate && rest ? t(`errors.${rest.code}`) + id : null,
        unknown
          ? t("pages.ideaCreate.uncertain") + id
          : duplicate
            ? t("pages.ideaCreate.duplicateAfterUncertain")
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

  const label = (f: IdeaField) => t(`fields.${f === "name" ? "ideaName" : f}`);

  const fieldError = (f: IdeaField) =>
    problems[f] && (
      <FieldError id={`${f}-error`} role={undefined}>
        {message(f, problems[f])}
      </FieldError>
    );

  const wrap = (f: IdeaField, control: ReactNode, below?: ReactNode) => (
    <Field key={f} data-invalid={problems[f] ? true : undefined} className="min-w-0 gap-2">
      <FieldLabel htmlFor={f}>{label(f)}</FieldLabel>
      {control}
      {below}
      {fieldError(f)}
    </Field>
  );

  const counter = (f: "name" | "summary", n: number, max: number) => (
    <>
      <FieldDescription id={`${f}-count`} className={cn("text-xs", n > max && "text-destructive")}>
        {t("pages.ideaForm.count", { n, max })}
      </FieldDescription>
      <span className="sr-only" aria-live="polite">
        {nearLimit(n, max) ? t("pages.ideaForm.count", { n, max }) : ""}
      </span>
    </>
  );

  const listed = IDEA_FIELDS.filter((f) => problems[f] !== undefined);
  const hasSummary = listed.length > 0 || formError !== null;
  const nameCount = countCodePoints(cleanName(values.name));
  const summaryCount = countCodePoints(cleanSummary(values.summary));
  const file = files[0];
  const status = fileStatus(file, expectedBase);

  const categoryCards = (
    <fieldset
      className="flex min-w-0 flex-col gap-2"
      aria-describedby={problems.category ? "category-error" : undefined}
    >
      <legend className={cn("mb-2 text-sm font-medium", problems.category && "text-destructive")}>{label("category")}</legend>
      <div className="grid gap-3 sm:grid-cols-2">
        {CATEGORIES.map((code, i) => {
          const Icon = CATEGORY_ICON[code];
          // The first radio carries the field id, so the error summary link can focus it.
          const radioId = i === 0 ? "category" : `category-${code}`;
          return (
            <label
              key={code}
              className={cn(
                "group/card flex cursor-pointer items-start gap-3 rounded-lg border bg-background p-3.5 transition-colors",
                "has-checked:border-primary has-checked:bg-primary/5 has-focus-visible:ring-[3px] has-focus-visible:ring-ring/50",
                problems.category && "border-destructive",
              )}
            >
              <Icon aria-hidden="true" className="mt-0.5 size-[18px] shrink-0 text-muted-foreground group-has-checked/card:text-primary" />
              <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                <span id={`${radioId}-label`} className="text-sm font-semibold text-foreground">
                  {t(`enums.category.${code}`)}
                </span>
                <span id={`${radioId}-hint`} className="text-xs text-muted-foreground">
                  {t(`pages.ideaForm.categoryHint.${code}`)}
                </span>
              </span>
              <input
                type="radio"
                id={radioId}
                name="category"
                value={code}
                required
                checked={values.category === code}
                onChange={set("category")}
                aria-labelledby={`${radioId}-label`}
                aria-describedby={[`${radioId}-hint`, problems.category && "category-error"].filter(Boolean).join(" ")}
                className="mt-0.5 size-4 shrink-0 accent-primary outline-none"
              />
            </label>
          );
        })}
      </div>
      {fieldError("category")}
    </fieldset>
  );

  const fileRow = (
    <Field data-invalid={problems.slides ? true : undefined} className="gap-2">
      <FieldLabel htmlFor="slides">{label("slides")}</FieldLabel>
      <div className="flex flex-col gap-2.5 rounded-lg border p-3.5">
        <div className="flex flex-wrap items-center gap-3">
          <input
            {...aria("slides", "slides-hint", "slides-status")}
            type="file"
            className="peer sr-only"
            accept=".pdf,.pptx,application/pdf,application/vnd.openxmlformats-officedocument.presentationml.presentation"
            onChange={(e) => setFiles(Array.from(e.target.files ?? []))}
          />
          <label
            htmlFor="slides"
            className={cn(
              buttonVariants({ variant: "outline", size: "sm" }),
              "cursor-pointer peer-focus-visible:border-ring peer-focus-visible:ring-[3px] peer-focus-visible:ring-ring/50",
            )}
          >
            <UploadIcon aria-hidden="true" />
            {t("pages.ideaForm.choose")}
          </label>
          {file && (
            <div className="flex min-w-0 flex-1 items-center gap-3">
              <FileTextIcon aria-hidden="true" className="size-5 shrink-0 text-muted-foreground" />
              <div className="flex min-w-0 flex-col">
                <span className="text-sm font-medium break-all">{file.name}</span>
                <span className="text-xs text-muted-foreground">
                  {t("pages.ideaForm.fileSize", { size: formatSize(file.size, i18n.language), max: MAX_SLIDES_MIB })}
                </span>
              </div>
            </div>
          )}
        </div>
        <p id="slides-status" role="status" className="text-xs empty:hidden">
          {status.kind === "none" && <span className="text-muted-foreground">{t("pages.ideaForm.noFile")}</span>}
          {status.kind === "match" && (
            <span className="inline-flex items-center gap-1 font-medium text-success">
              <CircleCheckIcon aria-hidden="true" className="size-3.5" />
              {t("pages.ideaForm.fileMatches")}
            </span>
          )}
          {status.kind === "mismatch" && (
            <span className="inline-flex items-start gap-1 text-destructive">
              <CircleAlertIcon aria-hidden="true" className="mt-px size-3.5 shrink-0" />
              <span className="break-all">
                {t("errors.FILENAME_MISMATCH")} {t("pages.ideaForm.expectedIs", { name: status.expected })}
              </span>
            </span>
          )}
          {status.kind === "error" && (
            <span className="inline-flex items-center gap-1 text-destructive">
              <CircleAlertIcon aria-hidden="true" className="size-3.5 shrink-0" />
              {t(`errors.${status.code}`)}
            </span>
          )}
        </p>
        {progress !== null && (
          <div className="flex items-center gap-2 text-sm">
            <progress
              className="h-2 flex-1 accent-primary"
              max={100}
              value={progress}
              aria-label={t("pages.ideaCreate.uploading")}
            />
            <span aria-hidden="true" className="tabular-nums">
              {progress}%
            </span>
          </div>
        )}
      </div>
      {fieldError("slides")}
    </Field>
  );

  const expectedBox = (
    <section aria-labelledby="expected-title" className="flex flex-col gap-1.5 rounded-lg bg-muted p-3.5 text-sm">
      <h3 id="expected-title" className="text-xs text-muted-foreground">
        {t("pages.ideaForm.expectedTitle")}
      </h3>
      {expectedBase === null ? (
        <p className="text-muted-foreground">{t("pages.ideaForm.expectedPending")}</p>
      ) : (
        <>
          <p className="text-xs text-muted-foreground">{t("pages.ideaForm.expectedHint")}</p>
          <ul className="flex flex-col gap-2">
            {SLIDE_EXTS.map((ext) => {
              const name = `${expectedBase}.${ext}`;
              return (
                <li key={ext} className="flex flex-wrap items-center gap-2">
                  <code className="min-w-0 flex-1 font-semibold break-all text-foreground">{name}</code>
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
  );

  return (
    <form ref={form} onSubmit={(e) => void onSubmit(e)} noValidate className="flex flex-col gap-6">
      {hasSummary && (
        <div
          key={summaryKey}
          ref={summary}
          role="alert"
          tabIndex={-1}
          className="rounded-md border border-destructive bg-card p-3 text-sm outline-none focus-visible:ring-[3px] focus-visible:ring-destructive/40"
        >
          <p className="font-medium text-destructive">{t("pages.register.summaryTitle")}</p>
          {formError !== null && <p>{formError}</p>}
          {listed.length > 0 && (
            <ul className="mt-1 list-disc pl-5">
              {listed.map((f) => (
                <li key={f}>
                  <a href={`#${f}`} onClick={focusField(f)} className="underline underline-offset-4">
                    {label(f)}: {message(f, problems[f]!)}
                  </a>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}

      <div className="flex flex-col gap-1 text-sm">
        {!edit && <p className="text-muted-foreground">{t("pages.ideaForm.leaderHint")}</p>}
        <p>
          <span className="font-medium">{t("fields.leader")}</span>{" "}
          <span>
            {leader.name} ({leader.empNo})
          </span>
        </p>
      </div>

      <FormSection
        id="classification"
        title={t("pages.ideaForm.sections.classification")}
        description={<p>{t("pages.ideaForm.sections.classificationHint")}</p>}
      >
        {categoryCards}
        <div className="grid gap-4 sm:grid-cols-2">
          {(["org", "direction"] as const).map((f) =>
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
        </div>
      </FormSection>

      <FormSection
        id="about"
        title={t("pages.ideaForm.sections.about")}
        description={
          <>
            <p>{t("pages.ideaForm.sections.aboutHint", { name: NAME_MAX, summary: SUMMARY_MAX })}</p>
            <p id="name-hint">{t("pages.ideaForm.nameHint")}</p>
          </>
        }
      >
        {wrap(
          "name",
          <Input {...aria("name", "name-count", "name-hint")} autoComplete="off" value={values.name} onChange={set("name")} />,
          counter("name", nameCount, NAME_MAX),
        )}
        {wrap(
          "summary",
          <Textarea {...aria("summary", "summary-count")} rows={6} value={values.summary} onChange={set("summary")} />,
          counter("summary", summaryCount, SUMMARY_MAX),
        )}
      </FormSection>

      <FormSection
        id="slides-section"
        title={t("pages.ideaForm.sections.slides")}
        description={edit ? undefined : <p id="slides-hint">{t("pages.ideaForm.slidesHint")}</p>}
      >
        {edit ? (
          <p className="text-sm text-muted-foreground">{t("pages.ideaEdit.slidesNote")}</p>
        ) : (
          <>
            {expectedBox}
            {fileRow}
          </>
        )}
      </FormSection>

      {notice !== null && (
        <div
          key={`notice-${summaryKey}`}
          ref={noticeRef}
          role="alert"
          tabIndex={-1}
          className="flex items-start gap-2 rounded-md border border-warning bg-card p-3 text-sm outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
        >
          <TriangleAlertIcon aria-hidden="true" className="mt-0.5 size-4 shrink-0 text-warning" />
          <p>{notice}</p>
        </div>
      )}

      <div className="flex flex-col gap-2">
        {!canSubmit && (
          <p className="text-sm text-muted-foreground sm:text-right">
            {t(edit ? "pages.ideaEdit.closedHint" : "pages.ideaCreate.closedHint")}
          </p>
        )}
        <div className="flex flex-col gap-2 sm:flex-row sm:justify-end [&>*]:w-full sm:[&>*]:w-auto">
          {onCancel}
          <Button type="submit" disabled={pending || !canSubmit} aria-busy={pending || undefined}>
            {!edit && <SendIcon aria-hidden="true" />}
            {edit
              ? pending
                ? t("pages.ideaEdit.saving")
                : t("common.save")
              : pending
                ? t("pages.ideaCreate.submitting")
                : t("pages.ideaCreate.submit")}
          </Button>
        </div>
      </div>
    </form>
  );
}
