import { sentAtParts } from './requests';

describe('sentAtParts', () => {
  const now = new Date(2026, 10, 8, 12, 0).getTime();

  it('uses a relative day for today and yesterday', () => {
    expect(sentAtParts(new Date(2026, 10, 8, 10, 42).toISOString(), now, 'en')).toEqual({ kind: 'today', time: expect.stringMatching(/^10:42\sAM$/) });
    expect(sentAtParts(new Date(2026, 10, 7, 23, 5).toISOString(), now, 'en')).toEqual({ kind: 'yesterday', time: expect.stringMatching(/^11:05\sPM$/) });
  });

  it('uses a medium date otherwise', () => {
    expect(sentAtParts(new Date(2026, 10, 6, 9, 0).toISOString(), now, 'en')).toEqual({ kind: 'date', date: 'Nov 6, 2026' });
  });
});
