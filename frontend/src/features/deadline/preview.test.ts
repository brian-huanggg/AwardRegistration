import { isoToLocalParts, localPartsToIso, previewFor, zoneLabel } from './preview';

/** Restores TZ; assigning undefined would set the string "undefined". */
function restoreTz(tz: string | undefined) {
  if (tz === undefined) delete process.env.TZ;
  else process.env.TZ = tz;
}

describe('Date + Time combine into one instant', () => {
  const tz = process.env.TZ;
  afterEach(() => restoreTz(tz));

  it.each([
    ['UTC', '2026-11-30', '10:00', '2026-11-30T10:00:00+00:00', '2026-11-30T10:00:00.000Z'],
    ['UTC', '2026-01-01', '00:00:30', '2026-01-01T00:00:30+00:00', '2026-01-01T00:00:30.000Z'],
    ['Asia/Taipei', '2026-11-30', '18:00', '2026-11-30T18:00:00+08:00', '2026-11-30T10:00:00.000Z'],
    ['Asia/Taipei', '2026-12-01', '07:59:59', '2026-12-01T07:59:59+08:00', '2026-11-30T23:59:59.000Z'],
    ['Asia/Kolkata', '2026-11-30', '15:30', '2026-11-30T15:30:00+05:30', '2026-11-30T10:00:00.000Z'],
  ])('%s: %s %s -> %s', (zone, date, time, iso, utc) => {
    process.env.TZ = zone;
    const out = localPartsToIso({ date, time });
    expect(out).toBe(iso);
    expect(new Date(out!).toISOString()).toBe(utc);
    expect(isoToLocalParts(utc)).toEqual({ date, time: time.length === 5 ? `${time}:00` : time });
  });

  it.each([
    ['', '10:00'],
    ['2026-11-30', ''],
    ['2026-02-31', '10:00'],
    ['2026-11-30', '25:00'],
  ])('rejects %j %j', (date, time) => {
    expect(localPartsToIso({ date, time })).toBeNull();
  });

  it('splits a missing deadline into empty inputs', () => {
    expect(isoToLocalParts(null)).toEqual({ date: '', time: '' });
  });
});

describe('preview', () => {
  const now = Date.parse('2026-11-02T03:54:00Z');

  it('is OPEN with the time left for a future instant', () => {
    expect(previewFor(Date.parse('2026-11-30T10:00:00Z'), now)).toEqual({ state: 'OPEN', days: 28, hours: 6, minutes: 6 });
  });

  it('is CLOSED for a past instant and at the deadline itself (no grace period)', () => {
    expect(previewFor(now - 1000, now)).toEqual({ state: 'CLOSED' });
    expect(previewFor(now, now)).toEqual({ state: 'CLOSED' });
  });

  it('labels the zone with its offset', () => {
    const tz = process.env.TZ;
    process.env.TZ = 'Asia/Taipei';
    try {
      expect(zoneLabel(new Date('2026-11-30T10:00:00Z'), 'en')).toBe('Asia/Taipei (GMT+8)');
      expect(zoneLabel(new Date(Number.NaN), 'en')).toBe('Asia/Taipei');
    } finally {
      restoreTz(tz);
    }
  });
});
