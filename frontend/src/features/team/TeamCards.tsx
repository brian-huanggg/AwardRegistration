import { Plus } from 'lucide-react';
import { useId } from 'react';
import { useTranslation } from 'react-i18next';
import { CopyButton } from '@/shared/ui/copy-button';
import { cn } from '@/shared/ui/utils';

/** Team size including the Leader (PRD §6.7). The server's 409 `TEAM_FULL` stays the source of truth. */
export const TEAM_SIZE = 4;

interface Person {
  empNo: string;
  name: string;
}

/** Decorative initial; the name is always next to it as text. */
function Avatar({ name, tone = 'muted' }: { name: string; tone?: 'muted' | 'primary' }) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        'flex size-9 shrink-0 items-center justify-center rounded-full text-sm font-semibold',
        tone === 'primary' ? 'bg-primary text-primary-foreground' : 'bg-muted text-foreground',
      )}
    >
      {Array.from(name.trim())[0] ?? '?'}
    </span>
  );
}

function CardShell({ labelledBy, children }: { labelledBy: string; children: React.ReactNode }) {
  return (
    <section aria-labelledby={labelledBy} className="rounded-xl border bg-card text-card-foreground shadow-sm">
      {children}
    </section>
  );
}

/**
 * Team card (UI-04): the Leader first, then the approved Members. `memberActions` renders the
 * per-row control (remove) for a Member; the Leader row carries a badge instead.
 */
export function TeamCard<M extends Person>({
  leader,
  members,
  meEmpNo,
  ideaName,
  showOpenSeats,
  memberActions,
}: {
  leader: Person;
  members: readonly M[];
  meEmpNo: string;
  ideaName: string;
  /** Leader only: the hint that tells applicants what to enter (PRD §6.7). */
  showOpenSeats: boolean;
  memberActions?: (member: M) => React.ReactNode;
}) {
  const { t } = useTranslation();
  const titleId = useId();
  const filled = 1 + members.length;
  const open = Math.max(0, TEAM_SIZE - filled);
  const you = (empNo: string) =>
    empNo === meEmpNo ? <span className="font-normal text-muted-foreground"> {t('pages.ideaDetail.you')}</span> : null;

  return (
    <CardShell labelledBy={titleId}>
      <div className="flex flex-wrap items-baseline justify-between gap-2 border-b px-5 py-4">
        <h2 id={titleId} className="text-base font-semibold">
          {t('pages.ideaDetail.team')}
        </h2>
        <p className="text-sm text-muted-foreground">{t('pages.ideaDetail.seats', { count: filled, max: TEAM_SIZE })}</p>
      </div>
      <ul aria-labelledby={titleId} className="divide-y">
        <li className="flex items-center gap-3 px-5 py-3">
          <Avatar name={leader.name} tone="primary" />
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-medium">
              {leader.name}
              {you(leader.empNo)}
            </p>
            <p className="text-xs text-muted-foreground">{leader.empNo}</p>
          </div>
          <span className="rounded-full bg-primary px-2.5 py-0.5 text-xs font-semibold text-primary-foreground">
            {t('pages.ideaDetail.leaderBadge')}
          </span>
        </li>
        {members.map((m) => (
          <li key={m.empNo} className="flex items-center gap-3 px-5 py-3">
            <Avatar name={m.name} />
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium">
                {m.name}
                {you(m.empNo)}
              </p>
              <p className="text-xs text-muted-foreground">{m.empNo}</p>
            </div>
            {memberActions?.(m)}
          </li>
        ))}
      </ul>
      {showOpenSeats && open > 0 && (
        <div className="flex items-start gap-2 border-t px-5 py-3 text-sm text-muted-foreground">
          <Plus aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
          <div className="min-w-0 flex-1">
            <p>{t('pages.ideaDetail.openSeats', { count: open, empNo: leader.empNo })}</p>
            <div className="mt-1 flex items-center gap-2">
              <span className="min-w-0 flex-1 break-words font-medium text-foreground">{ideaName}</span>
              <CopyButton
                value={ideaName}
                label={t('pages.ideaDetail.copyName')}
                copiedText={t('pages.ideaDetail.nameCopied')}
                failedText={t('pages.ideaDetail.nameCopyFailed')}
              />
            </div>
          </div>
        </div>
      )}
    </CardShell>
  );
}

/**
 * Join requests card (UI-04), rendered only when the detail carries `pendingRequests` (Leader or
 * Admin). `now` is the server-corrected clock (Frontend.md §5.3); without it only the absolute time shows.
 */
export function JoinRequestsCard<R extends Person & { requestedAt: string }>({
  requests,
  hint,
  footer,
  now,
  actions,
}: {
  requests: readonly R[];
  hint: string;
  footer?: string;
  now: (() => number) | null;
  actions?: (request: R) => React.ReactNode;
}) {
  const { t, i18n } = useTranslation();
  const titleId = useId();
  const fmt = new Intl.DateTimeFormat(i18n.language, { dateStyle: 'medium', timeStyle: 'short' });

  return (
    <CardShell labelledBy={titleId}>
      <div className="border-b px-5 py-4">
        <h2 id={titleId} className="flex items-center gap-2 text-base font-semibold">
          {t('pages.ideaDetail.pending')}
          <span className="rounded-full bg-muted px-2 py-0.5 text-xs font-semibold">{requests.length}</span>
        </h2>
        <p className="mt-1 text-sm text-muted-foreground">{hint}</p>
      </div>
      {requests.length === 0 ? (
        <p className="px-5 py-4 text-sm">{t('pages.ideaDetail.noPending')}</p>
      ) : (
        <ul aria-labelledby={titleId} className="divide-y">
          {requests.map((r) => {
            const absolute = fmt.format(new Date(r.requestedAt));
            const when = now ? t('pages.ideaDetail.requestedAgo', { when: relative(r.requestedAt, now(), i18n.language) }) : absolute;
            return (
              <li key={r.empNo} className="flex flex-wrap items-center gap-x-3 gap-y-2 px-5 py-3">
                <Avatar name={r.name} />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium">{r.name}</p>
                  <p className="text-xs text-muted-foreground">
                    {r.empNo} ·{' '}
                    <time dateTime={r.requestedAt} title={absolute}>
                      {when}
                    </time>
                  </p>
                </div>
                <div className="flex flex-wrap gap-2">{actions?.(r)}</div>
              </li>
            );
          })}
        </ul>
      )}
      {footer && <p className="border-t px-5 py-3 text-xs text-muted-foreground">{footer}</p>}
    </CardShell>
  );
}

const UNITS: [Intl.RelativeTimeFormatUnit, number][] = [
  ['year', 365 * 86400],
  ['month', 30 * 86400],
  ['day', 86400],
  ['hour', 3600],
  ['minute', 60],
];

/** "2 hours ago"; a small server/client skew never yields a future time. */
export function relative(iso: string, now: number, lang: string): string {
  const seconds = Math.min(0, (Date.parse(iso) - now) / 1000);
  const rtf = new Intl.RelativeTimeFormat(lang, { numeric: 'auto' });
  for (const [unit, size] of UNITS) if (Math.abs(seconds) >= size) return rtf.format(Math.round(seconds / size), unit);
  return rtf.format(Math.round(seconds), 'second');
}
