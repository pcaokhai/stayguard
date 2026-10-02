import en from "../../messages/en.json";
import vi from "../../messages/vi.json";
import { getLocale } from "./locale";

type Leaves<T, P extends string = ""> = {
  [K in keyof T & string]: T[K] extends string ? `${P}${K}` : Leaves<T[K], `${P}${K}.`>;
}[keyof T & string];
export type MessageKey = Leaves<typeof vi>;

const lookup = (messages: unknown, key: string) => {
  const hit = key
    .split(".")
    .reduce<unknown>((o, k) => (o as Record<string, unknown>)?.[k], messages);
  return typeof hit === "string" ? hit : undefined;
};

// Locale comes from the route (`/vi`, `/en`); `npm run check:i18n` keeps the files in step.
export function t(key: MessageKey): string {
  return lookup(getLocale() === "en" ? en : vi, key) ?? lookup(vi, key) ?? key;
}

// Fills {name} placeholders: tf("stay.hoursMinutes", { h: 2, m: 35 }).
export const tf = (key: MessageKey, vars: Record<string, string | number>) =>
  Object.entries(vars).reduce((s, [k, v]) => s.replace(`{${k}}`, String(v)), t(key));
