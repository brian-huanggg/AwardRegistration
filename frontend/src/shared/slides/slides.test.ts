import fixture from 'testdata/expected-filename.json';
import { expectedFilenameBase, filenameTooLong, MAX_SLIDES_BYTES, precheckSlides, utf8Bytes, type Category } from './index';

describe('expectedFilenameBase (testdata/expected-filename.json)', () => {
  it.each(fixture.cases.map((c) => [c.id, c] as const))('%s', (_id, c) => {
    const base = expectedFilenameBase(c.category as Category, c.org, c.leaderName, c.leaderEmpNo, c.ideaName);
    expect(base).toBe(c.wantBase);
    expect(utf8Bytes(base + '.pptx')).toBe(c.wantBytes);
    expect(filenameTooLong(base)).toBe(c.wantTooLong);
  });
});

const PDF = '%PDF-1.7\n';
const ZIP = 'PK\u0003\u0004rest';
const BASE = 'I_BSID_王小明_089290_成立供應商永續學院';

const file = (name: string, content: string | Uint8Array = PDF) => new File([content as BlobPart], name);

describe('precheckSlides (PRD §6.8 order)', () => {
  it.each([
    ['no file', [], 'REQUIRED'],
    ['two files', [file(`${BASE}.pdf`), file(`${BASE}.pdf`)], 'REQUIRED'],
    ['empty', [file(`${BASE}.pdf`, '')], 'EMPTY_FILE'],
    ['PDF bytes, .pptx name', [file(`${BASE}.pptx`)], 'UNSUPPORTED_FILE_TYPE'],
    ['ZIP bytes, .pdf name', [file(`${BASE}.pdf`, ZIP)], 'UNSUPPORTED_FILE_TYPE'],
    ['.docx', [file(`${BASE}.docx`, ZIP)], 'UNSUPPORTED_FILE_TYPE'],
    ['no extension', [file(BASE)], 'UNSUPPORTED_FILE_TYPE'],
    ['wrong name', [file('slides.pdf')], 'FILENAME_MISMATCH'],
    ['name with a path', [file(`dir/${BASE}.pdf`)], 'FILENAME_MISMATCH'],
    ['PDF ok', [file(`${BASE}.pdf`)], ''],
    ['PPTX ok, upper-case extension', [file(`${BASE}.PPTX`, ZIP)], ''],
    ['NFD filename (macOS) matches', [file('I_ICSD_Zoe_000002_Café.pdf')], ''],
  ] as const)('%s → %j', async (_n, files, want) => {
    const base = files[0]?.name.startsWith('I_ICSD') ? 'I_ICSD_Zoe_000002_Café' : BASE;
    expect(await precheckSlides(files, base)).toBe(want);
  });

  it('size is checked before emptiness and type; 5 MiB is allowed', async () => {
    const big = file(`${BASE}.pdf`);
    Object.defineProperty(big, 'size', { value: MAX_SLIDES_BYTES + 1 });
    expect(await precheckSlides([big], BASE)).toBe('FILE_TOO_LARGE');
    const exact = new File([PDF, new Uint8Array(MAX_SLIDES_BYTES - PDF.length)], `${BASE}.pdf`);
    expect(exact.size).toBe(MAX_SLIDES_BYTES);
    expect(await precheckSlides([exact], BASE)).toBe('');
  });

  it('skips the filename step while no expected name exists', async () => {
    expect(await precheckSlides([file('anything.pdf')], null)).toBe('');
    expect(await precheckSlides([file('anything.pptx')], null)).toBe('UNSUPPORTED_FILE_TYPE');
  });
});
