"use client";

import { Button } from "@/components/ui/button";
import { useRouter } from "next/navigation";
import type { components } from "../../api/generated/schema";
import { t, type MessageKey } from "../../lib/t";
import { useCreateDemoSession } from "./useCreateDemoSession";
import { lp } from "../../lib/locale";

type Role = components["schemas"]["Role"];

const ROLES: { role: Role; href: string; title: MessageKey; sub: MessageKey; icon: string }[] = [
  {
    role: "RECEPTIONIST",
    href: "/rooms",
    title: "login.desk",
    sub: "login.deskSub",
    icon: "M3 20h18M5 20V11h14v9M8 11V7a4 4 0 0 1 8 0v4",
  },
  {
    role: "OWNER",
    href: "/owner",
    title: "login.owner",
    sub: "login.ownerSub",
    icon: "M4 19V9M10 19V5M16 19v-7M22 19H2",
  },
  {
    role: "HOUSEKEEPING",
    href: "/housekeeping",
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
          <Button
            key={role}
            type="button"
            variant="outline"
            loading={start.isPending}
            onClick={() => start.mutate(role, { onSuccess: () => router.push(lp(href)) })}
            className={`h-auto min-h-[76px] justify-start gap-3.5 rounded-xl bg-card px-4 py-3.5 text-left ${
              i === 0 ? "border-2 border-primary" : ""
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
            <span className="flex flex-col gap-0.5 whitespace-normal">
              <span className="text-base font-semibold">{t(title)}</span>
              <span className="text-[13px] font-normal text-muted-foreground">{t(sub)}</span>
            </span>
          </Button>
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
