import { useId } from 'react';
import { useTranslation } from 'react-i18next';
import { Chip } from '@/shared/ui/chip';
import { CopyButton } from '@/shared/ui/copy-button';
import { cn } from '@/shared/ui/utils';

/** Team size including the Leader (PRD §6.7). The server's 409 `TEAM_FULL` stays the source of truth. */
export const TEAM_SIZE = 4;

interface Person {
  empNo: string;
  name: string;
}

/** Decorative initial; the name is always next to it as text. Purple 6.4:1, lavender 5.1:1 text contrast. */
function Avatar({ name, tone = 'muted' }: { name: string; tone?: 'muted' | 'primary' }) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        'flex size-10 shrink-0 items-center justify-center rounded-full text-[15px] font-semibold',
        tone === 'primary' ? 'bg-brand-purple text-white' : 'bg-brand-lavender text-brand-purple',
      )}
    >
      {Array.from(name.trim())[0] ?? '?'}
    </span>
  );
}

function CardShell({ labelledBy, tone = 'plain', children }: { labelledBy: string; tone?: 'plain' | 'warm'; children: React.ReactNode }) {
  return (
    <section
      aria-labelledby={labelledBy}
      className={cn('rounded-3xl p-5 text-card-foreground sm:p-6', tone === 'warm' ? 'bg-field' : 'border bg-card')}
    >
      {children}
    </section>
  );
}

const cardTitle = 'text-xl font-bold tracking-[0.02em] text-brand-ink uppercase';

/**
 * Team card (UI-04, Eco `uwwe8`): seat bar, then the Leader first, then the approved Members. `memberActions` renders the
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
    <CardShell labelledBy={titleId} tone="warm">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h2 id={titleId} className={cardTitle}>
          {t('pages.ideaDetail.team')}
        </h2>
        <p className="text-[13px] font-medium text-muted-foreground">{t('pages.ideaDetail.seats', { count: filled, max: TEAM_SIZE })}</p>
      </div>
      {/* decorative: the seat count text above carries the same information */}
      <div aria-hidden="true" data-testid="seat-bar" className="mt-4 grid grid-cols-4 gap-1.5">
        {Array.from({ length: TEAM_SIZE }, (_, i) => (
          <span key={i} className={cn('h-1.5 rounded-[3px]', i < filled ? 'bg-accent-lime' : 'bg-border')} />
        ))}
      </div>
      <ul aria-labelledby={titleId} className="mt-2">
        <li className="flex items-center gap-3 py-2.5">
          <Avatar name={leader.name} tone="primary" />
          <div className="min-w-0 flex-1">
            <p className="truncate text-[15px] font-semibold text-brand-ink">
              {leader.name}
              {you(leader.empNo)}
            </p>
            <p className="text-[13px] text-muted-foreground">{leader.empNo}</p>
          </div>
          <Chip tone="lime">{t('pages.ideaDetail.leaderBadge')}</Chip>
        </li>
        {members.map((m) => (
          <li key={m.empNo} className="flex items-center gap-3 py-2.5">
            <Avatar name={m.name} />
            <div className="min-w-0 flex-1">
              <p className="truncate text-[15px] font-semibold text-brand-ink">
                {m.name}
                {you(m.empNo)}
              </p>
              <p className="text-[13px] text-muted-foreground">{m.empNo}</p>
            </div>
            {memberActions?.(m)}
          </li>
        ))}
      </ul>
      {showOpenSeats && open > 0 && (
        <div className="mt-2 text-[13px] leading-[19px] text-muted-foreground">
          <p>{t('pages.ideaDetail.openSeats', { count: open, empNo: leader.empNo })}</p>
          <div className="mt-1 flex items-center gap-2">
            <span className="min-w-0 flex-1 font-medium break-words text-foreground">{ideaName}</span>
            <CopyButton
              value={ideaName}
              label={t('pages.ideaDetail.copyName')}
              copiedText={t('pages.ideaDetail.nameCopied')}
              failedText={t('pages.ideaDetail.nameCopyFailed')}
            />
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
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h2 id={titleId} className={cardTitle}>
          {t('pages.ideaDetail.pending')}
        </h2>
        <p className="text-[13px] font-medium text-muted-foreground">{t('pages.ideaDetail.pendingCount', { count: requests.length })}</p>
      </div>
      <p className="mt-2 text-[13px] text-muted-foreground">{hint}</p>
      {requests.length === 0 ? (
        <p className="mt-4 text-sm">{t('pages.ideaDetail.noPending')}</p>
      ) : (
        <ul aria-labelledby={titleId} className="mt-2 divide-y">
          {requests.map((r) => {
            const absolute = fmt.format(new Date(r.requestedAt));
            const when = now ? t('pages.ideaDetail.requestedAgo', { when: relative(r.requestedAt, now(), i18n.language) }) : absolute;
            return (
              <li key={r.empNo} className="flex flex-col gap-3.5 py-4">
                <div className="flex items-center gap-3">
                  <Avatar name={r.name} />
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-[15px] font-semibold text-brand-ink">{r.name}</p>
                    <p className="text-[13px] text-muted-foreground">
                      {r.empNo} ·{' '}
                      <time dateTime={r.requestedAt} title={absolute}>
                        {when}
                      </time>
                    </p>
                  </div>
                </div>
                <div className="flex flex-wrap gap-2.5 [&>button]:min-w-[7.5rem] [&>button]:flex-1">{actions?.(r)}</div>
              </li>
            );
          })}
        </ul>
      )}
      {footer && <p className="mt-2 text-[13px] text-muted-foreground">{footer}</p>}
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
