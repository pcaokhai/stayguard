"use client";

import { useState } from "react";
import { Trash2 } from "lucide-react";
import { toast } from "sonner";
import { ResponsiveDialog } from "@/components/ResponsiveDialog";
import { Button } from "@/components/ui/button";
import { newIdempotencyKey } from "@/lib/api";
import { localized } from "@/lib/locale";
import { t, tf } from "@/lib/t";
import { useRemoveItem, type Service } from "./hooks";

// The server decides: items with sales history are never deleted, they stop selling (docs/15).
export function RemoveItemDialog({
  item,
  onClose,
  onDone,
}: {
  item: Service;
  onClose: () => void;
  onDone?: () => void;
}) {
  const remove = useRemoveItem();
  const [key] = useState(newIdempotencyKey);
  const sold = item.soldLast7Days ?? 0;
  return (
    <ResponsiveDialog
      open
      onOpenChange={(o) => !o && onClose()}
      title={tf("stock.removeTitle", { name: localized(item.name) })}
    >
      <p className="text-[14px] text-ink-2">
        {sold > 0 ? tf("stock.removeSold", { n: sold }) : t("stock.removeFresh")}
      </p>
      <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        <Button variant="outline" size="lg" onClick={onClose}>
          {t("stock.cancel")}
        </Button>
        <Button
          variant="outline"
          size="lg"
          className="border-destructive/40 font-bold text-destructive hover:bg-destructive/10"
          disabled={remove.isPending}
          onClick={() =>
            remove.mutate(
              { code: item.code, key },
              {
                onSuccess: (result) => {
                  toast.success(result === "DELETED" ? t("stock.removed") : t("stock.stopped"));
                  onClose();
                  onDone?.();
                },
                onError: () => toast.error(t("stock.actionFailed")),
              },
            )
          }
        >
          <Trash2 aria-hidden="true" />
          {sold > 0 ? t("stock.confirmStop") : t("stock.confirmRemove")}
        </Button>
      </div>
    </ResponsiveDialog>
  );
}
