import i18next, { type i18n } from 'i18next';
import { initReactI18next } from 'react-i18next';
import type { ErrorKey } from './locales/en';
import { en } from './locales/en';
import { zhTW } from './locales/zh-TW';

export const LOCALES = ['zh-TW', 'en'] as const;
export type Locale = (typeof LOCALES)[number];

const STORAGE_KEY = 'locale';

function isLocale(v: unknown): v is Locale {
  return typeof v === 'string' && (LOCALES as readonly string[]).includes(v);
}

/** Remembered choice first, then the browser language, else zh-TW. */
export function detectLocale(): Locale {
  try {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (isLocale(saved)) return saved;
  } catch {
    // storage blocked: fall through to the browser language
  }
  const langs = typeof navigator === 'undefined' ? [] : (navigator.languages ?? [navigator.language]);
  for (const l of langs) {
    if (l.toLowerCase().startsWith('zh')) return 'zh-TW';
    if (l.toLowerCase().startsWith('en')) return 'en';
  }
  return 'zh-TW';
}

function applyLang(locale: Locale): void {
  if (typeof document !== 'undefined') document.documentElement.lang = locale;
}

export async function initI18n(locale: Locale = detectLocale()): Promise<i18n> {
  await i18next.use(initReactI18next).init({
    resources: { en: { translation: en }, 'zh-TW': { translation: zhTW } },
    lng: locale,
    fallbackLng: false, // a missing key must be visible, not silently English
    interpolation: { escapeValue: false }, // React escapes
    returnNull: false,
  });
  applyLang(locale);
  return i18next;
}

export async function setLocale(locale: Locale): Promise<void> {
  await i18next.changeLanguage(locale);
  applyLang(locale);
  try {
    localStorage.setItem(STORAGE_KEY, locale);
  } catch {
    // choice just isn't remembered
  }
}

/** Localized text for an API error code; never the server `detail`. */
export function errorText(code: ErrorKey, t: i18n['t'] = i18next.t.bind(i18next)): string {
  return t(`errors.${code}`);
}

export { en, zhTW };
export type { Dictionary, ErrorKey } from './locales/en';
