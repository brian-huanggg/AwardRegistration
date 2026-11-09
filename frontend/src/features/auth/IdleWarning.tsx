import { useEffect, useState, useSyncExternalStore } from "react";
import { Dialog } from "radix-ui";
import { useTranslation } from "react-i18next";
import { apiFetch, lastActivity, subscribeActivity } from "@/shared/api";
import { Button } from "@/shared/ui/button";

/** Warn 2 min before the 30 min idle timeout: the 60 s refresh rule can put the server up to 1 min ahead (OI-11, OI-22). */
export const WARN_AFTER_MS = 28 * 60_000;
export const END_AFTER_MS = 30 * 60_000;

/**
 * Idle-timeout warning (WCAG 2.2.1, Frontend.md §8.1). 28 minutes after the last server request a dialog
 * offers "Continue", which calls `GET /auth/me` (that refreshes the session and restarts both timers).
 * With no answer the session ends at 30 minutes like any other 401 (`onExpire`).
 */
export function IdleWarning({
  signedIn,
  onExpire,
}: {
  signedIn: boolean;
  onExpire: () => void;
}) {
  const { t } = useTranslation();
  const last = useSyncExternalStore(subscribeActivity, lastActivity);
  // The `last` the warning fired for: a newer request makes it stale, which closes the dialog without an effect.
  const [warnedFor, setWarnedFor] = useState<number | null>(null);
  const warning = signedIn && warnedFor === last;

  useEffect(() => {
    if (!signedIn) return;
    const elapsed = Date.now() - last;
    const warn = setTimeout(() => setWarnedFor(last), Math.max(0, WARN_AFTER_MS - elapsed));
    const end = setTimeout(() => {
      setWarnedFor(null);
      onExpire();
    }, Math.max(0, END_AFTER_MS - elapsed));
    return () => {
      clearTimeout(warn);
      clearTimeout(end);
    };
  }, [signedIn, last, onExpire]);

  // A failure needs no handling here: a 401 already ends the session through the global handler.
  const keepAlive = () => void apiFetch("/auth/me").catch(() => undefined);

  return (
    <Dialog.Root open={warning} onOpenChange={(open) => !open && keepAlive()}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-50 bg-black/50" />
        <Dialog.Content
          role="alertdialog"
          className="fixed top-1/2 left-1/2 z-50 w-[calc(100%-2rem)] max-w-md -translate-x-1/2 -translate-y-1/2 rounded-lg border bg-background p-6 shadow-lg"
          onInteractOutside={(e) => e.preventDefault()}
        >
          <Dialog.Title className="text-lg font-semibold">{t("pages.idle.title")}</Dialog.Title>
          <Dialog.Description className="mt-2 text-sm text-muted-foreground">
            {t("pages.idle.body")}
          </Dialog.Description>
          <div className="mt-6 flex justify-end">
            <Button onClick={keepAlive}>{t("pages.idle.continue")}</Button>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
