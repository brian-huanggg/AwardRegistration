import { CheckCircle2, Clock, Lock, TriangleAlert, type LucideIcon } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { cn } from '@/shared/ui/utils';
import { useDeadline, type SubmissionState } from './useDeadline';

const ICON = { NOT_OPEN: Clock, OPEN: CheckCircle2, CLOSED: Lock } satisfies Record<SubmissionState, LucideIcon>;

/** Tint per state; the text colour on each tint passes 4.5:1 (index.css). */
const TONE: Record<SubmissionState | 'loading' | 'error', string> = {
  OPEN: 'bg-accent-lime-tint text-brand-ink [&>svg]:text-brand-forest',
  NOT_OPEN: 'bg-warning/10 text-warning',
  CLOSED: 'bg-muted text-foreground',
  loading: 'bg-muted text-muted-foreground',
  error: 'bg-muted text-foreground',
};

/**
 * Compact submission-state pill for the Eco navbar (Pencil "Deadline" pill). Same contract as
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
      className={cn('inline-flex min-w-0 items-center gap-2 rounded-full px-3.5 py-2 text-xs leading-tight tracking-[0.04em]', TONE[key], className)}
    >
      <Icon aria-hidden="true" className="size-3.5 shrink-0" />
      <span className="font-semibold">{state}</span>
      {when && <span className="hidden sm:inline">{when}</span>}
    </div>
  );
}
