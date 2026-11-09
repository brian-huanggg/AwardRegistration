import { isoToLocalInput, localInputToIso } from './local';

describe('local time conversion', () => {
  it('adds the browser offset and keeps seconds', () => {
    expect(localInputToIso('2026-12-01T10:30')).toMatch(/^2026-12-01T10:30:00[+-]\d{2}:\d{2}$/);
    expect(localInputToIso('2026-12-01T10:30:15')).toMatch(/^2026-12-01T10:30:15[+-]\d{2}:\d{2}$/);
  });
  it('rejects garbage and roll-over dates', () => {
    expect(localInputToIso('')).toBeNull();
    expect(localInputToIso('2026-02-31T10:00')).toBeNull();
    expect(localInputToIso('tomorrow')).toBeNull();
  });
  it('round-trips an instant through the local input value', () => {
    const iso = localInputToIso('2026-12-01T10:30:15')!;
    expect(isoToLocalInput(iso)).toBe('2026-12-01T10:30:15');
    expect(new Date(iso).getTime()).toBe(new Date(2026, 11, 1, 10, 30, 15).getTime());
  });
  it('returns an empty string for an invalid instant', () => {
    expect(isoToLocalInput('nope')).toBe('');
  });
});
