import { useQueryClient } from '@tanstack/react-query';
import { TriangleAlert } from 'lucide-react';
import { useEffect, useId, useMemo, useRef, useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { ApiError } from '@/shared/api';
import { makeServerClock } from '@/shared/time/offset';
import { Button } from '@/shared/ui/button';
import { ecoControl, ecoLabel } from '@/shared/ui/form-field';
import { cn } from '@/shared/ui/utils';
import { isoToLocalParts, localPartsToIso, offsetLabel, previewFor, zoneLabel, type LocalParts, type Preview } from './preview';
import { putDeadline } from './set';
import { StateBadge } from './StateBadge';
import { deadlineKey, useDeadline, type Deadline } from './useDeadline';

/** Debounce for the preview sentence, so the live region speaks once per edit and not per keystroke. */
const PREVIEW_DELAY_MS = 300;

/** Eco field on the warm form card: the inputs themselves are white, as in the frame. */
const control = cn(ecoControl, 'h-[52px] bg-background');

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
    <div className="flex min-w-0 flex-col gap-9">
      <CurrentDeadline data={data} loading={loading} error={isError} />

      <section aria-labelledby={`${id}-title`} className="rounded-[24px] border border-border bg-field p-5 text-card-foreground sm:p-7">
        <h2 id={`${id}-title`} className="text-xl font-bold tracking-[0.03em] text-brand-ink uppercase">
          {t('pages.adminDeadline.formTitle')}
        </h2>

        <form onSubmit={onSubmit} noValidate className="mt-5 flex flex-col gap-5">
          {problems.length > 0 && (
            <div
              ref={summary}
              key={failKey}
              tabIndex={-1}
              role="alert"
              aria-label={t('pages.adminDeadline.problems')}
              className="rounded-[14px] border-[1.5px] border-destructive bg-background px-[18px] py-3 text-sm text-destructive outline-none"
            >
              <ul>
                {problems.map((p) => (
                  <li key={p}>{p}</li>
                ))}
              </ul>
            </div>
          )}

          <fieldset className="min-w-0">
            <legend className="sr-only">{t('pages.adminDeadline.newDeadline')}</legend>
            <p id={hintId} className="text-sm text-muted-foreground">
              {t('pages.adminDeadline.hint')}
            </p>
            <div className="mt-4 grid gap-4 sm:grid-cols-3">
              <div className="min-w-0">
                <label htmlFor={`${id}-date`} className={ecoLabel}>
                  {t('pages.adminDeadline.date')}
                </label>
                <input {...fieldProps} id={`${id}-date`} name="date" type="date" value={value.date} onChange={(e) => change('date', e.target.value)} />
              </div>
              <div className="min-w-0">
                <label htmlFor={`${id}-time`} className={ecoLabel}>
                  {t('pages.adminDeadline.time')}
                </label>
                <input {...fieldProps} id={`${id}-time`} name="time" type="time" step={1} value={value.time} onChange={(e) => change('time', e.target.value)} />
              </div>
              <div className="min-w-0">
                {/* Read-only: the API stores an instant; choosing another zone is an open owner decision. */}
                <p id={`${id}-zone-label`} className={ecoLabel}>
                  {t('pages.adminDeadline.timeZone')}
                </p>
                <p aria-labelledby={`${id}-zone-label`} className="flex h-[52px] items-center truncate rounded-[14px] border-[1.5px] border-border bg-muted px-[18px] text-base text-foreground">
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

          <p aria-live="polite" className="flex flex-wrap items-center gap-2.5 text-sm text-muted-foreground">
            {preview && (
              <>
                <span>{t('pages.adminDeadline.preview.label')}</span>
                <PreviewPill preview={preview} />
              </>
            )}
          </p>

          <div data-prominent={past || undefined} className={cn('flex items-start gap-3 rounded-[14px] bg-warning/10 px-[18px] py-3.5 text-sm leading-5 text-foreground', past && 'font-medium ring-2 ring-warning')}>
            <TriangleAlert aria-hidden="true" className="mt-0.5 size-[18px] shrink-0 text-warning" />
            <p>{t('pages.adminDeadline.pastWarning')}</p>
          </div>

          <div className="flex flex-wrap items-center justify-end gap-3 sm:gap-4">
            <p role="status" className="mr-auto text-sm">
              {saved ? t('pages.adminDeadline.saved') : ''}
            </p>
            <Button
              type="button"
              variant="light"
              size="pill"
              onClick={() => {
                setEdited(null);
                setSaved(false);
              }}
            >
              {t('pages.adminDeadline.reset')}
            </Button>
            <Button type="submit" variant="lime" size="pill" className="px-8" disabled={pending}>
              {pending ? t('pages.adminDeadline.saving') : t('pages.adminDeadline.save')}
            </Button>
          </div>
        </form>
      </section>
    </div>
  );
}

