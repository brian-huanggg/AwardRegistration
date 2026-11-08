import cleanup from 'testdata/text-cleanup.json';
import { checkName, checkSummary, countCodePoints, trimWhitespace } from './index';

describe('shared/text contract (testdata/text-cleanup.json)', () => {
  it('has cases', () => {
    expect(cleanup.cases.length).toBeGreaterThan(0);
  });

  it.each(cleanup.cases)('$id', (c) => {
    const r = c.kind === 'name' ? checkName(c.input, c.max) : checkSummary(c.input, c.max);
    expect(r.cleaned).toBe(c.want);
    expect(countCodePoints(r.cleaned)).toBe(c.wantCount);
    expect(r.problem).toBe(c.wantProblem);
  });
});

describe('trimWhitespace', () => {
  it('uses White_Space, not String.prototype.trim', () => {
    expect(trimWhitespace('\u0085a\u0085')).toBe('a');
    expect(trimWhitespace('﻿a﻿')).toBe('﻿a﻿');
  });
});
