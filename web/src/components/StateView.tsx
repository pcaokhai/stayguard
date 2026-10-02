"use client";

import Link from "next/link";
import { Check, Lock, RefreshCw, type LucideIcon } from "lucide-react";
import type { ReactNode } from "react";
import { Button } from "@/components/ui/button";
import { FadeIn } from "@/components/motion";
import { lp } from "@/lib/locale";
import { t } from "@/lib/t";
import { cn } from "@/lib/utils";

const TONE = {
  neutral: "bg-secondary text-ink-2",
  info: "bg-info-bg text-info",
  ok: "bg-ok-bg text-ok",
} as const;

// Shared full-height state: icon tile, title, text, actions pinned to the bottom (boards P29-P32).
export function StateView({
  icon: Icon,
  tone = "neutral",
  title,
  body,
  children,
}: {
  icon: LucideIcon;
  tone?: keyof typeof TONE;
  title: string;
  body: string;
  children?: ReactNode;
}) {
  return (
    <section className="flex min-h-[calc(100dvh-8rem)] flex-col px-5 pb-6 pt-4">
      <FadeIn className="flex flex-1 flex-col items-center justify-center gap-3 text-center">
        <span
          className={cn("flex size-[72px] items-center justify-center rounded-[20px]", TONE[tone])}
        >
          <Icon className="size-7" aria-hidden="true" />
        </span>
        <h2 className="max-w-[300px] text-[22px] font-bold leading-tight">{title}</h2>
        <p className="max-w-[300px] text-sm leading-normal text-muted-foreground">{body}</p>
      </FadeIn>
      <div className="mx-auto flex w-full max-w-[480px] flex-col gap-2.5">{children}</div>
    </section>
  );
}

const BackToRooms = () => (
  <Button asChild variant="outline" size="lg">
    <Link href={lp("/rooms")}>{t("state.backRooms")}</Link>
  </Button>
);

export function OfflineState({ onRetry }: { onRetry: () => void }) {
  return (
    <StateView icon={RefreshCw} title={t("state.offlineTitle")} body={t("state.offlineBody")}>
      <Button size="lg" onClick={onRetry}>
        <RefreshCw aria-hidden="true" />
        {t("state.retry")}
      </Button>
      <BackToRooms />
    </StateView>
  );
}

export function ServerErrorState({ onRetry, code }: { onRetry: () => void; code?: string }) {
  const body =
    t("state.errorBody") + (code ? ` ${t("state.errorCode").replace("{code}", code)}` : "");
  return (
    <StateView icon={RefreshCw} title={t("state.errorTitle")} body={body}>
      <Button size="lg" onClick={onRetry}>
        <RefreshCw aria-hidden="true" />
        {t("state.retry")}
      </Button>
      <BackToRooms />
    </StateView>
  );
}

export function ForbiddenState({ building }: { building?: string }) {
  const body = building
    ? t("state.forbiddenViewOnly").replace("{building}", building)
    : t("state.forbiddenBody");
  return (
    <StateView icon={Lock} tone="info" title={t("state.forbiddenTitle")} body={body}>
      <Button asChild size="lg">
        <Link href={lp("/rooms")}>{t("state.backRooms")}</Link>
      </Button>
    </StateView>
  );
}

export function EmptyState({
  title,
  body,
  action,
}: {
  title: string;
  body: string;
  action?: ReactNode;
}) {
  return (
    <StateView icon={Check} tone="ok" title={title} body={body}>
      {action}
    </StateView>
  );
}

// Picks the offline or server variant for a failed query.
export function QueryError({ onRetry }: { onRetry: () => void }) {
  const offline = typeof navigator !== "undefined" && !navigator.onLine;
  return offline ? <OfflineState onRetry={onRetry} /> : <ServerErrorState onRetry={onRetry} />;
}
