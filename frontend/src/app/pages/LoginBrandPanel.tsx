import type { ReactNode } from "react";
import { Sprout } from "lucide-react";
import { useTranslation } from "react-i18next";
import illustration from "@/shared/assets/login-illustration.webp";
import {
  useDeadline,
  type Deadline,
  type SubmissionState,
} from "@/features/deadline/useDeadline";
import { TEAM_SIZE } from "@/features/team/TeamCards";
import { cn } from "@/shared/ui/utils";

/** Dot per state. It only repeats the state text, so colour never carries meaning alone. */
const DOT: Record<SubmissionState | "loading" | "error", string> = {
  OPEN: "bg-brand-forest",
  NOT_OPEN: "border-2 border-brand-forest",
  CLOSED: "bg-muted-foreground",
  loading: "border-2 border-muted-foreground",
  error: "border-2 border-muted-foreground",
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

const DAY_MS = 86_400_000;

/**
 * Whole days left, measured against the server's own `serverTime` of the same response, never the
 * client clock. Null when no deadline is set; 0 once it has passed.
 */
export function daysUntilDeadline(d: Deadline): number | null {
  if (!d.deadline) return null;
  const ms = Date.parse(d.deadline) - Date.parse(d.serverTime);
  return Number.isFinite(ms) ? Math.max(0, Math.ceil(ms / DAY_MS)) : null;
}

/** Pencil `SxCah` "Brand": the round forest logo mark with the lime sprout, then the wordmark. */
export function BrandMark() {
  const { t } = useTranslation();
  return (
    <div className="flex items-center gap-3">
      <span
        aria-hidden="true"
        className="flex size-[34px] shrink-0 items-center justify-center rounded-full bg-brand-forest"
      >
        <Sprout className="size-[18px] text-accent-lime" />
      </span>
      <p className="text-lg font-semibold tracking-[0.09em] text-brand-ink uppercase">
        {t("app.name")}
      </p>
    </div>
  );
}

/** Pencil `SxCah` "Eyebrow": the purple small-caps line above the big heading. */
export function BrandEyebrow() {
  const { t } = useTranslation();
  return (
    <p className="text-xl tracking-[0.04em] text-brand-purple uppercase sm:text-2xl">
      {t("pages.login.brand.eyebrow")}
    </p>
  );
}

/** Big uppercase heading type (Pencil "Headline": 54 / 55, bold). Shared by the pitch and the register h1. */
export const headlineClass =
  "text-4xl leading-[1.02] font-bold tracking-[0.01em] break-words text-brand-ink uppercase sm:text-5xl lg:text-[54px]";

/**
 * Submission state for visitors (`GET /deadline` is public). The figure is a snapshot of the server
 * clock in the same response; the block never ticks from the client clock.
 */
function DeadlineStatus() {
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
  const days = data && !isError ? daysUntilDeadline(data) : null;
  const stats = [
    {
      value: days ?? "–",
      label: t("pages.login.brand.stats.days"),
      caption: t("pages.login.brand.stats.daysCaption"),
    },
    {
      value: TEAM_SIZE,
      label: t("pages.login.brand.stats.team"),
      caption: t("pages.login.brand.stats.teamCaption"),
    },
    {
      value: 1,
      label: t("pages.login.brand.stats.idea"),
      caption: t("pages.login.brand.stats.ideaCaption"),
    },
  ];
  return (
    <div
      role="region"
      aria-label={t("deadlineBanner.label")}
      data-state={key}
      className="flex flex-col gap-4"
    >
      {/* Caption first in DOM order so each figure is read with its caption (as PageHero). */}
      <dl className="flex flex-wrap items-center gap-x-12 gap-y-4">
        {stats.map((s) => (
          <div key={s.label} className="flex items-center gap-3">
            <dt className="order-last text-sm leading-[18px] text-muted-foreground">
              {s.label}
              <br />
              {s.caption}
            </dt>
            <dd className="text-[44px] leading-none font-medium text-brand-purple">
              {s.value}
            </dd>
          </div>
        ))}
      </dl>
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-[13px] text-muted-foreground">
        <p className="flex items-center gap-2 font-semibold text-foreground">
          <span
            aria-hidden="true"
            className={cn("size-2 shrink-0 rounded-full", DOT[key])}
          />
          {state}
        </p>
        {when && <p>{when}</p>}
      </div>
    </div>
  );
}

/**
 * Visitor page layout (Pencil `SxCah` "Eco — Login"): a 640 px white column (brand, the page's own
 * content, the stats) and the decorative illustration with a curved bottom-left edge filling the
 * rest. Below 1024 px the illustration is dropped and the column takes the full width.
 * `languageSwitch` sits in the brand row on narrow screens and as a white pill top-right from 1024 px.
 */
export function LoginBrandPanel({
  children,
  languageSwitch,
}: {
  children: ReactNode;
  languageSwitch: ReactNode;
}) {
  return (
    <div className="relative flex min-h-screen bg-background">
      <div className="flex w-full min-w-0 flex-col justify-between gap-10 px-4 py-8 sm:px-10 lg:w-[640px] lg:shrink-0 lg:py-12 lg:pr-20 lg:pl-[88px]">
        <div className="flex items-center justify-between gap-4">
          <BrandMark />
          <div className="lg:absolute lg:top-8 lg:right-[88px] lg:z-10 lg:rounded-full lg:bg-background lg:px-2 lg:py-1">
            {languageSwitch}
          </div>
        </div>
        <div className="flex flex-col gap-7">{children}</div>
        <DeadlineStatus />
      </div>
      <div
        aria-hidden="true"
        className="hidden min-h-screen flex-1 rounded-bl-[200px] bg-cover bg-center bg-no-repeat lg:block"
        style={{ backgroundImage: `url(${illustration})` }}
      />
    </div>
  );
}
