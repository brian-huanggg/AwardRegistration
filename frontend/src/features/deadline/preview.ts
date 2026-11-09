import { isoToLocalInput, localInputToIso } from '@/shared/time/local';

/** Date (`YYYY-MM-DD`) and time (`HH:mm[:ss]`) input values, both in the browser time zone. */
export interface LocalParts {
  date: string;
  time: string;
}

/** Splits an instant into the values of the native date and time inputs (browser time zone, seconds kept). */
export function isoToLocalParts(iso: string | null | undefined): LocalParts {
  const local = iso ? isoToLocalInput(iso) : '';
  const [date = '', time = ''] = local.split('T');
  return { date, time };
}

/** Combines the date and time inputs into ISO 8601 with offset through `shared/time/local.ts`; `null` if incomplete or invalid. */
export function localPartsToIso({ date, time }: LocalParts): string | null {
  if (!date || !time) return null;
  return localInputToIso(`${date}T${time}`);
}

export type Preview =
  | { state: 'OPEN'; days: number; hours: number; minutes: number }
  | { state: 'CLOSED' };

/**
 * The submission state a deadline at `instantMs` would produce at server time `nowMs`, plus the time
 * left. Mirrors the server rule (PRD SubmissionGate): open while now is before the deadline, no grace
 * period. Only a preview: the server stays the judge.
 */
export function previewFor(instantMs: number, nowMs: number): Preview {
  const left = instantMs - nowMs;
  if (left <= 0) return { state: 'CLOSED' };
  const totalMinutes = Math.floor(left / 60_000);
  return {
    state: 'OPEN',
    days: Math.floor(totalMinutes / 1440),
    hours: Math.floor((totalMinutes % 1440) / 60),
    minutes: totalMinutes % 60,
  };
}

/** The zone label shown read-only beside the inputs, e.g. "Asia/Taipei (GMT+8)", for the offset at `at`. */
export function zoneLabel(at: Date, locale: string): string {
  const zone = Intl.DateTimeFormat().resolvedOptions().timeZone;
  const offset = Number.isNaN(at.getTime())
    ? undefined
    : new Intl.DateTimeFormat(locale, { timeZoneName: 'shortOffset' }).formatToParts(at).find((p) => p.type === 'timeZoneName')?.value;
  return offset ? `${zone} (${offset})` : zone;
}
