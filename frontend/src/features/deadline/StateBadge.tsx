import { useTranslation } from 'react-i18next';
import { Chip } from '@/shared/ui/chip';
import { cn } from '@/shared/ui/utils';
import type { SubmissionState } from './useDeadline';

/**
 * Eco submission-state chip (Pencil ra682): OPEN lime, NOT OPEN lavender, CLOSED red text on white
 * with a border (the design's red-on-pink tint is avoided; Chip has no danger tone). The state is
 * always written out, colour only supports it.
 */
export function StateBadge({ state, className }: { state: SubmissionState; className?: string }) {
  const { t } = useTranslation();
  const label = t(`pages.adminDeadline.states.${state}.badge`);
  if (state === 'CLOSED')
    return (
      <Chip data-state={state} dot={false} className={cn('border border-border bg-background text-destructive', className)}>
        <span aria-hidden="true" className="size-1.5 shrink-0 rounded-full bg-destructive" />
        {label}
      </Chip>
    );
  return (
    <Chip data-state={state} tone={state === 'OPEN' ? 'lime' : 'lavender'} className={className}>
      {label}
    </Chip>
  );
}
