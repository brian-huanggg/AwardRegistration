import { useEffect, useRef, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

/** One h1 per page; after a route change focus moves to it and the title updates (Frontend.md §8). */
export function PageHeading({ title }: { title: string }) {
  const { t } = useTranslation();
  const ref = useRef<HTMLHeadingElement>(null);
  useEffect(() => {
    document.title = `${title} · ${t("app.name")}`;
    ref.current?.focus();
  }, [title, t]);
  return (
    <h1
      ref={ref}
      tabIndex={-1}
      className="text-2xl font-semibold outline-none"
    >
      {title}
    </h1>
  );
}

/** Visitor pages (login, register, a visitor's 404) fill the viewport bare; signed-in pages use `AppShell`. */
export function Screen({
  children,
  padded = true,
}: {
  children: ReactNode;
  padded?: boolean;
}) {
  return (
    <main id="main" className={padded ? "min-h-screen space-y-6 p-6 md:p-10" : "min-h-screen"}>
      {children}
    </main>
  );
}
