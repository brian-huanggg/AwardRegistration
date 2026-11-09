import { useQueryClient } from '@tanstack/react-query';
import { useEffect, useRef, useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { ApiError } from '@/shared/api';
import { isoToLocalInput, localInputToIso } from '@/shared/time/local';
import { Button } from '@/shared/ui/button';
import { Field } from '@/shared/ui/form-field';
import { putDeadline } from './set';
import { deadlineKey, useDeadline } from './useDeadline';

/** Admin form that sets the submission deadline (Frontend.md §5.3). The server stays the judge of range and format. */
export function DeadlineForm() {
  const { t, i18n } = useTranslation();
  const client = useQueryClient();
  const { data } = useDeadline();
  // null = untouched: the field shows the current deadline until the Admin types.
  const [edited, setEdited] = useState<string | null>(null);
  const [pending, setPending] = useState(false);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);
  const summary = useRef<HTMLDivElement>(null);
  const [failKey, setFailKey] = useState(0);

  const value = edited ?? (data?.deadline ? isoToLocalInput(data.deadline) : '');
  useEffect(() => {
    if (failKey > 0) summary.current?.focus();
  }, [failKey]);

  const text = (code: string) => (i18n.exists(`errors.${code}`) ? t(`errors.${code}`) : t('errors.UNKNOWN'));
  const fail = (field: string | null, rest: string | null) => {
    setFieldError(field);
    setFormError(rest);
    setFailKey((k) => k + 1);
  };

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (pending) return;
    setSaved(false);
    const iso = localInputToIso(value);
    if (iso === null) return fail(text('INVALID_FORMAT'), null);
    setPending(true);
    setFieldError(null);
    setFormError(null);
    try {
      client.setQueryData(deadlineKey, await putDeadline(iso));
      setEdited(null);
      setSaved(true);
    } catch (err) {
      if (!(err instanceof ApiError)) return fail(null, t('errors.UNKNOWN'));
      const id = err.requestId ? ` ${t('common.requestId', { id: err.requestId })}` : '';
      const onField = err.errors.find((x) => x.field === 'deadline');
      if (err.code === 'VALIDATION_FAILED' && onField) fail(text(onField.code), null);
      else fail(null, text(err.code) + id);
    } finally {
      setPending(false);
    }
  };

  const problems = [fieldError, formError].filter((x): x is string => x !== null);
  return (
    <form onSubmit={onSubmit} noValidate className="max-w-md rounded-xl border bg-card p-6">
      {problems.length > 0 && (
        <div ref={summary} key={failKey} tabIndex={-1} role="alert" aria-label={t('pages.adminDeadline.problems')} className="mb-4 rounded-md border border-destructive p-3 text-sm text-destructive outline-none">
          <ul>
            {problems.map((p) => (
              <li key={p}>{p}</li>
            ))}
          </ul>
        </div>
      )}
      <Field label={t('pages.adminDeadline.label')} hint={t('pages.adminDeadline.hint')} error={fieldError ?? undefined}>
        {(p) => (
          <input
            {...p}
            name="deadline"
            type="datetime-local"
            step={1}
            required
            value={value}
            onChange={(e) => {
                            setSaved(false);
              setEdited(e.target.value);
            }}
          />
        )}
      </Field>
      <Button type="submit" disabled={pending}>
        {pending ? t('pages.adminDeadline.saving') : t('common.save')}
      </Button>
      <p role="status" className="mt-3 text-sm">
        {saved ? t('pages.adminDeadline.saved') : ''}
      </p>
    </form>
  );
}
