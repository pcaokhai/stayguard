import { notFound } from "next/navigation";
import type { ReactNode } from "react";
import { LocaleProvider } from "@/lib/LocaleProvider";
import { LOCALES, type Locale } from "@/lib/locale";

export const dynamicParams = false;
export const generateStaticParams = () => LOCALES.map((locale) => ({ locale }));

export default async function LocaleLayout({
  children,
  params,
}: {
  children: ReactNode;
  params: Promise<{ locale: string }>;
}) {
  const { locale } = await params;
  if (!LOCALES.includes(locale as Locale)) notFound();
  return <LocaleProvider locale={locale as Locale}>{children}</LocaleProvider>;
}
