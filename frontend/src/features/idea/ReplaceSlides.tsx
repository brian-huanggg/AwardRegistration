import { useState, type FormEvent } from 'react';
import { Dialog } from 'radix-ui';
import { useTranslation } from 'react-i18next';
import { ApiError } from '@/shared/api';
import { precheckSlides } from '@/shared/slides';
import { Button } from '@/shared/ui/button';
import { Input } from '@/shared/ui/input';
import type { IdeaDetail } from './detail';
import { mapServerError } from './create';
import { replaceSlides } from './slides';

/**
 * Replace-slides dialog (Frontend.md §5.5, §6.2). The file is checked against the CURRENT expected
 * filename (the server does the same); the server stays the judge. A 503 SERVICE_UNAVAILABLE or a
 * lost answer is an unknown outcome (§5.2): say so and let the caller refresh the detail first.
 */
export function ReplaceSlides({
  idea,
  disabled,
  onReplaced,
  onSubmissionRejected,
  onOutcomeUnknown,
}: {
  idea: IdeaDetail;
  /** Leader outside the submission window; an Admin is never disabled (Frontend.md §5.4). */
  disabled: boolean;
  onReplaced: (idea: IdeaDetail) => void;
  onSubmissionRejected?: () => void;
  onOutcomeUnknown?: () => void;
}) {
  const { t, i18n } = useTranslation();
  const [open, setOpen] = useState(false);
  const [files, setFiles] = useState<File[]>([]);
  const [problem, setProblem] = useState<string | null>(null);
  const [expected, setExpected] = useState<string | undefined>();
  const [failure, setFailure] = useState<string | null>(null);
  const [pending, setPending] = useState(false);
  const [progress, setProgress] = useState<number | null>(null);

  const reset = () => {
    setFiles([]);
    setProblem(null);
    setExpected(undefined);
    setFailure(null);
  };
  const text = (code: string) => (i18n.exists(`errors.${code}`) ? t(`errors.${code}`) : t('errors.UNKNOWN'));
  const problemText =
    problem === null
      ? null
      : problem === 'FILENAME_MISMATCH'
        ? `${text(problem)} ${t('pages.ideaForm.expectedIs', { name: expected ?? `${idea.expectedFilenameBase}.${idea.slides.ext}` })}`
        : text(problem);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (pending) return;
    setFailure(null);
    setExpected(undefined);
    const local = await precheckSlides(files, idea.expectedFilenameBase);
    if (local !== '') {
      setProblem(local);
      return;
    }
    setProblem(null);
    setPending(true);
    setProgress(0);
    try {
      const saved = await replaceSlides(idea.id, files[0]!, (loaded, total) => setProgress(Math.round((loaded / total) * 100)));
      setOpen(false);
      reset();
      onReplaced(saved);
    } catch (err) {
      const { problems, rest, expected: exp } = mapServerError(err);
      if (problems.slides) {
        setProblem(problems.slides);
        setExpected(exp);
        return;
      }
      if (rest?.code === 'SUBMISSION_CLOSED' || rest?.code === 'SUBMISSION_NOT_OPEN') onSubmissionRejected?.();
      const id = rest?.requestId ? ` ${t('common.requestId', { id: rest.requestId })}` : '';
      if (rest && (rest.code === 'SERVICE_UNAVAILABLE' || rest.status === 0)) {
        onOutcomeUnknown?.();
        setFailure(t('pages.ideaDetail.replaceUncertain') + id);
      } else setFailure(text(rest instanceof ApiError ? rest.code : 'UNKNOWN') + id);
    } finally {
      setPending(false);
      setProgress(null);
    }
  };

  return (
    <Dialog.Root
      open={open}
      onOpenChange={(o) => {
        if (pending) return;
        setOpen(o);
        if (!o) reset();
      }}
    >
      <Dialog.Trigger asChild>
        <Button variant="outline" disabled={disabled}>
          {t('pages.ideaDetail.replace')}
        </Button>
      </Dialog.Trigger>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-50 bg-black/50" />
        <Dialog.Content className="fixed top-1/2 left-1/2 z-50 w-[calc(100%-2rem)] max-w-md -translate-x-1/2 -translate-y-1/2 rounded-lg border bg-background p-6 shadow-lg">
          <Dialog.Title className="text-lg font-semibold">{t('pages.ideaDetail.replace')}</Dialog.Title>
          <Dialog.Description className="mt-2 text-sm text-muted-foreground">
            {t('pages.ideaDetail.replaceBody', { name: idea.expectedFilenameBase })}
          </Dialog.Description>
          <form className="mt-4 flex flex-col gap-3" onSubmit={(e) => void submit(e)} noValidate>
            <label htmlFor="replace-file" className="text-sm font-medium">
              {t('fields.slides')}
            </label>
            <Input
              id="replace-file"
              type="file"
              accept=".pdf,.pptx,application/pdf,application/vnd.openxmlformats-officedocument.presentationml.presentation"
              aria-invalid={problemText ? true : undefined}
              aria-describedby={problemText ? 'replace-file-error' : undefined}
              onChange={(e) => {
                setFiles(Array.from(e.target.files ?? []));
                setProblem(null);
              }}
            />
            {problemText && (
              <p id="replace-file-error" role="alert" className="text-sm text-destructive">
                {problemText}
              </p>
            )}
            {failure && (
              <p role="alert" className="text-sm text-destructive">
                {failure}
              </p>
            )}
            {progress !== null && <progress className="h-2 w-full" max={100} value={progress} aria-label={t('pages.ideaCreate.uploading')} />}
            <div className="mt-2 flex justify-end gap-2">
              <Dialog.Close asChild>
                <Button type="button" variant="outline" disabled={pending}>
                  {t('common.cancel')}
                </Button>
              </Dialog.Close>
              <Button type="submit" disabled={pending} aria-busy={pending || undefined}>
                {pending ? t('pages.ideaDetail.replacing') : t('pages.ideaDetail.replaceSubmit')}
              </Button>
            </div>
          </form>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
