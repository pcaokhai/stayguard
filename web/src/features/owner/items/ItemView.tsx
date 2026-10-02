"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useState } from "react";
import { Pencil, Plus, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { FadeIn, SlidingPill } from "@/components/motion";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { QueryError } from "@/components/StateView";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { lp, localized } from "@/lib/locale";
import { formatVnd } from "@/lib/money";
import { t, tf, type MessageKey } from "@/lib/t";
import { cn } from "@/lib/utils";
import { clockOf, formatDayMonth } from "../format";
import { useIsOwner } from "../role";
import { ItemForm } from "./ItemForm";
import { RemoveItemDialog } from "./RemoveItemDialog";
import { RestockSheet } from "./RestockSheet";
import { useItems, useMovements, useUpdateItem, type Movement, type Service } from "./hooks";

type Filter = "ALL" | "IN" | "SALE" | "COUNT";
const FILTERS: { value: Filter; label: MessageKey }[] = [
  { value: "ALL", label: "stock.fAll" },
  { value: "IN", label: "stock.fIn" },
  { value: "SALE", label: "stock.fSales" },
  { value: "COUNT", label: "stock.fCounts" },
];
const KIND_TONE: Record<Movement["kind"], string> = {
  OPENING: "border-ok-line bg-ok-bg text-ok",
  IN: "border-ok-line bg-ok-bg text-ok",
  SALE: "border-line bg-maint-bg text-maint",
  COUNT: "border-info-line bg-info-bg text-info",
  ADJUST: "border-warn-line bg-warn-bg text-warn-ink",
};
const pill =
  "inline-block rounded-full border px-2.5 py-0.5 text-[12px] font-bold whitespace-nowrap";
const qty = (n: number) => (n > 0 ? `+${n}` : n < 0 ? `−${-n}` : "0");

function moveText(m: Movement): string {
  if (m.kind === "SALE")
    return m.ref ? tf("stock.text.SALE", { ref: m.ref }) : t("stock.text.SALE_NO_REF");
  if (m.kind === "COUNT")
    return t(m.quantity === 0 ? "stock.text.COUNT_OK" : "stock.text.COUNT_OFF");
  return t(`stock.text.${m.kind}` as MessageKey);
}
const moveBy = (m: Movement) =>
  m.unitCost != null
    ? tf("stock.costBy", { actor: m.actorName, cost: formatVnd(m.unitCost) })
    : m.actorName;
const when = (iso: string) => `${formatDayMonth(iso)} ${clockOf(iso)}`;

function Kpi({
  label,
  children,
  sub,
  warn,
}: {
  label: string;
  children: React.ReactNode;
  sub?: string;
  warn?: boolean;
}) {
  return (
    <Card
      className={cn(
        "gap-0.5 p-3.5 shadow-none",
        warn && "border-dirty-line bg-dirty-bg text-dirty",
      )}
    >
      <p className="text-[13px]">{label}</p>
      <p className="text-[22px] font-bold leading-tight lg:text-[26px]">{children}</p>
      {sub && <p className="text-[12px]">{sub}</p>}
    </Card>
  );
}

// Manage one item (boards ChiTietMatHang*): numbers, details, stock history, stop selling or remove.
export function ItemView() {
  const owner = useIsOwner();
  const code = useSearchParams().get("code");
  const router = useRouter();
  const items = useItems();
  const [filter, setFilter] = useState<Filter>("ALL");
  const moves = useMovements(code, filter === "ALL" ? undefined : filter === "IN" ? "IN" : filter);
  const update = useUpdateItem();
  const [dialog, setDialog] = useState<"edit" | "restock" | "remove" | null>(null);
  const item: Service | undefined = items.data?.find((x) => x.code === code);

  if (items.isError || moves.isError)
    return (
      <AppFrame tabs={false}>
        <QueryError onRetry={() => void Promise.all([items.refetch(), moves.refetch()])} />
      </AppFrame>
    );
  if (!item)
    return (
      <AppFrame tabs={false}>
        {items.isLoading ? (
          <Skeleton className="m-5 h-64 rounded-card" aria-busy="true" />
        ) : (
          <p className="p-8 text-center text-muted-foreground">{t("stock.notFound")}</p>
        )}
      </AppFrame>
    );

  const low = item.lowStockAt != null && item.stock <= item.lowStockAt;
  const unit = item.unit ?? "";
  const status = `${item.onSale === false ? t("stock.offSale") : t("stock.onSale")}${low ? ` · ${t("stock.lowSuffix")}` : ""}`;
  // Display only: the margin is the gap between two numbers the API gave, not a price the app sets.
  const margin = item.latestUnitCost != null ? item.price - item.latestUnitCost : null;
  const marginPct =
    margin != null && item.price > 0 ? Math.round((margin / item.price) * 100) : null;
  const perDay = item.soldLast7Days ? Math.round((item.soldLast7Days / 7) * 10) / 10 : 0;
  const rows = moves.data ?? [];
  const stop = () =>
    update.mutate(
      { code: item.code, body: { onSale: false } },
      {
        onSuccess: () => toast.success(t("stock.stopped")),
        onError: () => toast.error(t("stock.actionFailed")),
      },
    );

  return (
    <AppFrame tabs={false}>
      <main className="mx-auto flex w-full max-w-[1280px] flex-col gap-3 pb-10 lg:px-3">
        <TopBar
          title={localized(item.name)}
          subtitle={status}
          back="/owner/items"
          right={
            <span className="hidden gap-2 lg:flex">
              <Button asChild variant="outline" size="lg" className="font-bold">
                <Link href={lp("/owner/items")}>{t("stock.backToList")}</Link>
              </Button>
              <Button
                variant="outline"
                size="lg"
                className="font-bold"
                onClick={() => setDialog("edit")}
              >
                <Pencil aria-hidden="true" />
                {t("stock.editDetails")}
              </Button>
              <Button size="lg" className="font-bold" onClick={() => setDialog("restock")}>
                <Plus aria-hidden="true" />
                {t("stock.restock")}
              </Button>
            </span>
          }
        />
        <FadeIn className="flex flex-col gap-3 px-5">
          <div className="grid grid-cols-2 gap-3 lg:grid-cols-5">
            <Kpi
              warn={low}
              label={t("stock.inStock")}
              sub={
                item.lowStockAt != null ? tf("stock.alertAt", { n: item.lowStockAt }) : undefined
              }
            >
              {tf("stock.pcs", { n: item.stock, unit })}
            </Kpi>
            <Kpi label={t("stock.price")}>{formatVnd(item.price)}</Kpi>
            <Kpi label={t("stock.latestCost")}>
              {item.latestUnitCost != null ? formatVnd(item.latestUnitCost) : "—"}
            </Kpi>
            <div className="max-lg:hidden">
              <Kpi label={t("stock.margin")} sub={marginPct != null ? `${marginPct}%` : undefined}>
                {margin != null ? formatVnd(margin) : "—"}
              </Kpi>
            </div>
            <Kpi
              label={t("stock.sold7")}
              sub={perDay ? tf("stock.perDay", { n: perDay, unit }) : undefined}
            >
              {item.soldLast7Days ?? 0}
            </Kpi>
          </div>

          <div className="grid grid-cols-2 gap-2 lg:hidden">
            <Button size="lg" className="font-bold" onClick={() => setDialog("restock")}>
              <Plus aria-hidden="true" />
              {t("stock.restock")}
            </Button>
            <Button
              variant="outline"
              size="lg"
              className="font-bold"
              onClick={() => setDialog("edit")}
            >
              <Pencil aria-hidden="true" />
              {t("stock.editDetails")}
            </Button>
          </div>

          <div className="grid gap-3 lg:grid-cols-[280px_1fr] lg:items-start lg:gap-4">
            <Card className="hidden gap-1.5 p-5 shadow-none lg:flex">
              <h2 className="text-[13px] font-bold uppercase tracking-wide text-ink-2">
                {t("stock.details")}
              </h2>
              <dl className="text-[14px]">
                {[
                  [t("stock.nameEn"), item.name.en],
                  [t("stock.unit"), unit || "—"],
                  [t("stock.lowAt"), String(item.lowStockAt ?? "—")],
                  [
                    t("stock.status"),
                    item.onSale === false ? t("stock.offSale") : t("stock.onSale"),
                  ],
                ].map(([k, v]) => (
                  <div key={k} className="flex justify-between gap-3 py-1">
                    <dt className="text-ink-2">{k}</dt>
                    <dd className="text-right">{v}</dd>
                  </div>
                ))}
              </dl>
            </Card>

            <div className="flex flex-col gap-2">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <h2 className="text-[13px] font-bold uppercase tracking-wide text-ink-2">
                  {t("stock.history")}
                </h2>
                <ToggleGroup
                  type="single"
                  value={filter}
                  onValueChange={(v) => v && setFilter(v as Filter)}
                  aria-label={t("stock.history")}
                  className="hidden gap-2 lg:flex"
                >
                  {FILTERS.map((f) => (
                    <ToggleGroupItem
                      key={f.value}
                      value={f.value}
                      className="relative h-10 rounded-full! border border-border bg-card px-4 text-[14px] font-bold data-[state=on]:border-transparent data-[state=on]:bg-transparent data-[state=on]:text-primary-foreground"
                    >
                      {filter === f.value && (
                        <SlidingPill
                          id="move-pill"
                          className="absolute inset-0 rounded-full bg-primary"
                        />
                      )}
                      <span className="relative">{t(f.label)}</span>
                    </ToggleGroupItem>
                  ))}
                </ToggleGroup>
              </div>
              {moves.isLoading && <Skeleton className="h-40 rounded-card" />}
              {!moves.isLoading && rows.length === 0 && (
                <p className="py-4 text-sm text-muted-foreground">{t("stock.noMoves")}</p>
              )}

              <Card className="gap-0 p-0 shadow-none lg:hidden">
                {rows.map((m, i) => (
                  <div
                    key={i}
                    className="flex items-center gap-3 border-t border-border px-4 py-3 first:border-t-0"
                  >
                    <span className="min-w-0 flex-1 leading-snug">
                      <b className="block text-[15px]">{moveText(m)}</b>
                      <span className="text-[13px] text-ink-2">
                        {when(m.at)} · {moveBy(m)}
                      </span>
                    </span>
                    <span
                      className={cn(
                        pill,
                        "text-[13px]",
                        m.quantity > 0
                          ? KIND_TONE.IN
                          : m.kind === "COUNT"
                            ? KIND_TONE.COUNT
                            : KIND_TONE.SALE,
                      )}
                    >
                      {qty(m.quantity)}
                    </span>
                  </div>
                ))}
              </Card>
              {rows.length > 0 && (
                <Card className="hidden gap-0 overflow-x-auto p-0 shadow-none lg:block">
                  <table className="w-full text-[14px]">
                    <thead>
                      <tr className="h-11 text-left text-[12px] uppercase tracking-wide text-ink-2">
                        {(["colTime", "colType", "colDetail", "colQty", "colBy"] as const).map(
                          (k) => (
                            <th key={k} scope="col" className="px-3 font-bold first:pl-5">
                              {t(`stock.${k}`)}
                            </th>
                          ),
                        )}
                      </tr>
                    </thead>
                    <tbody>
                      {rows.map((m, i) => (
                        <tr key={i} className="h-14 border-t border-border">
                          <td className="px-3 pl-5">{when(m.at)}</td>
                          <td className="px-3">
                            <span className={cn(pill, KIND_TONE[m.kind])}>
                              {t(`stock.kind.${m.kind}` as MessageKey)}
                            </span>
                          </td>
                          <td className="px-3 font-bold">{moveText(m)}</td>
                          <td className="px-3 font-bold">{qty(m.quantity)}</td>
                          <td className="px-3 text-ink-2">{moveBy(m)}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </Card>
              )}
            </div>
          </div>

          <Card className="flex-col gap-3 border-destructive/30 p-4 shadow-none sm:flex-row sm:items-center">
            <div className="flex-1 leading-snug">
              <b className="block text-[16px]">{t("stock.danger")}</b>
              <span className="text-[13px] text-ink-2">{t("stock.dangerBody")}</span>
            </div>
            <div className="flex gap-2">
              <Button
                variant="outline"
                size="lg"
                className="font-bold"
                disabled={item.onSale === false || update.isPending}
                onClick={stop}
              >
                {t("stock.stopSelling")}
              </Button>
              {owner && (
                <Button
                  variant="outline"
                  size="lg"
                  className="border-destructive/40 font-bold text-destructive hover:bg-destructive/10"
                  onClick={() => setDialog("remove")}
                >
                  <Trash2 aria-hidden="true" />
                  {t("stock.remove")}
                </Button>
              )}
            </div>
          </Card>
        </FadeIn>
      </main>
      {dialog === "edit" && <ItemForm item={item} onClose={() => setDialog(null)} />}
      {dialog === "restock" && <RestockSheet item={item} onClose={() => setDialog(null)} />}
      {dialog === "remove" && (
        <RemoveItemDialog
          item={item}
          onClose={() => setDialog(null)}
          onDone={() => router.replace(lp("/owner/items"))}
        />
      )}
    </AppFrame>
  );
}
