import cleanup from 'testdata/text-cleanup.json';
import { checkEmail, checkName, checkSummary, countCodePoints, trimWhitespace } from './index';

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

describe('checkEmail (PRD §6.3)', () => {
  it.each([
    ['a@x.com', 'a@x.com', ''],
    ['  A@X.Com ', 'a@x.com', ''],
    ['　a@x　', 'a@x', ''],
    ['a@localhost', 'a@localhost', ''],
    ['', '', 'REQUIRED'],
    ['   ', '', 'REQUIRED'],
    ['a b@x.com', 'a b@x.com', 'INVALID_FORMAT'],
    ['a@x .com', 'a@x .com', 'INVALID_FORMAT'],
    ['é@x.com', 'é@x.com', 'INVALID_FORMAT'],
    ['ax.com', 'ax.com', 'INVALID_FORMAT'],
    ['@x.com', '@x.com', 'INVALID_FORMAT'],
    ['a@', 'a@', 'INVALID_FORMAT'],
    ['a@@x.com', 'a@@x.com', 'INVALID_FORMAT'],
    ['a@b@x.com', 'a@b@x.com', 'INVALID_FORMAT'],
  ])('%j', (input, cleaned, problem) => {
    expect(checkEmail(input)).toEqual({ cleaned, problem });
  });

  it('allows 254 characters and rejects 255', () => {
    const at = (n: number) => `${'a'.repeat(n - 2)}@x`;
    expect(checkEmail(at(254)).problem).toBe('');
    expect(checkEmail(at(255)).problem).toBe('OUT_OF_RANGE');
  });
});
