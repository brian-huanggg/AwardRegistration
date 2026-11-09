import { useState } from 'react';
import { Ellipsis } from 'lucide-react';
import { AlertDialog, DropdownMenu } from 'radix-ui';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { ApiError } from '@/shared/api';
import { Button } from '@/shared/ui/button';
import { fetchSlides, IncompleteDownloadError, saveBlob } from './detail';
import type { IdeaSummary } from './list';
import { deleteIdea } from './slides';

const ITEM =
  'flex cursor-default items-center rounded-sm px-2 py-1.5 text-sm outline-none select-none data-highlighted:bg-accent data-highlighted:text-accent-foreground data-disabled:opacity-50';

/**
 * Admin row action menu on the shared list (F6-01, UI-03). Which items exist is decided by the
 * page through `can()`; this component only runs them. Replace needs the expected file name from
 * the detail, so it links to the detail page instead of fetching it here (every Admin detail GET
 * is audited, Frontend.md §6.7).
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
      <DropdownMenu.Root modal={false}>
        <DropdownMenu.Trigger asChild>
          <Button type="button" variant="ghost" size="icon-sm" aria-label={t('pages.ideaList.rowActions', { name: idea.name })} aria-busy={busy || undefined}>
            <Ellipsis aria-hidden="true" />
          </Button>
        </DropdownMenu.Trigger>
        <DropdownMenu.Portal>
          <DropdownMenu.Content align="end" sideOffset={4} className="z-50 min-w-40 rounded-md border bg-popover p-1 text-popover-foreground shadow-md">
            {download && (
              <DropdownMenu.Item className={ITEM} disabled={busy} onSelect={() => void runDownload()}>
                {busy ? t('pages.ideaDetail.downloading') : t('pages.ideaDetail.download')}
              </DropdownMenu.Item>
            )}
            {replace && (
              <DropdownMenu.Item className={ITEM} asChild>
                <Link to={`/ideas/${idea.id}`}>{t('pages.ideaDetail.replace')}</Link>
              </DropdownMenu.Item>
            )}
            {remove && (
              <DropdownMenu.Item className={`${ITEM} text-destructive`} onSelect={() => setConfirm(true)}>
                {t('pages.ideaDetail.delete')}
              </DropdownMenu.Item>
            )}
          </DropdownMenu.Content>
        </DropdownMenu.Portal>
      </DropdownMenu.Root>
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
          <AlertDialog.Content className="fixed top-1/2 left-1/2 z-50 w-[calc(100%-2rem)] max-w-md -translate-x-1/2 -translate-y-1/2 rounded-lg border bg-background p-6 shadow-lg">
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