/** The state the entered instant would produce, styled like the shell pill. */
function PreviewPill({ preview: p }: { preview: Preview }) {
  const { t } = useTranslation();
  const base = 'inline-flex items-center gap-2 rounded-full px-3.5 py-2 text-xs leading-tight font-medium tracking-[0.06em] uppercase';
  if (p.state === 'CLOSED')
    return (
      <span className={cn(base, 'border border-border bg-background text-destructive')}>
        <span aria-hidden="true" className="size-2 shrink-0 rounded-full bg-destructive" />
        {t('pages.adminDeadline.preview.CLOSED')}
      </span>
    );
  const left =
    p.days > 0
      ? t('pages.adminDeadline.preview.days', {
          days: p.days,
          hours: String(p.hours).padStart(2, '0'),
        })
      : t('pages.adminDeadline.preview.hours', {
          hours: p.hours,
          minutes: String(p.minutes).padStart(2, '0'),
        });
  return (
    <span className={cn(base, 'bg-accent-lime-tint text-brand-ink')}>
      <span aria-hidden="true" className="size-2 shrink-0 rounded-full bg-accent-lime" />
      {t('pages.adminDeadline.preview.OPEN', { left })}
    </span>
  );
}

/**
 * Current deadline (Pencil ra682 "Current"): state chip, the instant in the browser's zone, its offset
 * and the stored UTC value, and the time left by server time (a snapshot; it does not tick).
 * A "not set" variant when there is no deadline.
 */
function CurrentDeadline({ data, loading, error }: { data: Deadline | undefined; loading: boolean; error: boolean }) {
  const { t, i18n } = useTranslation();
  const id = useId();
  const left = useMemo(() => {
    if (!data?.deadline || data.state !== 'OPEN') return null;
    const p = previewFor(Date.parse(data.deadline), makeServerClock(data.serverTime).now());
    return p.state === 'OPEN' ? p : null;
  }, [data]);

  let body;
  if (loading) body = <p className="text-sm text-muted-foreground">{t('deadlineBanner.loading')}</p>;
  else if (error || !data) body = <p className="text-sm">{t('deadlineBanner.error')}</p>;
  else if (!data.deadline) body = <p className="text-3xl font-bold text-brand-ink uppercase sm:text-[40px] sm:leading-tight">{t('pages.adminDeadline.notSet')}</p>;
  else {
    const at = new Date(data.deadline);
    const offset = offsetLabel(at, i18n.language);
    body = (
      <>
        <p className="text-3xl font-bold break-words text-brand-ink uppercase sm:text-[40px] sm:leading-tight">
          {new Intl.DateTimeFormat(i18n.language, {
            weekday: 'short',
            year: 'numeric',
            month: 'short',
            day: 'numeric',
            hour: '2-digit',
            minute: '2-digit',
          }).format(at)}
        </p>
        <p className="font-mono text-[13px] break-all text-muted-foreground">
          {offset && <>{offset} · </>}
          <code className="font-mono">{data.deadline}</code> {t('pages.adminDeadline.utc')}
        </p>
        {left && (
          <div role="group" aria-label={t('pages.adminDeadline.countdown.label')}>
            <dl className="flex flex-wrap items-center gap-x-11 gap-y-3 pt-2.5">
              {(
                [
                  ['days', String(left.days)],
                  ['hours', String(left.hours).padStart(2, '0')],
                  ['minutes', String(left.minutes).padStart(2, '0')],
                ] as const
              ).map(([unit, n]) => (
                <div key={unit} className="flex items-center gap-2.5">
                  <dt className="order-last text-sm text-muted-foreground">{t(`pages.adminDeadline.countdown.${unit}`)}</dt>
                  <dd className="text-[44px] leading-none font-medium text-brand-purple">{n}</dd>
                </div>
              ))}
            </dl>
          </div>
        )}
      </>
    );
  }
  return (
    <section aria-labelledby={id} data-slot="current-deadline" className="flex flex-col gap-3.5">
      <div className="flex flex-wrap items-center gap-3">
        <h2 id={id} className="text-xs font-semibold tracking-[0.12em] text-muted-foreground uppercase">
          {t('pages.adminDeadline.current')}
        </h2>
        {data && !loading && !error && <StateBadge state={data.state} />}
      </div>
      {body}
    </section>
  );
}
