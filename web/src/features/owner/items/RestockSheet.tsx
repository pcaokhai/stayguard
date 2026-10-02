"use client";

import { useState } from "react";
import { toast } from "sonner";
import { ResponsiveDialog } from "@/components/ResponsiveDialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { newIdempotencyKey } from "@/lib/api";
import { localized } from "@/lib/locale";
import { parseVnd, vndNumber } from "@/lib/money";
import { t, tf } from "@/lib/t";
import { useRestock, type Service } from "./hooks";

// Bottom sheet on phones, dialog from 640 px (boards NhapThemHang, NhapThemHangPC).
export function RestockSheet({ item, onClose }: { item: Service; onClose: () => void }) {
  const restock = useRestock();
  const [key] = useState(newIdempotencyKey);
  const [qty, setQty] = useState("");
  const [cost, setCost] = useState(
    item.latestUnitCost != null ? vndNumber(item.latestUnitCost) : "",
  );
  const n = Number(qty);
  const ok = /^\d+$/.test(qty) && n > 0 && /\d/.test(cost);
  const unit = item.unit ?? "";
  const submit = () =>
    ok &&
    restock.mutate(
      { code: item.code, key, quantity: n, unitCost: parseVnd(cost) },
      {
        onSuccess: () => {
          toast.success(t("stock.restocked"));
          onClose();
        },
        onError: () => toast.error(t("stock.actionFailed")),
      },
    );
  return (
    <ResponsiveDialog
      open
      onOpenChange={(o) => !o && onClose()}
      title={tf("stock.restockTitle", { name: localized(item.name) })}
    >
      <div className="grid grid-cols-2 gap-3">
        <label className="flex flex-col gap-1 text-[13px] font-bold">
          {t("stock.qty")}
          <Input
            value={qty}
            inputMode="numeric"
            autoFocus
            onChange={(e) => setQty(e.target.value.replace(/\D/g, ""))}
            className="h-12 rounded-[10px] bg-card text-[16px] font-normal"
          />
        </label>
        <label className="flex flex-col gap-1 text-[13px] font-bold">
          {t("stock.cost")}
          <span className="relative">
            <Input
              value={cost}
              inputMode="numeric"
              onChange={(e) => setCost(e.target.value.replace(/[^\d.,]/g, ""))}
              className="h-12 rounded-[10px] bg-card pr-9 text-[16px] font-normal"
            />
            <span className="pointer-events-none absolute right-3.5 top-1/2 -translate-y-1/2 text-sm font-normal text-muted-foreground">
              đ
            </span>
          </span>
          {item.latestUnitCost != null && (
            <span className="text-[12px] font-normal text-muted-foreground">
              {t("stock.prefilled")}
            </span>
          )}
        </label>
      </div>
      <p className="flex justify-between text-[14px] text-ink-2">
        {t("stock.after")}
        <b className="text-ink">{tf("stock.pcs", { n: item.stock + (ok ? n : 0), unit })}</b>
      </p>
      <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        <Button variant="outline" size="lg" onClick={onClose}>
          {t("stock.cancel")}
        </Button>
        <Button size="lg" disabled={!ok || restock.isPending} onClick={submit}>
          {t("stock.record")}
        </Button>
      </div>
    </ResponsiveDialog>
  );
}
