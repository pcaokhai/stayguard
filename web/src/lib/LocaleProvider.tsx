"use client";

import { useEffect, type ReactNode } from "react";
import { setLocale, type Locale } from "./locale";

export function LocaleProvider({ locale, children }: { locale: Locale; children: ReactNode }) {
  setLocale(locale);
  useEffect(() => {
    document.documentElement.lang = locale;
  }, [locale]);
  return children;
}
