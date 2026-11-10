import { useState } from 'react';
import { Download, Eye, LoaderCircle, Trash2, Upload } from 'lucide-react';
import { AlertDialog } from 'radix-ui';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { ApiError } from '@/shared/api';
import { Button } from '@/shared/ui/button';
import { fetchSlides, IncompleteDownloadError, saveBlob } from './detail';
import type { IdeaSummary } from './list';
import { deleteIdea } from './slides';

/** Round 34 px outlined icon button (design `kVnXe` row actions). */
const ICON = 'size-[34px] border-[1.5px] p-0';

/**
 * Admin row actions on the shared list (F6-01, UI-03, Eco E5): view, download, replace and delete
 * as round icon buttons, each named after the Idea. Which ones exist is decided by the page through
 * `can()`; this component only runs them. Replace needs the expected file name from the detail, so
 * it links to the detail page instead of fetching it here (every Admin detail GET is audited,
 * Frontend.md §6.7).
 */
export function IdeaRowActions({
  idea,
  download,
  replace,
  remove,
  onDeleted,
}: {
  idea: IdeaSummary;
  download: boolean;
  replace: boolean;
  remove: boolean;
  onDeleted: () => void;
}) {
  const { t, i18n } = useTranslation();
  const [confirm, setConfirm] = useState(false);
  const [pending, setPending] = useState(false);
  const [busy, setBusy] = useState(false);
  const [failure, setFailure] = useState<string | null>(null);
  const [dialogFailure, setDialogFailure] = useState<string | null>(null);

  const message = (e: unknown) => {
    const code = e instanceof ApiError && i18n.exists(`errors.${e.code}`) ? e.code : 'UNKNOWN';
    const id = e instanceof ApiError && e.requestId ? ` ${t('common.requestId', { id: e.requestId })}` : '';
    return t(`errors.${code}`) + id;
  };

  const runDownload = async () => {
    setBusy(true);
    setFailure(null);
    try {
      const { blob, filename } = await fetchSlides(idea.id, idea.name);
      saveBlob(blob, filename);
    } catch (e) {
      setFailure(e instanceof IncompleteDownloadError ? t('pages.ideaDetail.downloadIncomplete') : message(e));
    } finally {
      setBusy(false);
    }
  };

  const runDelete = async () => {
    if (pending) return;
    setPending(true);
    setDialogFailure(null);
    try {
      await deleteIdea(idea.id);
      setConfirm(false);
      onDeleted();
    } catch (e) {
      // a 404 means the Idea is already gone, which is what the Admin asked for
      if (e instanceof ApiError && e.status === 404) {
        setConfirm(false);
        onDeleted();
        return;
      }
      setDialogFailure(message(e));
    } finally {
      setPending(false);
    }
  };

  if (!download && !replace && !remove) return null;
  return (
    <div className="flex flex-col items-end gap-1">
      <div className="flex items-center justify-end gap-2">
        <Button asChild variant="light" size="pill-icon" className={ICON}>
          <Link to={`/ideas/${idea.id}`} aria-label={t('pages.ideaList.view', { name: idea.name })}>
            <Eye aria-hidden="true" />
          </Link>
        </Button>
        {download && (
          <Button
            type="button"
            variant="light"
            size="pill-icon"
            className={ICON}
            aria-label={t('pages.ideaList.downloadRow', { name: idea.name })}
            aria-busy={busy || undefined}
            disabled={busy}
            onClick={() => void runDownload()}
          >
            {busy ? <LoaderCircle aria-hidden="true" className="animate-spin" /> : <Download aria-hidden="true" />}
          </Button>
        )}
        {replace && (
          <Button asChild variant="light" size="pill-icon" className={ICON}>
            <Link to={`/ideas/${idea.id}`} aria-label={t('pages.ideaList.replaceRow', { name: idea.name })}>
              <Upload aria-hidden="true" />
            </Link>
          </Button>
        )}
        {remove && (
          <Button
            type="button"
            variant="danger-outline"
            size="pill-icon"
            className="size-[34px] p-0"
            aria-label={t('pages.ideaList.deleteRow', { name: idea.name })}
            aria-haspopup="dialog"
            onClick={() => setConfirm(true)}
          >
            <Trash2 aria-hidden="true" />
          </Button>
        )}
      </div>
      {failure && (
        <p role="alert" className="text-xs text-destructive">
          {failure}
        </p>
      )}
      <AlertDialog.Root
        open={confirm}
        onOpenChange={(o) => {
          if (pending) return;
          setConfirm(o);
          if (!o) setDialogFailure(null);
        }}
      >
        <AlertDialog.Portal>
          <AlertDialog.Overlay className="fixed inset-0 z-50 bg-black/50" />
          <AlertDialog.Content className="fixed top-1/2 left-1/2 z-50 w-[calc(100%-2rem)] max-w-md -translate-x-1/2 -translate-y-1/2 rounded-[14px] border bg-background p-6 shadow-lg">
            <AlertDialog.Title className="text-lg font-semibold">{t('pages.ideaDetail.deleteTitle')}</AlertDialog.Title>
            <AlertDialog.Description className="mt-2 text-sm text-muted-foreground">{t('pages.ideaDetail.deleteBody', { name: idea.name })}</AlertDialog.Description>
            {dialogFailure && (
              <p role="alert" className="mt-3 text-sm text-destructive">
                {dialogFailure}
              </p>
            )}
            <div className="mt-6 flex justify-end gap-2">
              <AlertDialog.Cancel asChild>
                <Button variant="outline" disabled={pending}>
                  {t('common.cancel')}
                </Button>
              </AlertDialog.Cancel>
              {/* not AlertDialog.Action: it would close the dialog before the request ends */}
              <Button variant="destructive" disabled={pending} aria-busy={pending || undefined} onClick={() => void runDelete()}>
                {pending ? t('pages.ideaDetail.deleting') : t('pages.ideaDetail.deleteConfirm')}
              </Button>
            </div>
          </AlertDialog.Content>
        </AlertDialog.Portal>
      </AlertDialog.Root>
    </div>
  );
}
