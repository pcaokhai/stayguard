"use client";

import { Minus, Plus } from "lucide-react";
import { useState } from "react";
import type { components } from "../../api/generated/schema";
import { ResponsiveDialog } from "@/components/ResponsiveDialog";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { newIdempotencyKey } from "../../lib/api";
import { localized } from "../../lib/locale";
import { formatVnd } from "../../lib/money";
import { t, tf } from "../../lib/t";
import { useAddExtras, useServices } from "./hooks";

type ExtraLine = components["schemas"]["ExtraLine"];

export function ExtrasSheet({
  stayId,
  roomCode,
  extras = [],
  onClose,
}: {
  stayId: string;
  roomCode: string;
  extras?: ExtraLine[];
  onClose: () => void;
}) {
  const services = useServices();
  const add = useAddExtras(stayId);
  const [qty, setQty] = useState<Record<string, number>>({});
  const [key] = useState(newIdempotencyKey);
  const items = Object.entries(qty)
    .filter(([, n]) => n > 0)
    .map(([serviceCode, quantity]) => ({ serviceCode, quantity }));
  // List price x quantity of what is picked, shown on the button; the server prices the stay.
  const picked = (services.data ?? []).reduce((sum, s) => sum + s.price * (qty[s.code] ?? 0), 0);
  const step = (code: string, d: number, stock: number) =>
    setQty((q) => ({ ...q, [code]: Math.min(stock, Math.max(0, (q[code] ?? 0) + d)) }));
  const has = extras.map((x) => `${localized(x.name)} × ${x.quantity}`).join(", ");

  return (
    <ResponsiveDialog
      open
      onOpenChange={(o) => !o && onClose()}
      title={t("extras.title")}
      description={`${t("stay.roomTitle")} ${roomCode}${has ? ` · ${t("extras.alreadyHas")} ${has}` : ""}`}
    >
      <ul className="flex flex-col gap-2">
        {services.data?.map((s) => (
          <li key={s.code}>
            <Card
              className={`flex-row items-center justify-between gap-3 p-3.5 shadow-none ${(qty[s.code] ?? 0) > 0 ? "border-info-line" : ""}`}
            >
              <div>
                <p className="font-semibold">{localized(s.name)}</p>
                <p className="text-[13px] text-muted-foreground">
                  {tf("extras.stock", { price: formatVnd(s.price), n: s.stock })}
                </p>
              </div>
              <div className="flex items-center gap-3">
                <Button
                  type="button"
                  variant="outline"
                  size="icon"
                  aria-label={t("extras.less")}
                  onClick={() => step(s.code, -1, s.stock)}
                >
                  <Minus />
                </Button>
                {/* Quantities update without animation (docs/16 §5). */}
                <span className="w-5 text-center text-lg font-bold">{qty[s.code] ?? 0}</span>
                <Button
                  type="button"
                  variant="secondary"
                  size="icon"
                  aria-label={t("extras.more")}
                  onClick={() => step(s.code, 1, s.stock)}
                >
                  <Plus />
                </Button>
              </div>
            </Card>
          </li>
        ))}
      </ul>
      <p className="text-[13px] text-muted-foreground">{t("extras.note")}</p>
      {add.isError && (
        <p role="alert" className="text-sm text-warn">
          {t("extras.failed")}
        </p>
      )}
      <Button
        type="button"
        size="lg"
        loading={add.isPending}
        disabled={!items.length}
        onClick={() => add.mutate({ key, body: { items } }, { onSuccess: onClose })}
      >
        {items.length ? tf("extras.addTo", { amount: formatVnd(picked) }) : t("extras.add")}
      </Button>
    </ResponsiveDialog>
  );
}
