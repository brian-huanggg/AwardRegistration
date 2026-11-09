import { useId, useState, type ReactNode } from 'react';
import { CirclePlus, X } from 'lucide-react';
import { Popover } from 'radix-ui';
import { useTranslation } from 'react-i18next';
import { Button } from '@/shared/ui/button';
import { cn } from '@/shared/ui/utils';

const POPOVER = 'z-50 w-64 rounded-md border bg-popover p-3 text-popover-foreground shadow-md outline-none';

/**
 * The design's dashed "faceted filter" button (shadcn data-table pattern). The selected value is
 * shown inside the button; a separate clear button sits next to it (buttons cannot nest).
 */
function FacetShell({
  label,
  value,
  open,
  onOpenChange,
  onClear,
  children,
}: {
  label: string;
  /** Text of the current selection; empty when nothing is selected. */
  value: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onClear: () => void;
  children: ReactNode;
}) {
  const { t } = useTranslation();
  const selected = value !== '';
  return (
    <div className="inline-flex items-center">
      <Popover.Root open={open} onOpenChange={onOpenChange}>
        <Popover.Trigger asChild>
          <Button type="button" variant="outline" size="sm" className={cn('border-dashed', selected && 'rounded-r-none border-r-0')}>
            <CirclePlus aria-hidden="true" />
            {label}
            {selected && (
              <>
                <span aria-hidden="true" className="h-4 w-px bg-border" />
                <span className="rounded-sm bg-secondary px-1.5 py-0.5 text-xs font-normal text-secondary-foreground">{value}</span>
              </>
            )}
          </Button>
        </Popover.Trigger>
        <Popover.Portal>
          <Popover.Content align="start" sideOffset={4} aria-label={label} className={POPOVER}>
            {children}
          </Popover.Content>
        </Popover.Portal>
      </Popover.Root>
      {selected && (
        <Button
          type="button"
          variant="outline"
          size="icon-sm"
          className="rounded-l-none border-dashed"
          aria-label={t('pages.ideaList.clearFacet', { name: label })}
          onClick={onClear}
        >
          <X aria-hidden="true" />
        </Button>
      )}
    </div>
  );
}

/** Single choice: the API takes one value per filter (UI-03, Design vs contract). */
export function FacetFilter({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: string;
  options: readonly { value: string; label: string }[];
  onChange: (value: string) => void;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const name = useId();
  const pick = (v: string) => {
    onChange(v);
    setOpen(false);
  };
  const all = [{ value: '', label: t('pages.ideaList.all') }, ...options];
  return (
    <FacetShell label={label} value={options.find((o) => o.value === value)?.label ?? ''} open={open} onOpenChange={setOpen} onClear={() => onChange('')}>
      <fieldset>
        <legend className="mb-2 text-sm font-medium">{label}</legend>
        <div className="flex flex-col gap-1">
          {all.map((o) => (
            <label key={o.value || 'all'} className="flex cursor-pointer items-center gap-2 rounded-sm px-2 py-1.5 text-sm hover:bg-accent">
              <input type="radio" name={name} value={o.value} checked={value === o.value} onChange={() => pick(o.value)} className="accent-primary" />
              {o.label}
            </label>
          ))}
        </div>
      </fieldset>
    </FacetShell>
  );
}

/** "Created": a from / to pair of local dates, both inclusive (`createdFrom` / `createdTo` in the API). */
export function DateRangeFilter({
  label,
  from,
  to,
  onChange,
}: {
  label: string;
  from: string;
  to: string;
  onChange: (patch: { from?: string; to?: string }) => void;
}) {
  const { t, i18n } = useTranslation();
  const [open, setOpen] = useState(false);
  const id = useId();
  const fmt = (d: string) => {
    const [y, m, dd] = d.split('-').map(Number) as [number, number, number];
    return new Intl.DateTimeFormat(i18n.language, { dateStyle: 'medium' }).format(new Date(y, m - 1, dd));
  };
  const value = from || to ? `${from ? fmt(from) : '…'} – ${to ? fmt(to) : '…'}` : '';
  const input = 'h-9 w-full rounded-md border border-input bg-transparent px-3 text-sm shadow-xs outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50';
  return (
    <FacetShell label={label} value={value} open={open} onOpenChange={setOpen} onClear={() => onChange({ from: '', to: '' })}>
      <fieldset className="flex flex-col gap-3">
        <legend className="mb-2 text-sm font-medium">{label}</legend>
        <div className="flex flex-col gap-1">
          <label htmlFor={`${id}-from`} className="text-sm">
            {t('pages.ideaList.from')}
          </label>
          <input id={`${id}-from`} type="date" className={input} value={from} max={to || undefined} onChange={(e) => onChange({ from: e.target.value })} />
        </div>
        <div className="flex flex-col gap-1">
          <label htmlFor={`${id}-to`} className="text-sm">
            {t('pages.ideaList.to')}
          </label>
          <input id={`${id}-to`} type="date" className={input} value={to} min={from || undefined} onChange={(e) => onChange({ to: e.target.value })} />
        </div>
      </fieldset>
    </FacetShell>
  );
}
