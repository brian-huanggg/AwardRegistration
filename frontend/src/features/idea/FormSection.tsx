import type { ReactNode } from "react";

/**
 * One numbered section of the Idea form (Pencil Eco frame `FIFiM`): a big decorative number,
 * uppercase title and hint on the left, fields on the right from 1024 px, stacked below; sections
 * are separated by a hairline. A `<section>` named by its `<h2>`; radio groups inside bring their
 * own `<fieldset>`.
 */
export function FormSection({
  id,
  index,
  title,
  description,
  children,
}: {
  id: string;
  /** 1-based position, shown as "01", "02"… (decorative, hidden from assistive technology). */
  index: number;
  title: string;
  /** Ids inside may be referenced by `aria-describedby` of the fields. */
  description?: ReactNode;
  children: ReactNode;
}) {
  const titleId = `${id}-title`;
  return (
    <section
      aria-labelledby={titleId}
      className="grid gap-5 border-b border-border py-8 lg:grid-cols-[minmax(0,21.25rem)_minmax(0,1fr)] lg:gap-14"
    >
      <div className="flex min-w-0 flex-col gap-2">
        <span aria-hidden="true" className="text-[44px] leading-none font-medium text-brand-purple-soft">
          {String(index).padStart(2, "0")}
        </span>
        <h2 id={titleId} className="mt-2 text-xl font-bold tracking-[0.025em] text-brand-ink uppercase">
          {title}
        </h2>
        {description && <div className="text-sm leading-[1.5] text-muted-foreground">{description}</div>}
      </div>
      <div className="flex min-w-0 flex-col gap-5">{children}</div>
    </section>
  );
}
