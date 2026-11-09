import { Lock, Timer } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { useDeadline } from './useDeadline';

/**
 * Idea-detail deadline alert (UI-04). OPEN: amber "Changes lock on …" with the static deadline
 * (the live countdown waits for F2-02); CLOSED: a neutral "Changes are locked". Nothing otherwise.
 * State is carried by text and icon, never colour alone.
 */
export function DeadlineNotice() {
  const { t, i18n } = useTranslation();
  const { data } = useDeadline();
  if (data?.state !== 'OPEN' && data?.state !== 'CLOSED') return null;

  if (data.state === 'CLOSED')
    return (
      <div role="note" data-state="CLOSED" className="flex gap-3 rounded-xl border bg-muted p-4 text-sm text-foreground">
        <Lock aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
        <div>
          <p className="font-semibold">{t('pages.ideaDetail.locked')}</p>
          <p className="mt-1">{t('pages.ideaDetail.lockedBody')}</p>
        </div>
      </div>
    );

  const title = data.deadline
    ? t('pages.ideaDetail.lockOn', {
        date: new Intl.DateTimeFormat(i18n.language, {
          weekday: 'short',
          year: 'numeric',
          month: 'short',
          day: 'numeric',
          hour: '2-digit',
          minute: '2-digit',
          timeZoneName: 'shortOffset',
        }).format(new Date(data.deadline)),
      })
    : t('pages.ideaDetail.lockNotSet');
  return (
    <div role="note" data-state="OPEN" className="flex gap-3 rounded-xl border border-warning/25 bg-warning/10 p-4 text-sm text-warning">
      <Timer aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
      <div>
        <p className="font-semibold">
          {data.deadline ? <time dateTime={data.deadline}>{title}</time> : title}
        </p>
        <p className="mt-1">{t('pages.ideaDetail.lockBody')}</p>
      </div>
    </div>
  );
}
