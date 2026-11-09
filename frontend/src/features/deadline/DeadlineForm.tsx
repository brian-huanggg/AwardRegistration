import { useQueryClient } from '@tanstack/react-query';
import { Calendar, Clock4, Eye, Globe, Save, TriangleAlert } from 'lucide-react';
import { useEffect, useId, useMemo, useRef, useState, type FormEvent } from 'react';
import type { TFunction } from 'i18next';
import { useTranslation } from 'react-i18next';
import { ApiError } from '@/shared/api';
import { makeServerClock } from '@/shared/time/offset';
import { Button } from '@/shared/ui/button';
import { cn } from '@/shared/ui/utils';
import { isoToLocalParts, localPartsToIso, previewFor, zoneLabel, type LocalParts, type Preview } from './preview';
import { putDeadline } from './set';
import { StateBadge } from './StateBadge';
import { deadlineKey, useDeadline, type Deadline } from './useDeadline';

/** Debounce for the preview sentence, so the live region speaks once per edit and not per keystroke. */
const PREVIEW_DELAY_MS = 300;

const control = 'h-9 w-full rounded-md border bg-background px-3 text-sm aria-invalid:border-destructive';

/**
 * Admin deadline card (Frontend.md §5.3, UI-07): the current deadline, a Date + Time pair in the
 * browser time zone, a preview of the resulting state, and Save. The deadline can be moved but never
 * cleared, so there is no clear action. The server stays the judge of range and format.
 */
export function DeadlineForm() {
  const { t, i18n } = useTranslation();
  const client = useQueryClient();
  const { data, isPending: loading, isError } = useDeadline();
  // null = untouched: the inputs show the current deadline until the Admin edits them.
  const [edited, setEdited] = useState<LocalParts | null>(null);
  const [pending, setPending] = useState(false);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);
  const summary = useRef<HTMLDivElement>(null);
  const [failKey, setFailKey] = useState(0);
  const id = useId();

  const current = useMemo(() => isoToLocalParts(data?.deadline), [data?.deadline]);
  const value = edited ?? current;
  const iso = localPartsToIso(value);

  useEffect(() => {
    if (failKey > 0) summary.current?.focus();
  }, [failKey]);

  // Server-time offset (§5.3): the preview never judges by the client clock alone.
  const clock = useMemo(() => (data ? makeServerClock(data.serverTime) : null), [data]);
  const [preview, setPreview] = useState<Preview | null>(null);
  useEffect(() => {
    const timer = setTimeout(() => setPreview(iso && clock ? previewFor(Date.parse(iso), clock.now()) : null), PREVIEW_DELAY_MS);
    return () => clearTimeout(timer);
  }, [iso, clock]);

  const text = (code: string) => (i18n.exists(`errors.${code}`) ? t(`errors.${code}`) : t('errors.UNKNOWN'));
  const fail = (field: string | null, rest: string | null) => {
    setFieldError(field);
    setFormError(rest);
    setFailKey((k) => k + 1);
  };

  const change = (part: keyof LocalParts, v: string) => {
    setSaved(false);
    setEdited({ ...value, [part]: v });
  };

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (pending) return;
    setSaved(false);
    if (iso === null) return fail(text('INVALID_FORMAT'), null);
    setPending(true);
    setFieldError(null);
    setFormError(null);
    try {
      // The answer has the GET /deadline body, so the current block and the shell pill refresh without another GET.
      client.setQueryData(deadlineKey, await putDeadline(iso));
      setEdited(null);
      setSaved(true);
    } catch (err) {
      if (!(err instanceof ApiError)) return fail(null, t('errors.UNKNOWN'));
      const rid = err.requestId ? ` ${t('common.requestId', { id: err.requestId })}` : '';
      const onField = err.errors.find((x) => x.field === 'deadline');
      if (err.code === 'VALIDATION_FAILED' && onField) fail(text(onField.code), null);
      else fail(null, text(err.code) + rid);
    } finally {
      setPending(false);
    }
  };

  const problems = [fieldError, formError].filter((x): x is string => x !== null);
  const past = preview?.state === 'CLOSED';
  const errorId = `${id}-error`;
  const hintId = `${id}-hint`;
  const fieldProps = {
    className: control,
    required: true,
    'aria-invalid': fieldError ? true : undefined,
    'aria-describedby': [hintId, fieldError && errorId].filter(Boolean).join(' '),
  } as const;
  const zoneAt = new Date(iso ?? data?.deadline ?? data?.serverTime ?? Number.NaN);

  return (
    <section aria-labelledby={`${id}-title`} className="rounded-xl border bg-card p-6 text-card-foreground shadow-sm">
      <h2 id={`${id}-title`} className="sr-only">
        {t('pages.adminDeadline.formTitle')}
      </h2>
      <CurrentDeadline data={data} loading={loading} error={isError} />

      <form onSubmit={onSubmit} noValidate className="mt-6 space-y-5 border-t pt-6">
        {problems.length > 0 && (
          <div ref={summary} key={failKey} tabIndex={-1} role="alert" aria-label={t('pages.adminDeadline.problems')} className="rounded-md border border-destructive p-3 text-sm text-destructive outline-none">
            <ul>
              {problems.map((p) => (
                <li key={p}>{p}</li>
              ))}
            </ul>
          </div>
        )}

        <fieldset>
          <legend className="text-sm font-semibold">{t('pages.adminDeadline.newDeadline')}</legend>
          <p id={hintId} className="mt-1 text-sm text-muted-foreground">
            {t('pages.adminDeadline.hint')}
          </p>
          <div className="mt-3 grid gap-4 sm:grid-cols-3">
            <div>
              <label htmlFor={`${id}-date`} className="mb-1 flex items-center gap-1.5 text-sm font-medium">
                <Calendar aria-hidden="true" className="size-4 text-muted-foreground" />
                {t('pages.adminDeadline.date')}
              </label>
              <input {...fieldProps} id={`${id}-date`} name="date" type="date" value={value.date} onChange={(e) => change('date', e.target.value)} />
            </div>
            <div>
              <label htmlFor={`${id}-time`} className="mb-1 flex items-center gap-1.5 text-sm font-medium">
                <Clock4 aria-hidden="true" className="size-4 text-muted-foreground" />
                {t('pages.adminDeadline.time')}
              </label>
              <input {...fieldProps} id={`${id}-time`} name="time" type="time" step={1} value={value.time} onChange={(e) => change('time', e.target.value)} />
            </div>
            <div>
              {/* Read-only: the API stores an instant; choosing another zone is an open owner decision. */}
              <p id={`${id}-zone-label`} className="mb-1 flex items-center gap-1.5 text-sm font-medium">
                <Globe aria-hidden="true" className="size-4 text-muted-foreground" />
                {t('pages.adminDeadline.timeZone')}
              </p>
              <p aria-labelledby={`${id}-zone-label`} className="flex h-9 items-center truncate rounded-md border bg-muted px-3 text-sm">
                {zoneLabel(zoneAt, i18n.language)}
              </p>
            </div>
          </div>
          {fieldError && (
            <p id={errorId} role="alert" className="mt-2 text-sm text-destructive">
              {fieldError}
            </p>
          )}
        </fieldset>

        <div className="flex items-start gap-2 rounded-md bg-muted px-3 py-2 text-sm">
          <Eye aria-hidden="true" className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
          <p aria-live="polite">{preview && previewText(preview, t)}</p>
        </div>

        <div data-prominent={past || undefined} className={cn('flex items-start gap-2 rounded-md border p-3 text-sm', past ? 'border-warning bg-warning/10 font-medium text-warning' : 'text-muted-foreground')}>
          <TriangleAlert aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
          <p>{t('pages.adminDeadline.pastWarning')}</p>
        </div>

        <div className="flex flex-wrap items-center justify-end gap-3">
          <p role="status" className="mr-auto text-sm">
            {saved ? t('pages.adminDeadline.saved') : ''}
          </p>
          <Button
            type="button"
            variant="outline"
            onClick={() => {
              setEdited(null);
              setSaved(false);
            }}
          >
            {t('pages.adminDeadline.reset')}
          </Button>
          <Button type="submit" disabled={pending}>
            <Save aria-hidden="true" />
            {pending ? t('pages.adminDeadline.saving') : t('pages.adminDeadline.save')}
          </Button>
        </div>
      </form>
    </section>
  );
}

