import { useId, type ReactNode } from 'react';
import { cn } from './utils';

const control = 'w-full rounded-md border bg-background px-3 py-2 text-sm';

interface FieldProps {
  label: string;
  hint?: string;
  error?: string;
  children: (props: { id: string; 'aria-describedby'?: string; 'aria-invalid'?: true; className: string }) => ReactNode;
}

/** Label + control + hint/error, wired with ids so the control is always named (WCAG 1.3.1, 3.3.2). */
export function Field({ label, hint, error, children }: FieldProps) {
  const id = useId();
  const describedBy = [hint && `${id}-hint`, error && `${id}-error`].filter(Boolean).join(' ') || undefined;
  return (
    <div className="mb-4">
      <label htmlFor={id} className="mb-1 block text-sm font-medium">
        {label}
      </label>
      {children({ id, 'aria-describedby': describedBy, 'aria-invalid': error ? true : undefined, className: cn(control) })}
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
