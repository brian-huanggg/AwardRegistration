import { useId } from 'react';
import { useTranslation } from 'react-i18next';
import { StateBadge } from './StateBadge';
import type { SubmissionState } from './useDeadline';

const STATES: SubmissionState[] = ['NOT_OPEN', 'OPEN', 'CLOSED'];

/** Static explainer of the three submission states (PRD SubmissionGate), shown beside the deadline form. */
export function DeadlineStates() {
  const { t } = useTranslation();
  const id = useId();
  return (
    <section aria-labelledby={id} className="rounded-xl border bg-card p-6 text-card-foreground shadow-sm">
      <h2 id={id} className="font-semibold">
        {t('pages.adminDeadline.states.title')}
      </h2>
      <p className="mt-1 text-sm text-muted-foreground">{t('pages.adminDeadline.states.description')}</p>
      <dl className="mt-4 space-y-4">
        {STATES.map((s) => (
          <div key={s} className="flex flex-col items-start gap-1.5">
            <dt>
              <StateBadge state={s} />
            </dt>
            <dd className="text-sm">{t(`pages.adminDeadline.states.${s}.text`)}</dd>
          </div>
        ))}
      </dl>
    </section>
  );
}
