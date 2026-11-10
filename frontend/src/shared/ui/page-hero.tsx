import type { ReactNode } from 'react';
import { cn } from './utils';

export interface HeroStat {
  /** The big figure. */
  value: ReactNode;
  /** Caption, first line (e.g. "Ideas"). */
  label: string;
  /** Caption, second line (e.g. "in total"). */
  caption?: string;
}

/**
 * Eco page hero (Pencil "Hero"): eyebrow, big uppercase title, subtitle, up to three stat
 * figures, and a decorative illustration on the right with a curved bottom-left edge.
 *
 * `title` is a slot: pass the page's `<PageHeading>` (app/layouts) so the single h1, its focus on
 * navigation and the document title stay where they are; any h1 in the slot gets the hero type.
 * Below 1024 px the illustration is dropped and the text block stacks full width.
 * The stats are a `<dl>` (caption in DOM order first, figure shown first) so each figure keeps its caption.
 */
export function PageHero({
  eyebrow,
  title,
  subtitle,
  stats,
  image,
  actions,
  className,
}: {
  eyebrow?: ReactNode;
  title: ReactNode;
  subtitle?: ReactNode;
  stats?: readonly HeroStat[];
  /** Illustration URL (decorative, rendered with alt=""). */
  image?: string;
  /** Optional content under the subtitle (e.g. buttons). */
  actions?: ReactNode;
  className?: string;
}) {
  return (
    <section data-slot="page-hero" className={cn('flex min-h-[220px] items-stretch bg-background', className)}>
      <div className="flex min-w-0 flex-1 flex-col justify-between gap-6 px-4 py-6 sm:px-8 lg:py-7 xl:px-14">
        <div className="flex flex-col gap-1.5">
          {eyebrow && <p className="text-lg tracking-[0.05em] text-brand-purple uppercase sm:text-xl">{eyebrow}</p>}
          <div className="break-words [&_h1]:text-4xl [&_h1]:leading-[1.06] [&_h1]:font-bold [&_h1]:tracking-[0.01em] [&_h1]:text-brand-ink [&_h1]:uppercase sm:[&_h1]:text-5xl xl:[&_h1]:text-[52px]">
            {title}
          </div>
          {subtitle && <p className="text-base text-muted-foreground">{subtitle}</p>}
          {actions && <div className="mt-3 flex flex-wrap gap-2.5">{actions}</div>}
        </div>
        {stats && stats.length > 0 && (
          <dl className="flex flex-wrap items-center gap-x-12 gap-y-4">
            {stats.slice(0, 3).map((s, i) => (
              <div key={i} className="flex items-center gap-2.5">
                <dt className="order-last text-[13px] leading-4 text-muted-foreground">
                  {s.label}
                  {s.caption && (
                    <>
                      <br />
                      {s.caption}
                    </>
                  )}
                </dt>
                <dd className="text-[38px] leading-none font-medium text-brand-purple">{s.value}</dd>
              </div>
            ))}
          </dl>
        )}
      </div>
      {image && (
        <div
          aria-hidden="true"
          className="hidden w-[45%] max-w-[720px] shrink-0 rounded-bl-[125px] bg-cover bg-center bg-no-repeat lg:block"
          style={{ backgroundImage: `url(${image})` }}
        />
      )}
    </section>
  );
}
