import { useState } from 'react';
import { AlertDialog } from 'radix-ui';
import { useTranslation } from 'react-i18next';
import { ApiError } from '@/shared/api';
import { Button } from '@/shared/ui/button';

/** Turns any thrown value into the text shown to the user, with the request ID when there is one. */
export function useErrorText() {
  const { t, i18n } = useTranslation();
  return (e: unknown) => {
    const err = e instanceof ApiError ? e : new ApiError({ status: 0, code: 'UNKNOWN' });
    const key = i18n.exists(`errors.${err.code}`) ? err.code : 'UNKNOWN';
    return t(`errors.${key}`) + (err.requestId ? ` ${t('common.requestId', { id: err.requestId })}` : '');
  };
}

export interface ActionHandlers {
  /** Runs the request. Resolve when it worked; throw the ApiError otherwise. */
  run: () => Promise<void>;
  /** A 404 means the row is already gone (handled elsewhere, or the Idea was deleted): refresh. */
  onGone: () => void;
  /** A state error (409 / 403 SUBMISSION_*) means the page is stale: refresh. */
  onStale: (e: ApiError) => void;
}

async function attempt(h: ActionHandlers, text: (e: unknown) => string): Promise<string | null> {
  try {
    await h.run();
    return null;
  } catch (e) {
    if (e instanceof ApiError && e.status === 404) {
      h.onGone();
      return null;
    }
    if (e instanceof ApiError && (e.status === 409 || e.code === 'SUBMISSION_CLOSED' || e.code === 'SUBMISSION_NOT_OPEN')) h.onStale(e);
    return text(e);
  }
}

/** Approve: not destructive, so no confirmation. */
export function ApproveButton({ name, disabled, ...h }: ActionHandlers & { name: string; disabled: boolean }) {
  const { t } = useTranslation();
  const text = useErrorText();
  const [pending, setPending] = useState(false);
  const [failure, setFailure] = useState<string | null>(null);
  return (
    <>
      <Button
        variant="outline"
        disabled={disabled || pending}
        aria-label={t('pages.ideaDetail.approveNamed', { name })}
        onClick={async () => {
          setPending(true);
          setFailure(null);
          setFailure(await attempt(h, text));
          setPending(false);
        }}
      >
        {t('pages.ideaDetail.approve')}
      </Button>
      {failure && (
        <p role="alert" className="basis-full text-sm text-destructive">
          {failure}
        </p>
      )}
    </>
  );
}

/** Reject, remove and leave: a confirmation dialog (WCAG 3.3.4, Frontend.md §8). */
export function ConfirmAction({
  trigger,
  triggerLabel,
  title,
  body,
  confirm,
  busy,
  disabled,
  ...h
}: ActionHandlers & { trigger: string; triggerLabel: string; title: string; body: string; confirm: string; busy: string; disabled: boolean }) {
  const { t } = useTranslation();
  const text = useErrorText();
  const [open, setOpen] = useState(false);
  const [pending, setPending] = useState(false);
  const [failure, setFailure] = useState<string | null>(null);

  const go = async () => {
    if (pending) return;
    setPending(true);
    setFailure(null);
    const f = await attempt(h, text);
    setPending(false);
    if (f === null) setOpen(false);
    else setFailure(f);
  };

  return (
    <AlertDialog.Root
      open={open}
      onOpenChange={(o) => {
        if (pending) return;
        setOpen(o);
        if (!o) setFailure(null);
      }}
    >
      <AlertDialog.Trigger asChild>
        <Button variant="outline" disabled={disabled} aria-label={triggerLabel}>
          {trigger}
        </Button>
      </AlertDialog.Trigger>
      <AlertDialog.Portal>
        <AlertDialog.Overlay className="fixed inset-0 z-50 bg-black/50" />
        <AlertDialog.Content className="fixed top-1/2 left-1/2 z-50 w-[calc(100%-2rem)] max-w-md -translate-x-1/2 -translate-y-1/2 rounded-lg border bg-background p-6 shadow-lg">
          <AlertDialog.Title className="text-lg font-semibold">{title}</AlertDialog.Title>
          <AlertDialog.Description className="mt-2 text-sm text-muted-foreground">{body}</AlertDialog.Description>
          {failure && (
            <p role="alert" className="mt-3 text-sm text-destructive">
              {failure}
            </p>
          )}
          <div className="mt-6 flex justify-end gap-2">
            <AlertDialog.Cancel asChild>
              <Button variant="outline" disabled={pending}>
                {t('common.cancel')}
              </Button>
            </AlertDialog.Cancel>
            {/* not AlertDialog.Action: it would close the dialog before the request ends */}
            <Button variant="destructive" disabled={pending} aria-busy={pending || undefined} onClick={() => void go()}>
              {pending ? busy : confirm}
            </Button>
          </div>
        </AlertDialog.Content>
      </AlertDialog.Portal>
    </AlertDialog.Root>
  );
}
