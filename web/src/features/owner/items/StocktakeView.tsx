"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { Info } from "lucide-react";
import { toast } from "sonner";
import { FadeIn } from "@/components/motion";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { QueryError } from "@/components/StateView";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";
import { newIdempotencyKey } from "@/lib/api";
import { lp, localized } from "@/lib/locale";
import { formatVnd } from "@/lib/money";
import { t, tf } from "@/lib/t";
import { cn } from "@/lib/utils";
import { useMe } from "../../session/useMe";
import { formatDayMonth, localDay } from "../format";
import { useItems, useStocktake } from "./hooks";

// Count the shelf and confirm (boards KiemKho, KiemKhoPC). Counts start at the system number; the
// server records the differences and alerts the owner.
export function StocktakeView() {
  const router = useRouter();
  const me = useMe();
  const items = useItems();
  const take = useStocktake();
  const [key] = useState(newIdempotencyKey);
  const [counts, setCounts] = useState<Record<string, string>>({});
  const [note, setNote] = useState("");
  const list = items.data ?? [];
  const counted = (code: string, stock: number) =>
    counts[code] !== undefined && counts[code] !== "" ? Number(counts[code]) : stock;
  const diffs = list.map((s) => ({ s, diff: counted(s.code, s.stock) - s.stock }));
  const off = diffs.filter((d) => d.diff !== 0);
  // Display estimate before confirming; the saved figure comes from the server's stocktake result.
  const value = off.reduce((n, d) => n + d.diff * d.s.price, 0);
  const today = formatDayMonth(localDay(new Date()));

  if (items.isError)
    return (
      <AppFrame tabs={false}>
        <QueryError onRetry={() => void items.refetch()} />
      </AppFrame>
    );

  const submit = () =>
    take.mutate(
      {
        key,
        lines: list.map((s) => ({ serviceCode: s.code, counted: counted(s.code, s.stock) })),
        note: note.trim() || null,
      },
      {
        onSuccess: () => {
          toast.success(t("stock.taken"));
          router.replace(lp("/owner/items"));
        },
        onError: () => toast.error(t("stock.takeFailed")),
      },
    );
  const result = (diff: number) => (
    <span
      className={cn(
        "rounded-full border px-2.5 py-0.5 text-[12px] font-bold whitespace-nowrap",
        diff === 0
          ? "border-ok-line bg-ok-bg text-ok"
          : "border-warn-line bg-warn-bg text-warn-ink",
      )}
    >
      {diff === 0 ? t("stock.match") : tf("stock.off", { n: diff > 0 ? `+${diff}` : String(diff) })}
    </span>
  );
  const field = (s: (typeof list)[number]) => (
    <Input
      value={counts[s.code] ?? String(s.stock)}
      inputMode="numeric"
      aria-label={`${t("stock.counted")} ${localized(s.name)}`}
      onChange={(e) => setCounts({ ...counts, [s.code]: e.target.value.replace(/\D/g, "") })}
      className="h-11 w-[72px] rounded-[10px] bg-card text-center text-[16px] font-bold"
    />
  );
  const summary = (
    <Card className="gap-1.5 p-4 shadow-none">
      <h2 className="hidden text-[13px] font-bold uppercase tracking-wide text-ink-2 lg:block">
        {t("stock.summary")}
      </h2>
      <dl className="text-[14px]">
        <div className="flex justify-between py-0.5 lg:flex">
          <dt className="text-ink-2">{t("stock.by")}</dt>
          <dd className="font-bold">{me.data?.user.name}</dd>
        </div>
        <div className="flex justify-between py-0.5">
          <dt className="text-ink-2">{t("stock.itemsOff")}</dt>
          <dd className="font-bold">{off.length}</dd>
        </div>
        <div className="flex justify-between py-0.5">
          <dt className="text-ink-2">{t("stock.valueOff")}</dt>
          <dd className={cn("font-bold", value < 0 && "text-destructive")}>
            {value < 0 ? `−${formatVnd(-value)}` : formatVnd(value)}
          </dd>
        </div>
      </dl>
    </Card>
  );

  return (
    <AppFrame tabs={false}>
      <main className="mx-auto flex w-full max-w-[1280px] flex-col gap-3 pb-10 lg:px-3">
        <TopBar
          title={t("stock.takeTitle")}
          subtitle={tf("stock.takeSub", { date: today, by: me.data?.user.name ?? "" })}
          back="/owner/items"
        />
        {items.isLoading ? (
          <Skeleton className="mx-5 h-64 rounded-card" aria-busy="true" />
        ) : (
          <FadeIn className="grid gap-3 px-5 lg:grid-cols-[1fr_320px] lg:items-start lg:gap-4">
            <Card className="gap-0 overflow-hidden p-0 shadow-none">
              <div className="hidden grid-cols-[1fr_90px_100px_130px] px-5 py-3 text-[12px] font-bold uppercase tracking-wide text-ink-2 lg:grid">
                <span>{t("items.colItem")}</span>
                <span>{t("stock.systemCol")}</span>
                <span>{t("stock.counted")}</span>
                <span>{t("stock.result")}</span>
              </div>
              {diffs.map(({ s, diff }) => (
                <div
                  key={s.code}
                  className="flex items-center gap-3 border-t border-border px-4 py-3 first:border-t-0 lg:grid lg:grid-cols-[1fr_90px_100px_130px] lg:px-5"
                >
                  <span className="min-w-0 flex-1 leading-snug">
                    <b className="block text-[16px]">{localized(s.name)}</b>
                    <span className="text-[13px] text-ink-2 lg:hidden">
                      {tf("stock.system", { n: s.stock })}
                    </span>
                  </span>
                  <span className="hidden text-[14px] lg:block">{s.stock}</span>
                  {field(s)}
                  {result(diff)}
                </div>
              ))}
            </Card>
            <div className="flex flex-col gap-3">
              {summary}
              <label className="flex flex-col gap-1 text-[13px] font-bold lg:hidden">
                {t("stock.note")}
                <Textarea
                  value={note}
                  onChange={(e) => setNote(e.target.value)}
                  className="min-h-24 rounded-[10px] bg-card text-[15px] font-normal"
                />
              </label>
              <p className="flex items-start gap-2 rounded-card border border-warn-line bg-warn-bg p-3 text-[13px] text-warn-ink">
                <Info className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
                {t("stock.takeInfo")}
              </p>
              <Button
                size="lg"
                className="font-bold"
                disabled={take.isPending || list.length === 0}
                onClick={submit}
              >
                {t("stock.confirm")}
              </Button>
            </div>
          </FadeIn>
        )}
      </main>
    </AppFrame>
  );
}
