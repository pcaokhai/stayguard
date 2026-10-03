"use client";

import { Printer } from "lucide-react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { QueryError } from "@/components/StateView";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { localized, lp } from "../../lib/locale";
import { formatVnd } from "../../lib/money";
import { t, tf } from "../../lib/t";
import { clockLocale, formatClock, minutesBetween } from "../../lib/time";
import { useReceipt } from "../stay/hooks";
import { billLineLabel } from "../stay/labels";

const line = "flex justify-between gap-3";
const dashed = "border-t border-dashed border-ink-2/40";

// Board P8: shown at 80 mm wide so what is on screen is what the thermal printer prints.
export function ReceiptView() {
  const id = useSearchParams().get("invoice");
  const q = useReceipt(id);
  const r = q.data;
  if (q.isError)
    return (
      <AppFrame tabs={false}>
        <QueryError onRetry={() => void q.refetch()} />
      </AppFrame>
    );
  const mins = r ? minutesBetween(r.checkInAt, r.checkOutAt) : 0;
  const date =
    r &&
    new Date(r.checkOutAt).toLocaleDateString(clockLocale(), { day: "2-digit", month: "2-digit" });

  return (
    <AppFrame tabs={false}>
      {/* Only this page prints on an 80 mm roll (board BienLai); @page cannot be scoped by selector, so it exists only while this view is mounted. */}
      <style>{"@page { size: 80mm 200mm; margin: 4mm; }"}</style>
      <main className="mx-auto flex w-full max-w-[480px] flex-1 flex-col print:max-w-none">
        <div className="print:hidden">
          <TopBar title={t("receipt.title")} subtitle={t("receipt.preview")} back="/rooms" />
        </div>
        <div className="flex flex-1 flex-col gap-3 px-5 pb-8 print:p-0">
          {!r ? (
            <Skeleton className="h-96 w-full max-w-[302px] self-center" />
          ) : (
            <article
              aria-label={t("receipt.title")}
              className="mx-auto flex w-full max-w-[302px] flex-col gap-2 rounded-xl border bg-card p-5 text-[13px] print:max-w-[72mm] print:border-0 print:p-0 print:text-[11px]"
            >
              <header className="pb-1 text-center">
                <h1 className="text-lg font-bold">{r.propertyName}</h1>
                {(r.propertyAddress || r.propertyPhone) && (
                  <p className="text-xs text-muted-foreground">
                    {[r.propertyAddress, r.propertyPhone].filter(Boolean).join(" · ")}
                  </p>
                )}
              </header>
              <div className={`${dashed} flex flex-col gap-1.5 pt-2`}>
                <p className={line}>
                  <span>{t("receipt.bill")}</span>
                  <span>{r.billCode}</span>
                </p>
                <p className={line}>
                  <span>{t("receipt.room")}</span>
                  <span>{r.roomCode}</span>
                </p>
                <p className={line}>
                  <span>{t("receipt.inOut")}</span>
                  <span>
                    {formatClock(r.checkInAt)} · {formatClock(r.checkOutAt)}, {date}
                  </span>
                </p>
                <p className={line}>
                  <span>{t("receipt.duration")}</span>
                  <span>{tf("stay.hoursMinutes", { h: Math.floor(mins / 60), m: mins % 60 })}</span>
                </p>
              </div>
              <div className={`${dashed} flex flex-col gap-1.5 pt-2`}>
                {r.lines.map((l) => (
                  <p key={l.code} className={line}>
                    <span>
                      {billLineLabel(l.code)}
                      {l.quantity > 1 ? ` × ${l.quantity}` : ""}
                    </span>
                    <span>{formatVnd(l.amount)}</span>
                  </p>
                ))}
                {r.extras?.map((x) => (
                  <p key={x.serviceCode} className={line}>
                    <span>
                      {localized(x.name)} × {x.quantity}
                    </span>
                    <span>{formatVnd(x.amount)}</span>
                  </p>
                ))}
              </div>
              <div className={`${dashed} flex flex-col gap-1.5 pt-2`}>
                <p className={`${line} text-sm font-bold`}>
                  <span>{t("receipt.total")}</span>
                  <span>{formatVnd(r.total)}</span>
                </p>
                {r.deposit ? (
                  <p className={line}>
                    <span>{t("receipt.deposit")}</span>
                    <span>-{formatVnd(r.deposit)}</span>
                  </p>
                ) : null}
                {r.payments.map((p) => (
                  <p key={p.paymentId} className={line}>
                    <span>
                      {tf(p.method === "CASH" ? "receipt.cash" : "receipt.transfer", {
                        time: formatClock(p.at),
                      })}
                    </span>
                    <span>{formatVnd(p.amount)}</span>
                  </p>
                ))}
              </div>
              <p className={`${dashed} pt-2 text-center text-xs text-muted-foreground`}>
                {t("receipt.thanks")}
              </p>
            </article>
          )}
          <div className="mt-auto flex flex-col gap-3 pt-6 print:hidden">
            <Button size="lg" disabled={!r} onClick={() => window.print()}>
              <Printer aria-hidden="true" />
              {t("receipt.print")}
            </Button>
            <Button asChild variant="outline" size="lg">
              <Link href={lp("/rooms")}>{t("receipt.done")}</Link>
            </Button>
          </div>
        </div>
      </main>
    </AppFrame>
  );
}
