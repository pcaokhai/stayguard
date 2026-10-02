"use client";

import { useState } from "react";
import { ChevronRight, Info, ListChecks, Plus, Trash2 } from "lucide-react";
import { FadeIn, StaggerList } from "@/components/motion";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { EmptyState, QueryError } from "@/components/StateView";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { lp, localized } from "@/lib/locale";
import { formatVnd } from "@/lib/money";
import { t, tf } from "@/lib/t";
import { cn } from "@/lib/utils";
import Link from "next/link";
import { useIsOwner } from "../role";
import { ItemForm } from "./ItemForm";
import { RemoveItemDialog } from "./RemoveItemDialog";
import { useItems, type Service } from "./hooks";

const detailHref = (s: Service) => lp(`/owner/item?code=${encodeURIComponent(s.code)}`);
const isLow = (s: Service) => s.lowStockAt != null && s.stock <= s.lowStockAt;
const pill = "inline-block rounded-full border px-3 py-1 text-[13px] font-bold whitespace-nowrap";
const Stock = ({ s }: { s: Service }) => (
  <span
    className={cn(
      pill,
      isLow(s) ? "border-dirty-line bg-dirty-bg text-dirty" : "border-line bg-maint-bg text-maint",
    )}
  >
    {tf("items.left", { n: s.stock })}
  </span>
);

export function ItemsView() {
  const owner = useIsOwner();
  const q = useItems();
  const [form, setForm] = useState<{ item?: Service } | null>(null);
  const [removing, setRemoving] = useState<Service | null>(null);
  const items = q.data ?? [];
  const low = items.filter(isLow).length;

  if (q.isError)
    return (
      <AppFrame tabs={false}>
        <QueryError onRetry={() => void q.refetch()} />
      </AppFrame>
    );

  const add = (
    <Button
      size="lg"
      className="w-full border-dashed md:w-auto md:border-solid"
      onClick={() => setForm({})}
    >
      <Plus aria-hidden="true" />
      {t("items.add")}
    </Button>
  );
  const stocktake = (
    <Button asChild variant="outline" size="lg" className="font-bold">
      <Link href={lp("/owner/stocktake")}>
        <ListChecks aria-hidden="true" />
        {t("items.stocktake")}
      </Link>
    </Button>
  );

  return (
    <AppFrame tabs={false}>
      <main className="mx-auto flex w-full max-w-[1280px] flex-col gap-3 pb-10 lg:px-3">
        <TopBar
          title={t("items.title")}
          subtitle={
            low
              ? tf("items.sub", { n: items.length, l: low })
              : tf("items.subNone", { n: items.length })
          }
          back="/owner/settings"
          right={
            <span className="hidden gap-2 md:flex">
              {stocktake}
              {add}
            </span>
          }
        />
        <div className="flex flex-col gap-3 px-5">
          {q.isLoading && <Skeleton className="h-64 rounded-card" aria-busy="true" />}
          {!q.isLoading && items.length === 0 && (
            <EmptyState title={t("items.title")} body={t("items.empty")} />
          )}

          <StaggerList className="md:hidden">
            <Card className="gap-0 p-0 shadow-none">
              {items.map((s) => (
                <Link
                  key={s.code}
                  href={detailHref(s)}
                  className="flex min-h-[72px] w-full items-center gap-2 border-t border-border px-4 py-3 text-left first:border-t-0"
                >
                  <span className="min-w-0 flex-1 leading-snug">
                    <b className="block text-[17px]">{localized(s.name)}</b>
                    <span className="text-[13px] text-ink-2">{formatVnd(s.price)}</span>
                  </span>
                  <Stock s={s} />
                  <ChevronRight className="size-4 text-muted-foreground" aria-hidden="true" />
                </Link>
              ))}
            </Card>
          </StaggerList>

          {items.length > 0 && (
            <FadeIn className="hidden md:block">
              <Card className="gap-0 overflow-x-auto p-0 shadow-none">
                <table className="w-full min-w-[720px] text-[14px]">
                  <thead>
                    <tr className="h-11 text-left text-[12px] uppercase tracking-wide text-ink-2">
                      {(["colItem", "colPrice", "colStock", "colSold"] as const).map((k) => (
                        <th key={k} scope="col" className="px-3 font-bold first:pl-5">
                          {t(`items.${k}`)}
                        </th>
                      ))}
                      <th scope="col" className="sr-only">
                        {t("items.manage")}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {items.map((s) => (
                      <tr
                        key={s.code}
                        className="border-t border-border transition-colors duration-100 hover:bg-sunken/50"
                      >
                        <td className="px-3 py-3.5 pl-5 text-[16px] font-bold">
                          {localized(s.name)}
                          {s.onSale === false && (
                            <span className="ml-2 text-[12px] font-normal text-muted-foreground">
                              {t("items.offSale")}
                            </span>
                          )}
                        </td>
                        <td className="px-3">{formatVnd(s.price)}</td>
                        <td className="px-3">
                          <Stock s={s} />
                        </td>
                        <td className="px-3">{s.soldLast7Days ?? "—"}</td>
                        <td className="px-3 pr-5">
                          <span className="flex justify-end gap-2">
                            <Button asChild variant="outline" size="lg" className="font-bold">
                              <Link href={detailHref(s)}>{t("items.manage")}</Link>
                            </Button>
                            {owner && (
                              <Button
                                variant="outline"
                                size="icon-lg"
                                aria-label={`${t("stock.remove")} ${localized(s.name)}`}
                                className="border-destructive/40 text-destructive hover:bg-destructive/10"
                                onClick={() => setRemoving(s)}
                              >
                                <Trash2 aria-hidden="true" />
                              </Button>
                            )}
                          </span>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </Card>
            </FadeIn>
          )}
          <p className="flex items-start gap-2 text-[13px] text-muted-foreground">
            <Info className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
            <span className="md:hidden">{t("items.hint")}</span>
            <span className="hidden md:inline">{t("items.hintPc")}</span>
          </p>
          <div className="grid grid-cols-2 gap-2 md:hidden">
            {stocktake}
            {add}
          </div>
        </div>
      </main>
      {form && (
        <ItemForm key={form.item?.code ?? "new"} item={form.item} onClose={() => setForm(null)} />
      )}
    </AppFrame>
  );
}
