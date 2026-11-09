import type { ReactNode } from "react";

/**
 * One card of the Idea form (UI-05, Pencil `UfPqk`): title and description on the left,
 * fields on the right from 1024 px, stacked below. A `<section>` named by its `<h2>`;
 * radio groups inside bring their own `<fieldset>`.
 */
export function FormSection({
  id,
  title,
  description,
  children,
}: {
  id: string;
  title: string;
  /** Ids inside may be referenced by `aria-describedby` of the fields. */
  description?: ReactNode;
  children: ReactNode;
}) {
  const titleId = `${id}-title`;
  return (
    <section
      aria-labelledby={titleId}
      className="grid gap-4 rounded-lg border bg-card p-4 text-card-foreground sm:p-6 lg:grid-cols-[17.5rem_minmax(0,1fr)] lg:gap-8"
    >
      <div className="flex min-w-0 flex-col gap-1">
        <h2 id={titleId} className="text-base font-semibold">
          {title}
        </h2>
        {description && <div className="text-sm text-muted-foreground">{description}</div>}
      </div>
      <div className="flex min-w-0 flex-col gap-4">{children}</div>
    </section>
  );
}
