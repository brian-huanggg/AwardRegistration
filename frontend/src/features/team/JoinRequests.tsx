import { useState, type FormEvent } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { ApiError } from '@/shared/api';
import { Button } from '@/shared/ui/button';
import { Field } from '@/shared/ui/form-field';
import {
  createJoinRequest,
  fetchJoinRequests,
  joinRequestsKey,
  mapJoinError,
  precheckJoin,
  withdrawJoinRequest,
  type JoinProblems,
  type JoinRequest,
} from './requests';

/** `closed`: submissions are known not to be open; unknown leaves the controls on and the server decides. */
export function JoinRequestsPanel({ closed }: { closed: boolean }) {
  const { t, i18n } = useTranslation();
  const qc = useQueryClient();

  const [values, setValues] = useState({ leaderEmpNo: '', ideaName: '' });
  const [problems, setProblems] = useState<JoinProblems>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [sent, setSent] = useState(false);
  const [listError, setListError] = useState<string | null>(null);

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
    setFormError(null);
    const checked = precheckJoin(values);
    setProblems(checked.problems);
    if (Object.keys(checked.problems).length > 0) return;
    create.mutate(checked.values);
  };

  const fmt = new Intl.DateTimeFormat(i18n.language, { dateStyle: 'medium', timeStyle: 'short' });
  const set = (k: 'leaderEmpNo' | 'ideaName') => (ev: React.ChangeEvent<HTMLInputElement>) => setValues((v) => ({ ...v, [k]: ev.target.value }));

  return (
    <>
      <section className="mb-6 max-w-md rounded-xl border bg-card p-6">
        <h2 className="mb-2 text-lg font-semibold">{t('pages.joinRequests.apply')}</h2>
        {closed && <p className="mb-3 text-sm text-muted-foreground">{t('pages.joinRequests.closedHint')}</p>}
        <form onSubmit={submit} noValidate>
          <Field label={t('fields.leaderEmpNo')} error={problemText(problems.leaderEmpNo)}>
            {(p) => <input {...p} name="leaderEmpNo" inputMode="numeric" autoComplete="off" value={values.leaderEmpNo} onChange={set('leaderEmpNo')} />}
          </Field>
          <Field label={t('fields.ideaName')} error={problemText(problems.ideaName)}>
            {(p) => <input {...p} name="ideaName" autoComplete="off" value={values.ideaName} onChange={set('ideaName')} />}
          </Field>
          {formError && (
            <p role="alert" className="mb-3 text-sm text-destructive">
              {formError}
            </p>
          )}
          {sent && (
            <p role="status" className="mb-3 text-sm">
              {t('pages.joinRequests.sent')}
            </p>
          )}
          <Button type="submit" disabled={closed || create.isPending}>
            {create.isPending ? t('pages.joinRequests.sending') : t('pages.joinRequests.submit')}
          </Button>
        </form>
      </section>

      <section className="rounded-xl border bg-card p-6" aria-labelledby="my-requests">
        <h2 id="my-requests" className="mb-2 text-lg font-semibold">
          {t('pages.joinRequests.mine')}
        </h2>
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
        ) : list.data.length === 0 ? (
          <p className="text-sm">{t('pages.joinRequests.empty')}</p>
        ) : (
          <ul className="divide-y">
            {list.data.map((r) => (
              <li key={r.ideaId} className="flex flex-wrap items-center justify-between gap-3 py-3">
                <div className="text-sm">
                  <p className="font-medium">{r.ideaName}</p>
                  <p className="text-muted-foreground">
                    {t('pages.joinRequests.leader', { name: r.leader.name, empNo: r.leader.empNo })} · {t(`enums.category.${r.category}`, { defaultValue: r.category })} ·{' '}
                    {t(`enums.org.${r.org}`, { defaultValue: r.org })}
                  </p>
                  <p className="text-muted-foreground">{t('pages.joinRequests.requestedAt', { at: fmt.format(new Date(r.requestedAt)) })}</p>
                </div>
                <Button
                  variant="outline"
                  disabled={closed || (withdraw.isPending && withdraw.variables === r.ideaId)}
                  aria-label={t('pages.joinRequests.withdrawNamed', { name: r.ideaName })}
                  onClick={() => {
                    setListError(null);
                    withdraw.mutate(r.ideaId);
                  }}
                >
                  {t('pages.joinRequests.withdraw')}
                </Button>
              </li>
            ))}
          </ul>
        )}
      </section>
    </>
  );
}
