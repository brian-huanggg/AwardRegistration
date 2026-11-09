import { ArrowDownWideNarrow, Check } from 'lucide-react';
import { DropdownMenu } from 'radix-ui';
import { useTranslation } from 'react-i18next';
import { Button } from '@/shared/ui/button';
import type { Order, Sort } from './list';

/** The four API orderings offered by the design's sort button (UI-03). */
export const SORT_OPTIONS = [
  { key: 'newest', sort: 'createdAt', order: 'desc' },
  { key: 'oldest', sort: 'createdAt', order: 'asc' },
  { key: 'nameAsc', sort: 'name', order: 'asc' },
  { key: 'nameDesc', sort: 'name', order: 'desc' },
] as const satisfies readonly { key: string; sort: Sort; order: Order }[];

export function SortMenu({ sort, order, onChange }: { sort: Sort; order: Order; onChange: (v: { sort: Sort; order: Order }) => void }) {
  const { t } = useTranslation();
  const current = SORT_OPTIONS.find((o) => o.sort === sort && o.order === order) ?? SORT_OPTIONS[0];
  return (
    <DropdownMenu.Root modal={false}>
      <DropdownMenu.Trigger asChild>
        <Button type="button" variant="outline" size="sm" aria-label={`${t('pages.ideaList.sort.label')}: ${t(`pages.ideaList.sort.${current.key}`)}`}>
          <ArrowDownWideNarrow aria-hidden="true" />
          {t(`pages.ideaList.sort.${current.key}`)}
        </Button>
      </DropdownMenu.Trigger>
      <DropdownMenu.Portal>
        <DropdownMenu.Content align="end" sideOffset={4} className="z-50 min-w-44 rounded-md border bg-popover p-1 text-popover-foreground shadow-md">
          <DropdownMenu.RadioGroup
            value={current.key}
            onValueChange={(k) => {
              const o = SORT_OPTIONS.find((x) => x.key === k);
              if (o) onChange({ sort: o.sort, order: o.order });
            }}
          >
            {SORT_OPTIONS.map((o) => (
              <DropdownMenu.RadioItem
                key={o.key}
                value={o.key}
                className="relative flex cursor-default items-center rounded-sm py-1.5 pr-2 pl-8 text-sm outline-none select-none data-highlighted:bg-accent data-highlighted:text-accent-foreground"
              >
                <DropdownMenu.ItemIndicator className="absolute left-2 inline-flex">
                  <Check aria-hidden="true" className="size-4" />
                </DropdownMenu.ItemIndicator>
                {t(`pages.ideaList.sort.${o.key}`)}
              </DropdownMenu.RadioItem>
            ))}
          </DropdownMenu.RadioGroup>
        </DropdownMenu.Content>
      </DropdownMenu.Portal>
    </DropdownMenu.Root>
  );
}
