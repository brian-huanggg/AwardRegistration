import { Hourglass, Lock } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { useDeadline } from './useDeadline';

/**
 * Idea-detail deadline alert (UI-04; Eco `uwwe8` look: warm tint, hourglass). OPEN: amber "Changes lock on …" with the static deadline
 * (the live countdown waits for F2-02); CLOSED: a neutral "Changes are locked". Nothing otherwise.
 * State is carried by text and icon, never colour alone.
 */
export function DeadlineNotice() {
  const { t, i18n } = useTranslation();
  const { data } = useDeadline();
  if (data?.state !== 'OPEN' && data?.state !== 'CLOSED') return null;

  if (data.state === 'CLOSED')
    return (
      <div role="note" data-state="CLOSED" className="flex gap-3.5 rounded-[18px] bg-muted px-5 py-4 text-foreground sm:px-6">
        <Lock aria-hidden="true" className="mt-0.5 size-[18px] shrink-0 text-brand-purple" />
        <div>
          <p className="text-[15px] font-semibold text-brand-ink">{t('pages.ideaDetail.locked')}</p>
          <p className="mt-1 text-[13px]">{t('pages.ideaDetail.lockedBody')}</p>
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
    <div role="note" data-state="OPEN" className="flex gap-3.5 rounded-[18px] bg-warning/10 px-5 py-4 text-foreground sm:px-6">
      <Hourglass aria-hidden="true" className="mt-0.5 size-[18px] shrink-0 text-warning" />
      <div>
        <p className="text-[15px] font-semibold text-brand-ink">
          {data.deadline ? <time dateTime={data.deadline}>{title}</time> : title}
        </p>
        <p className="mt-1 text-[13px]">{t('pages.ideaDetail.lockBody')}</p>
      </div>
    </div>
  );
}
