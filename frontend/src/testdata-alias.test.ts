import codes from 'testdata/error-codes.json';

describe('testdata alias', () => {
  it('resolves shared files from ../testdata', () => {
    expect(codes.some((c) => c.code === 'NOT_FOUND' && c.status === 404)).toBe(true);
  });
});
