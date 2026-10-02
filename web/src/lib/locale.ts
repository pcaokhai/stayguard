export const LOCALES = ["vi", "en"] as const;
export type Locale = (typeof LOCALES)[number];

// ponytail: module state set by <LocaleProvider> on render. The pages are client-rendered
// (static export), so one tab has one locale; a per-request context is not needed.
let current: Locale = "vi";
export const setLocale = (l: Locale) => {
  current = l;
};
export const getLocale = () => current;

// "/rooms?b=1" -> "/vi/rooms?b=1"
export const lp = (path: string) => `/${current}${path === "/" ? "" : path}`;
