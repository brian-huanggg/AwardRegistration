import { useId, type ReactNode } from 'react';
import { cn } from './utils';

const control = 'w-full rounded-md border bg-background px-3 py-2 text-sm';

/**
 * Eco field look (Pencil field box): warm fill, 1.5px border, 14px radius, 52px tall.
 * Exported so selects / textareas outside `Field` can share it.
 */
export const ecoControl =
  'w-full min-h-[52px] rounded-[14px] border-[1.5px] border-input bg-field px-[18px] py-3 text-base text-foreground placeholder:text-muted-foreground outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/30 aria-invalid:border-destructive disabled:cursor-not-allowed disabled:opacity-60';
/** Eco field label: small uppercase, tracked, muted. */
export const ecoLabel = 'mb-2 block text-xs font-semibold tracking-[0.12em] text-muted-foreground uppercase';

interface FieldProps {
  label: string;
  hint?: string;
  error?: string;
  /** `eco`: the Pencil "Eco" look (uppercase small label, warm rounded input). */
  variant?: 'default' | 'eco';
  children: (props: { id: string; 'aria-describedby'?: string; 'aria-invalid'?: true; className: string }) => ReactNode;
}

/** Label + control + hint/error, wired with ids so the control is always named (WCAG 1.3.1, 3.3.2). */
export function Field({ label, hint, error, variant = 'default', children }: FieldProps) {
  const eco = variant === 'eco';
  const id = useId();
  const describedBy = [hint && `${id}-hint`, error && `${id}-error`].filter(Boolean).join(' ') || undefined;
  return (
    <div className="mb-4">
      <label htmlFor={id} className={eco ? ecoLabel : 'mb-1 block text-sm font-medium'}>
        {label}
      </label>
      {children({ id, 'aria-describedby': describedBy, 'aria-invalid': error ? true : undefined, className: cn(eco ? ecoControl : control) })}
      {hint && (
        <p id={`${id}-hint`} className="mt-1 text-sm text-muted-foreground">
          {hint}
        </p>
      )}
      {error && (
        <p id={`${id}-error`} role="alert" className="mt-1 text-sm text-destructive">
          {error}
        </p>
      )}
    </div>
  );
}
