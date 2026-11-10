import { useId, useState, type ReactNode } from 'react';
import { ChevronDown, X } from 'lucide-react';
import { Popover } from 'radix-ui';
import { useTranslation } from 'react-i18next';
import { Button } from '@/shared/ui/button';
import { cn } from '@/shared/ui/utils';

const POPOVER = 'z-50 w-64 rounded-[14px] border bg-popover p-3 text-popover-foreground shadow-md outline-none';
/** Eco filter pill (design `vF25U` / `kVnXe`): 44 px, 1.5 px outline, small uppercase label. */
const PILL = 'h-11 gap-2 rounded-full px-4 text-xs font-semibold tracking-[0.1em] uppercase';

/**
 * Eco filter pill. Empty: an outlined pill with a chevron. Selected: a dark filled pill reading
 * "LABEL: VALUE" with a clear button inside the same pill (two sibling buttons; buttons cannot nest).
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
    <div className={cn('inline-flex items-center rounded-full', selected && 'bg-brand-ink text-background')}>
      <Popover.Root open={open} onOpenChange={onOpenChange}>
        <Popover.Trigger asChild>
          {/* one element for both looks, so focus returns to it when the popover closes after a pick */}
          <Button
            type="button"
            variant={selected ? 'ghost' : 'light'}
            className={cn(PILL, selected ? 'pr-2 hover:bg-background/10 hover:text-background focus-visible:ring-accent-lime' : 'border-[1.5px]')}
          >
            {selected ? `${label}: ${value}` : label}
            {!selected && <ChevronDown aria-hidden="true" className="size-3.5" />}
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
          variant="ghost"
          size="icon-sm"
          className="mr-1.5 rounded-full hover:bg-background/15 hover:text-background focus-visible:ring-accent-lime"
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
