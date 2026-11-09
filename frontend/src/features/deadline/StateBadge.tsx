import { useTranslation } from 'react-i18next';
import { cn } from '@/shared/ui/utils';
import type { SubmissionState } from './useDeadline';

/** Same tints as the shell pill (UI-01 state tokens); the text on each passes 4.5:1. */
const TONE: Record<SubmissionState, string> = {
  OPEN: 'border-success/25 bg-success/10 text-success',
  NOT_OPEN: 'border-warning/25 bg-warning/10 text-warning',
  CLOSED: 'border-border bg-muted text-foreground',
};

/** Submission-state badge: the state is always written out, colour only supports it. */
export function StateBadge({ state, className }: { state: SubmissionState; className?: string }) {
  const { t } = useTranslation();
  return (
    <span data-state={state} className={cn('inline-flex shrink-0 items-center rounded-md border px-2 py-0.5 text-xs font-semibold tracking-wide', TONE[state], className)}>
      {t(`pages.adminDeadline.states.${state}.badge`)}
    </span>
  );
}
