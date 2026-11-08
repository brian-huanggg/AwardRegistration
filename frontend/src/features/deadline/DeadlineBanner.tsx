import { CheckCircle2, Clock, Lock, TriangleAlert } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { useDeadline, type SubmissionState } from './useDeadline';

const ICON = { NOT_OPEN: Clock, OPEN: CheckCircle2, CLOSED: Lock } satisfies Record<SubmissionState, unknown>;

/** Submission-state banner (Frontend.md §5.3, §6.5). State is text + icon, never colour alone. */
export function DeadlineBanner() {
  const { t, i18n } = useTranslation();
  const { data, isPending, isError } = useDeadline();

  let body: string;
  let Icon = Clock;
  if (isPending) body = t('deadlineBanner.loading');
  else if (isError) {
    body = t('deadlineBanner.error');
    Icon = TriangleAlert;
  } else {
    Icon = ICON[data.state];
    const when = data.deadline
      ? t('deadlineBanner.until', {
          date: new Intl.DateTimeFormat(i18n.language, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(data.deadline)),
        })
      : t('deadlineBanner.notSet');
    body = `${t(`deadlineBanner.${data.state}`)} ${when}`;
  }

  return (
    <div role="region" aria-label={t('deadlineBanner.label')} data-state={data?.state ?? (isError ? 'error' : 'loading')} className="flex items-center gap-2 border-b bg-secondary px-4 py-2 text-sm text-secondary-foreground">
      <Icon aria-hidden="true" className="size-4 shrink-0" />
      <span>{body}</span>
    </div>
  );
}
