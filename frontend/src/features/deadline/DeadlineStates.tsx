import { useId } from 'react';
import { useTranslation } from 'react-i18next';
import { cn } from '@/shared/ui/utils';
import { StateBadge } from './StateBadge';
import type { SubmissionState } from './useDeadline';

const STATES: SubmissionState[] = ['NOT_OPEN', 'OPEN', 'CLOSED'];

/** Static explainer of the three submission states (PRD SubmissionGate), shown beside the deadline form. */
export function DeadlineStates() {
  const { t } = useTranslation();
  const id = useId();
  return (
    <section aria-labelledby={id}>
      <h2 id={id} className="text-lg font-bold tracking-[0.03em] text-brand-ink uppercase">
        {t('pages.adminDeadline.states.title')}
      </h2>
      <p className="mt-3 text-sm text-muted-foreground">{t('pages.adminDeadline.states.description')}</p>
      <dl className="mt-5 flex flex-col gap-5">
        {STATES.map((s) => (
          <div
            key={s}
            className={cn('flex flex-col items-start gap-2.5 rounded-[20px] p-5', s === 'OPEN' ? 'bg-accent-lime-tint' : 'border-[1.5px] border-border bg-background')}
          >
            <dt>
              {/* On the lime card the OPEN chip turns white, as in the frame. */}
              <StateBadge state={s} className={s === 'OPEN' ? 'bg-background' : undefined} />
            </dt>
            <dd className="text-sm leading-5 text-foreground">{t(`pages.adminDeadline.states.${s}.text`)}</dd>
          </div>
        ))}
      </dl>
    </section>
  );
}
