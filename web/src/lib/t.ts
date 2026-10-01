import vi from "../../messages/vi.json";

type Leaves<T, P extends string = ""> = {
  [K in keyof T & string]: T[K] extends string ? `${P}${K}` : Leaves<T[K], `${P}${K}.`>;
}[keyof T & string];
export type MessageKey = Leaves<typeof vi>;

// Vietnamese only for now; English is a second file plus a locale switch later.
export function t(key: MessageKey): string {
  const hit = key.split(".").reduce<unknown>((o, k) => (o as Record<string, unknown>)?.[k], vi);
  return typeof hit === "string" ? hit : key;
}
