import { useState } from 'react';
import { AlertDialog } from 'radix-ui';
import { useTranslation } from 'react-i18next';
import { ApiError } from '@/shared/api';
import { Button } from '@/shared/ui/button';
import type { IdeaDetail } from './detail';
import { deleteIdea } from './slides';

/**
 * Delete confirmation (WCAG 3.3.4, Frontend.md §8). Esc and Cancel close it and return focus to the
 * trigger. A 404 on the DELETE means the Idea is already gone, which is what the user asked for.
 */
export function DeleteIdea({
  idea,
  disabled,
  onDeleted,
  onSubmissionRejected,
}: {
  idea: IdeaDetail;
  /** Leader outside the submission window; an Admin is never disabled (Frontend.md §5.4). */
  disabled: boolean;
  onDeleted: () => void;
  onSubmissionRejected?: () => void;
}) {
  const { t, i18n } = useTranslation();
  const [open, setOpen] = useState(false);
  const [pending, setPending] = useState(false);
  const [failure, setFailure] = useState<string | null>(null);

  const confirm = async () => {
    if (pending) return;
    setPending(true);
    setFailure(null);
    try {
      await deleteIdea(idea.id);
      setOpen(false);
      onDeleted();
    } catch (e) {
      if (e instanceof ApiError && e.status === 404) {
        setOpen(false);
        onDeleted();
        return;
      }
      const code = e instanceof ApiError && i18n.exists(`errors.${e.code}`) ? e.code : 'UNKNOWN';
      if (e instanceof ApiError && (e.code === 'SUBMISSION_CLOSED' || e.code === 'SUBMISSION_NOT_OPEN')) onSubmissionRejected?.();
      const id = e instanceof ApiError && e.requestId ? ` ${t('common.requestId', { id: e.requestId })}` : '';
      setFailure(t(`errors.${code}`) + id);
    } finally {
      setPending(false);
    }
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
        <Button variant="destructive" disabled={disabled}>
          {t('pages.ideaDetail.delete')}
        </Button>
      </AlertDialog.Trigger>
      <AlertDialog.Portal>
        <AlertDialog.Overlay className="fixed inset-0 z-50 bg-black/50" />
        <AlertDialog.Content className="fixed top-1/2 left-1/2 z-50 w-[calc(100%-2rem)] max-w-md -translate-x-1/2 -translate-y-1/2 rounded-lg border bg-background p-6 shadow-lg">
          <AlertDialog.Title className="text-lg font-semibold">{t('pages.ideaDetail.deleteTitle')}</AlertDialog.Title>
          <AlertDialog.Description className="mt-2 text-sm text-muted-foreground">
            {t('pages.ideaDetail.deleteBody', { name: idea.name })}
          </AlertDialog.Description>
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
            <Button variant="destructive" disabled={pending} aria-busy={pending || undefined} onClick={() => void confirm()}>
              {pending ? t('pages.ideaDetail.deleting') : t('pages.ideaDetail.deleteConfirm')}
            </Button>
          </div>
        </AlertDialog.Content>
      </AlertDialog.Portal>
    </AlertDialog.Root>
  );
}
