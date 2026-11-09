import { CheckCircle2, Clock, Lock, TriangleAlert, type LucideIcon } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { cn } from '@/shared/ui/utils';
import { useDeadline, type SubmissionState } from './useDeadline';

const ICON = { NOT_OPEN: Clock, OPEN: CheckCircle2, CLOSED: Lock } satisfies Record<SubmissionState, LucideIcon>;

/** Tint per state; the text colour on each tint passes 4.5:1 (index.css). */
const TONE: Record<SubmissionState | 'loading' | 'error', string> = {
  OPEN: 'border-success/25 bg-success/10 text-success',
  NOT_OPEN: 'border-warning/25 bg-warning/10 text-warning',
  CLOSED: 'border-border bg-muted text-foreground',
  loading: 'border-border bg-muted text-muted-foreground',
  error: 'border-border bg-muted text-foreground',
};

/**
 * Compact submission-state pill for the app shell's top bar (UI-01). Same contract as
 * `DeadlineBanner`: a labelled region, state as text + icon (never colour alone), loading and
 * error texts. Shows the formatted deadline until the server-time countdown (F2-02) lands; it
 * never ticks from the client clock. Below 640 px the date is dropped, state text and icon stay.
 */
export function DeadlinePill({ className }: { className?: string }) {
  const { t, i18n } = useTranslation();
  const { data, isPending, isError } = useDeadline();

  const key = isPending ? 'loading' : isError ? 'error' : data.state;
  let Icon: LucideIcon = Clock;
  let state: string;
  let when: string | null = null;
  if (isPending) state = t('deadlineBanner.loading');
  else if (isError) {
    state = t('deadlineBanner.error');
    Icon = TriangleAlert;
  } else {
    Icon = ICON[data.state];
    state = t(`deadlineBanner.${data.state}`);
    when = data.deadline
      ? t('deadlineBanner.until', {
          date: new Intl.DateTimeFormat(i18n.language, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(data.deadline)),
        })
      : t('deadlineBanner.notSet');
  }

  return (
    <div
      role="region"
      aria-label={t('deadlineBanner.label')}
      data-state={key}
      className={cn('inline-flex min-w-0 items-center gap-2 rounded-full border px-3 py-1.5 text-[13px] leading-tight', TONE[key], className)}
    >
      <Icon aria-hidden="true" className="size-3.5 shrink-0" />
      <span className="font-semibold">{state}</span>
      {when && <span className="hidden sm:inline">{when}</span>}
    </div>
  );
}
