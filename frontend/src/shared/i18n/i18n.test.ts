import errorCodes from 'testdata/error-codes.json';
import { en, zhTW, detectLocale, initI18n, setLocale, errorText } from './index';

function flatten(o: object, prefix = ''): string[] {
  return Object.entries(o).flatMap(([k, v]) =>
    typeof v === 'object' && v !== null ? flatten(v, `${prefix}${k}.`) : [`${prefix}${k}`],
  );
}

describe('dictionaries', () => {
  it.each([
    ['en', en],
    ['zh-TW', zhTW],
  ])('%s has text for every code in error-codes.json plus UNKNOWN', (_n, dict) => {
    const missing = [...errorCodes.map((e) => e.code), 'UNKNOWN'].filter(
      (c) => !(dict.errors as Record<string, string>)[c]?.trim(),
    );
    expect(missing).toEqual([]);
  });

  it('has no extra error keys', () => {
    const expected = new Set([...errorCodes.map((e) => e.code), 'UNKNOWN']);
    expect(Object.keys(en.errors).filter((k) => !expected.has(k))).toEqual([]);
  });

  it('zh-TW and en have identical key sets and no empty values', () => {
    expect(flatten(zhTW).sort()).toEqual(flatten(en).sort());
    for (const dict of [en, zhTW]) {
      const empty = flatten(dict).filter((k) =>
        !k.split('.').reduce<unknown>((o, p) => (o as Record<string, unknown>)[p], dict),
      );
      expect(empty).toEqual([]);
    }
  });
});

describe('runtime', () => {
  beforeEach(() => localStorage.clear());

  it('detects the browser language, then remembers the choice', async () => {
    vi.spyOn(navigator, 'languages', 'get').mockReturnValue(['en-US']);
    expect(detectLocale()).toBe('en');
    await initI18n();
    await setLocale('zh-TW');
    expect(detectLocale()).toBe('zh-TW');
    expect(document.documentElement.lang).toBe('zh-TW');
    expect(errorText('TEAM_FULL')).toBe(zhTW.errors.TEAM_FULL);
    await setLocale('en');
    expect(errorText('TEAM_FULL')).toBe(en.errors.TEAM_FULL);
  });
});
