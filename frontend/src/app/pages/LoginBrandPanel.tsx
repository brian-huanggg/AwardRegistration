import { Leaf } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useDeadline, type SubmissionState } from "@/features/deadline/useDeadline";
import { cn } from "@/shared/ui/utils";

/** Dot per state. It only repeats the state text, so colour never carries meaning alone. */
const DOT: Record<SubmissionState | "loading" | "error", string> = {
  OPEN: "bg-brand-panel-state",
  NOT_OPEN: "border-2 border-brand-panel-accent",
  CLOSED: "bg-brand-panel-muted/60",
  loading: "border-2 border-brand-panel-muted/60",
  error: "border-2 border-brand-panel-muted/60",
};

/** "Mon, Nov 30, 2026, 18:00 GMT+8" in the user's own time zone (UI-02 "Design vs contract"). */
function formatDeadline(iso: string, locale: string): string {
  return new Intl.DateTimeFormat(locale, {
    weekday: "short",
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    hourCycle: "h23",
    timeZoneName: "shortOffset",
  }).format(new Date(iso));
}

/**
 * Submission state for visitors (`GET /deadline` is public). The ticking countdown cells wait for
 * the server-clock countdown (F2-02, bead i5o.8); the block never ticks from the client clock.
 */
function DeadlineBlock() {
  const { t, i18n } = useTranslation();
  const { data, isPending, isError } = useDeadline();
  const key = isPending ? "loading" : isError ? "error" : data.state;
  const state = isPending
    ? t("deadlineBanner.loading")
    : isError
      ? t("deadlineBanner.error")
      : t(`pages.login.brand.state.${data.state}`);
  const when =
    data && !isError
      ? data.deadline
        ? t(
            data.state === "CLOSED"
              ? "pages.login.brand.closed"
              : "pages.login.brand.closes",
            { date: formatDeadline(data.deadline, i18n.language) },
          )
        : t("deadlineBanner.notSet")
      : null;
  return (
    <div
      role="region"
      aria-label={t("deadlineBanner.label")}
      data-state={key}
      className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 rounded-xl border border-brand-panel-foreground/10 bg-brand-panel-foreground/5 p-4 text-[13px] lg:p-6"
    >
      <p className="flex items-center gap-2 font-semibold">
        <span
          aria-hidden="true"
          className={cn("size-2 shrink-0 rounded-full", DOT[key])}
        />
        {state}
      </p>
      {/* Below 1024 px the stacked panel keeps the state only (UI-02 "Responsive behaviour"). */}
      {when && <p className="hidden text-brand-panel-muted lg:block">{when}</p>}
    </div>
  );
}

/** Left half of the visitor pages (Pencil `ZziSC` "Brand Panel"): brand, pitch and submission state. */
export function LoginBrandPanel() {
  const { t } = useTranslation();
  return (
    <div className="flex flex-col justify-between gap-6 bg-brand-panel bg-[radial-gradient(ellipse_140%_100%_at_85%_10%,color-mix(in_oklch,var(--brand-panel-glow)_40%,transparent),transparent)] p-6 text-brand-panel-foreground md:p-10 lg:min-h-screen lg:gap-10 lg:p-14">
      <div className="flex items-center gap-2.5">
        <div
          aria-hidden="true"
          className="flex size-9 items-center justify-center rounded-[10px] bg-brand-panel-logo text-brand-panel"
        >
          <Leaf className="size-5" />
        </div>
        <p className="font-semibold">{t("app.name")}</p>
      </div>
      <div className="flex flex-col gap-5">
        <p className="hidden text-xs font-semibold tracking-[0.17em] text-brand-panel-accent uppercase lg:block">
          {t("pages.login.brand.eyebrow")}
        </p>
        <p className="text-3xl leading-[1.05] font-bold tracking-tight whitespace-pre-line text-balance lg:text-[56px] lg:tracking-[-0.027em]">
          {t("pages.login.brand.headline")}
        </p>
        <p className="hidden max-w-[460px] leading-relaxed text-brand-panel-muted lg:block">
          {t("pages.login.brand.body")}
        </p>
      </div>
      <DeadlineBlock />
    </div>
  );
}
