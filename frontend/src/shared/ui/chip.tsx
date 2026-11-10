import type { ComponentProps } from 'react';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from './utils';

const chipVariants = cva(
  'inline-flex w-fit shrink-0 items-center gap-1.5 rounded-full px-3 py-[5px] text-[11px] leading-tight font-semibold tracking-[0.11em] whitespace-nowrap uppercase',
  {
    variants: {
      /** Text on each tint passes 4.5:1: lime 7.5:1, lavender 5.1:1, neutral 4.6:1, warning >= 5.9:1. */
      tone: {
        lime: 'bg-accent-lime-tint text-brand-forest',
        lavender: 'bg-brand-lavender text-brand-purple',
        neutral: 'bg-muted text-muted-foreground',
        warning: 'bg-warning/10 text-warning',
      },
    },
    defaultVariants: { tone: 'neutral' },
  },
);

const DOT = {
  lime: 'bg-accent-lime',
  lavender: 'bg-brand-purple-soft',
  neutral: 'bg-muted-foreground',
  warning: 'bg-warning',
} as const;

/**
 * Eco chip / badge (Pencil "Role Chip"): a tinted pill with an optional state dot. The dot is
 * decorative; the text always carries the meaning (never colour alone).
 */
export function Chip({ className, tone = 'neutral', dot = true, children, ...props }: ComponentProps<'span'> & VariantProps<typeof chipVariants> & { dot?: boolean }) {
  return (
    <span data-slot="chip" className={cn(chipVariants({ tone }), className)} {...props}>
      {dot && <span aria-hidden="true" className={cn('size-1.5 shrink-0 rounded-full', DOT[tone ?? 'neutral'])} />}
      {children}
    </span>
  );
}

export { chipVariants };