function previewText(p: Preview, t: TFunction): string {
  if (p.state === 'CLOSED') return t('pages.adminDeadline.preview.CLOSED');
  const left =
    p.days > 0
      ? t('pages.adminDeadline.preview.days', { days: p.days, hours: String(p.hours).padStart(2, '0') })
      : t('pages.adminDeadline.preview.hours', { hours: p.hours, minutes: String(p.minutes).padStart(2, '0') });
  return t('pages.adminDeadline.preview.OPEN', { left });
}

/** Current deadline in the browser's zone, the stored UTC value, and the state badge; a "not set" variant. */
function CurrentDeadline({ data, loading, error }: { data: Deadline | undefined; loading: boolean; error: boolean }) {
  const { t, i18n } = useTranslation();
  let body;
  if (loading) body = <p className="text-sm text-muted-foreground">{t('deadlineBanner.loading')}</p>;
  else if (error || !data) body = <p className="text-sm">{t('deadlineBanner.error')}</p>;
  else
    body = (
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          {data.deadline ? (
            <>
              <p className="text-lg font-semibold">
                {new Intl.DateTimeFormat(i18n.language, {
                  weekday: 'short',
                  year: 'numeric',
                  month: 'short',
                  day: 'numeric',
                  hour: '2-digit',
                  minute: '2-digit',
                  timeZoneName: 'shortOffset',
                }).format(new Date(data.deadline))}
              </p>
              <p className="mt-1 text-sm break-all text-muted-foreground">
                <code className="font-mono">{data.deadline}</code> {t('pages.adminDeadline.utc')}
              </p>
            </>
          ) : (
            <p className="text-lg font-semibold">{t('pages.adminDeadline.notSet')}</p>
          )}
        </div>
        <StateBadge state={data.state} />
      </div>
    );
  return (
    <div>
      <h3 className="mb-2 text-sm font-medium text-muted-foreground">{t('pages.adminDeadline.current')}</h3>
      {body}
    </div>
  );
}
