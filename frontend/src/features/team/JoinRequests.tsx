import { useState, type FormEvent } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Trans, useTranslation } from 'react-i18next';
import { Hourglass, Lock, Send, Undo2 } from 'lucide-react';
import { ApiError } from '@/shared/api';
import { Button } from '@/shared/ui/button';
import { Field } from '@/shared/ui/form-field';
import {
  createJoinRequest,
  fetchJoinRequests,
  isJoinNotFound,
  joinRequestsKey,
  mapJoinError,
  precheckJoin,
  sentAtParts,
  withdrawJoinRequest,
  type JoinProblems,
  type JoinRequest,
} from './requests';

const card = 'rounded-xl border bg-card p-6 text-card-foreground shadow-sm';
const STEPS = ['apply', 'review', 'joined'] as const;

/**
 * `closed`: submissions are known not to be open; unknown leaves the controls on and the server decides.
 * `serverTime`: the server clock from `GET /deadline`, used to tell "today" and "yesterday" apart; the browser clock is the fallback.
 */
export function JoinRequestsPanel({ closed, serverTime }: { closed: boolean; serverTime?: string }) {
  const { t, i18n } = useTranslation();
  const qc = useQueryClient();

  const [values, setValues] = useState({ leaderEmpNo: '', ideaName: '' });
  const [problems, setProblems] = useState<JoinProblems>({});
  const [notFound, setNotFound] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [sent, setSent] = useState(false);
  const [listError, setListError] = useState<string | null>(null);
  const [clientNow] = useState(() => Date.now());
  const now = serverTime ? Date.parse(serverTime) : clientNow;

  const message = (e: ApiError) => {
    const code = i18n.exists(`errors.${e.code}`) ? e.code : 'UNKNOWN';
    return t(`errors.${code}`) + (e.requestId ? ` ${t('common.requestId', { id: e.requestId })}` : '');
  };
  const problemText = (code: string | undefined) => (code === undefined ? undefined : t(i18n.exists(`errors.${code}`) ? `errors.${code}` : 'errors.UNKNOWN'));

  const list = useQuery({ queryKey: joinRequestsKey, queryFn: ({ signal }) => fetchJoinRequests(signal), retry: false });

  const create = useMutation({
    mutationFn: createJoinRequest,
    onSuccess: (created) => {
      qc.setQueryData<JoinRequest[]>(joinRequestsKey, (old) => [created, ...(old ?? []).filter((r) => r.ideaId !== created.ideaId)]);
      void qc.invalidateQueries({ queryKey: joinRequestsKey });
      setValues({ leaderEmpNo: '', ideaName: '' });
      setSent(true);
    },
    onError: (e) => {
      if (isJoinNotFound(e)) {
        setNotFound(true);
        return;
      }
      const mapped = mapJoinError(e);
      setProblems(mapped.problems);
      setFormError(mapped.rest ? message(mapped.rest) : null);
    },
  });

  const withdraw = useMutation({
    mutationFn: withdrawJoinRequest,
    onSuccess: (_d, ideaId) => qc.setQueryData<JoinRequest[]>(joinRequestsKey, (old) => (old ?? []).filter((r) => r.ideaId !== ideaId)),
    onError: (e) => {
      // 404: the request is already gone (handled or the Idea deleted); a fresh list shows the truth.
      if (e instanceof ApiError && e.status === 404) void qc.invalidateQueries({ queryKey: joinRequestsKey });
      else setListError(message(e instanceof ApiError ? e : new ApiError({ status: 0, code: 'UNKNOWN' })));
    },
  });

  const submit = (ev: FormEvent) => {
    ev.preventDefault();
    if (create.isPending) return;
    setSent(false);
    setNotFound(false);
    setFormError(null);
    const checked = precheckJoin(values);
    setProblems(checked.problems);
    if (Object.keys(checked.problems).length > 0) return;
    create.mutate(checked.values);
  };

  const sentAt = (iso: string) => {
    const p = sentAtParts(iso, now, i18n.language);
    return p.kind === 'date' ? p.date : t(`pages.joinRequests.${p.kind}`, { time: p.time });
  };
  const set = (k: 'leaderEmpNo' | 'ideaName') => (ev: React.ChangeEvent<HTMLInputElement>) => setValues((v) => ({ ...v, [k]: ev.target.value }));
  const items = list.data ?? [];

  return (
    <>
      <div className="grid items-start gap-6 lg:grid-cols-12">
        <section className={`${card} lg:col-span-5`} aria-labelledby="apply-title">
          <h2 id="apply-title" className="text-base font-semibold">
            {t('pages.joinRequests.apply')}
          </h2>
          <p className="mt-1 mb-4 text-sm text-muted-foreground">{t('pages.joinRequests.applyHint')}</p>
          {closed && <p className="mb-4 text-sm text-muted-foreground">{t('pages.joinRequests.closedHint')}</p>}
          <form onSubmit={submit} noValidate>
            <Field label={t('fields.leaderEmpNo')} hint={t('pages.joinRequests.leaderEmpNoHint')} error={problemText(problems.leaderEmpNo)}>
              {(p) => <input {...p} name="leaderEmpNo" inputMode="numeric" autoComplete="off" value={values.leaderEmpNo} onChange={set('leaderEmpNo')} />}
            </Field>
            <Field label={t('fields.ideaName')} error={notFound ? t('pages.joinRequests.notFound') : problemText(problems.ideaName)}>
              {(p) => <input {...p} name="ideaName" autoComplete="off" value={values.ideaName} onChange={set('ideaName')} />}
            </Field>
            {formError && (
              <p role="alert" className="mb-4 text-sm text-destructive">
                {formError}
              </p>
            )}
            {sent && (
              <p role="status" className="mb-4 text-sm text-success">
                {t('pages.joinRequests.sent')}
              </p>
            )}
            <Button type="submit" className="w-full" disabled={closed || create.isPending}>
              <Send aria-hidden="true" />
              {create.isPending ? t('pages.joinRequests.sending') : t('pages.joinRequests.submit')}
            </Button>
          </form>
        </section>

        <section className={`${card} lg:col-span-7`} aria-labelledby="my-requests">
          <div className="mb-4 flex flex-wrap items-start justify-between gap-3">
            <div>
              <h2 id="my-requests" className="text-base font-semibold">
                {t('pages.joinRequests.mine')}
              </h2>
              <p className="mt-1 text-sm text-muted-foreground">{t('pages.joinRequests.mineHint')}</p>
            </div>
            {items.length > 0 && (
              <span data-testid="pending-count" className="rounded-full border px-2.5 py-0.5 text-xs font-semibold">
                {t('pages.joinRequests.pendingCount', { count: items.length })}
              </span>
            )}
          </div>
          {listError && (
            <p role="alert" className="mb-3 text-sm text-destructive">
              {listError}
            </p>
          )}
          {list.isPending ? (
            <p role="status" className="text-sm">
              {t('common.loading')}
            </p>
          ) : list.isError ? (
            <div role="alert">
              <p className="mb-2 text-sm">{message(list.error instanceof ApiError ? list.error : new ApiError({ status: 0, code: 'UNKNOWN' }))}</p>
              <Button variant="outline" onClick={() => void list.refetch()}>
                {t('common.retry')}
              </Button>
            </div>
          ) : items.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t('pages.joinRequests.empty')}</p>
          ) : (
            <ul className="divide-y rounded-lg border">
              {items.map((r) => (
                <li key={r.ideaId} className="flex flex-wrap items-center gap-3 p-3">
                  <span aria-hidden="true" className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                    <Hourglass className="size-4" />
                  </span>
                  <div className="min-w-0 flex-1 text-sm">
                    <p className="font-semibold break-words">{r.ideaName}</p>
                    <p className="text-xs text-muted-foreground">
                      {t('pages.joinRequests.leader', { name: r.leader.name, empNo: r.leader.empNo })} ·{' '}
                      <Trans i18nKey="pages.joinRequests.sentAt" values={{ at: sentAt(r.requestedAt) }} components={{ time: <time dateTime={r.requestedAt} /> }} />
                    </p>
                  </div>
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={closed || (withdraw.isPending && withdraw.variables === r.ideaId)}
                    aria-label={t('pages.joinRequests.withdrawNamed', { name: r.ideaName })}
                    onClick={() => {
                      setListError(null);
                      withdraw.mutate(r.ideaId);
                    }}
                  >
                    <Undo2 aria-hidden="true" />
                    {t('pages.joinRequests.withdraw')}
                  </Button>
                </li>
              ))}
            </ul>
          )}
          <p className="mt-4 flex items-start gap-2 text-xs text-muted-foreground">
            <Lock aria-hidden="true" className="mt-0.5 size-3.5 shrink-0" />
            {t('pages.joinRequests.lockedHint')}
          </p>
        </section>
      </div>

      <section aria-labelledby="how-it-works" className="rounded-xl bg-muted p-6">
        <h2 id="how-it-works" className="sr-only">
          {t('pages.joinRequests.stepsTitle')}
        </h2>
        <ol className="grid gap-6 lg:grid-cols-3">
          {STEPS.map((s, i) => (
            <li key={s} className="flex items-start gap-3">
              <span aria-hidden="true" className="flex size-7 shrink-0 items-center justify-center rounded-full bg-primary text-sm font-bold text-primary-foreground">
                {i + 1}
              </span>
              <div className="text-sm">
                <p className="font-semibold">{t(`pages.joinRequests.steps.${s}.title`)}</p>
                <p className="text-muted-foreground">{t(`pages.joinRequests.steps.${s}.body`)}</p>
              </div>
            </li>
          ))}
        </ol>
      </section>
    </>
  );
}
