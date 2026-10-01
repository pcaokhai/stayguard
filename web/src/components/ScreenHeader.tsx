import Link from "next/link";
import type { ReactNode } from "react";
import { t } from "../lib/t";

export function ScreenHeader({
  title,
  subtitle,
  back = "/vi/rooms",
  right,
}: {
  title: string;
  subtitle?: string;
  back?: string;
  right?: ReactNode;
}) {
  return (
    <header className="flex items-start gap-3 px-5 pb-4 pt-5">
      <Link
        href={back}
        aria-label={t("stay.back")}
        className="flex size-11 shrink-0 items-center justify-center"
      >
        <svg
          width="22"
          height="22"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2.2"
          strokeLinecap="round"
          strokeLinejoin="round"
          aria-hidden="true"
        >
          <path d="M15 5l-7 7 7 7" />
        </svg>
      </Link>
      <div className="flex-1">
        <h1 className="text-[22px] font-bold leading-tight">{title}</h1>
        {subtitle && <p className="text-sm text-muted">{subtitle}</p>}
      </div>
      {right}
    </header>
  );
}
