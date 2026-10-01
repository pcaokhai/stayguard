"use client";

import { useRouter } from "next/navigation";
import type { components } from "../../api/generated/schema";
import { t, type MessageKey } from "../../lib/t";
import { useCreateDemoSession } from "./useCreateDemoSession";

type Role = components["schemas"]["Role"];

const ROLES: { role: Role; href: string; title: MessageKey; sub: MessageKey; icon: string }[] = [
  {
    role: "RECEPTIONIST",
    href: "/vi/rooms",
    title: "login.desk",
    sub: "login.deskSub",
    icon: "M3 20h18M5 20V11h14v9M8 11V7a4 4 0 0 1 8 0v4",
  },
  {
    role: "OWNER",
    href: "/vi/owner",
    title: "login.owner",
    sub: "login.ownerSub",
    icon: "M4 19V9M10 19V5M16 19v-7M22 19H2",
  },
  {
    role: "HOUSEKEEPING",
    href: "/vi/housekeeping",
    title: "login.housekeeping",
    sub: "login.housekeepingSub",
    icon: "M4 21l6-6M14 4l6 6-8 8-6-6z",
  },
];

export function RolePicker() {
  const router = useRouter();
  const start = useCreateDemoSession();

  return (
    <main className="mx-auto flex min-h-dvh max-w-[480px] flex-col gap-7 px-6 pb-8 pt-12">
      <header className="flex flex-col gap-3 pt-6">
        <h1 className="text-[30px] font-bold leading-tight">{t("login.title")}</h1>
        <p className="text-[15px] leading-normal text-ink-2">{t("login.lead")}</p>
      </header>
      <section className="flex flex-col gap-3" aria-labelledby="try-as">
        <h2 id="try-as" className="text-sm font-semibold">
          {t("login.tryAs")}
        </h2>
        {ROLES.map(({ role, href, title, sub, icon }, i) => (
          <button
            key={role}
            type="button"
            disabled={start.isPending}
            onClick={() => start.mutate(role, { onSuccess: () => router.push(href) })}
            className={`flex min-h-[76px] items-center gap-3.5 rounded-card bg-surface px-4 py-3.5 text-left text-ink disabled:opacity-60 ${
              i === 0 ? "border-2 border-brand" : "border border-line"
            }`}
          >
            <svg
              width="28"
              height="28"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.8"
              strokeLinecap="round"
              strokeLinejoin="round"
              className="shrink-0"
              aria-hidden="true"
            >
              <path d={icon} />
            </svg>
            <span className="flex flex-col gap-0.5">
              <span className="text-base font-semibold">{t(title)}</span>
              <span className="text-[13px] text-muted">{t(sub)}</span>
            </span>
          </button>
        ))}
        {start.isError && (
          <p role="alert" className="text-sm text-warn">
            {t("login.failed")}
          </p>
        )}
      </section>
    </main>
  );
}
