"use client";

import { useState } from "react";
import { Trash2 } from "lucide-react";
import { toast } from "sonner";
import { ResponsiveDialog } from "@/components/ResponsiveDialog";
import { Button } from "@/components/ui/button";
import { t, tf } from "@/lib/t";
import { PinInput } from "../../auth/PinInput";
import { isSixDigits } from "../../auth/pin";
import { statusOf, useRemoveStaff, type Staff } from "./hooks";

// Removing staff needs the owner's PIN again (docs/15 rule 13); the PIN is sent once and never kept.
export function RemoveDialog({ staff, onClose }: { staff: Staff | null; onClose: () => void }) {
  const [pin, setPin] = useState("");
  const [shake, setShake] = useState(0);
  const [error, setError] = useState("");
  const remove = useRemoveStaff();
  if (!staff) return null;
  const name = staff.name;
  const submit = () => {
    if (!isSixDigits(pin)) return;
    setError("");
    remove.mutate(
      { userId: staff.id, ownerPin: pin },
      {
        onSuccess: () => {
          toast.success(tf("staff.del.removed", { name }));
          onClose();
        },
        onError: (e) => {
          setPin("");
          setShake((n) => n + 1);
          setError(
            statusOf(e) === 409
              ? tf("staff.del.shiftOpen", { name })
              : statusOf(e) === 422 || statusOf(e) === 403
                ? t("staff.del.wrongPin")
                : t("staff.actionFailed"),
          );
        },
      },
    );
  };
  return (
    <ResponsiveDialog
      open
      onOpenChange={(o) => !o && onClose()}
      title={tf("staff.del.title", { name })}
    >
      <ul className="list-disc space-y-1 pl-5 text-[14px] text-ink-2">
        <li>{t("staff.del.b1")}</li>
        <li>{tf("staff.del.b2", { name })}</li>
        <li>{tf("staff.del.b3", { name })}</li>
      </ul>
      <label className="flex flex-col gap-1.5 text-[13px] font-bold">
        {t("staff.del.pin")}
        <PinInput
          value={pin}
          onChange={setPin}
          shake={shake}
          invalid={!!error}
          onComplete={submit}
        />
      </label>
      {error && (
        <p role="alert" className="text-[13px] font-bold text-destructive">
          {error}
        </p>
      )}
      <Button
        variant="outline"
        size="lg"
        className="border-destructive/40 font-bold text-destructive hover:bg-destructive/10"
        disabled={!isSixDigits(pin) || remove.isPending}
        onClick={submit}
      >
        <Trash2 aria-hidden="true" />
        {t("staff.del.confirm")}
      </Button>
      <Button variant="outline" size="lg" onClick={onClose}>
        {t("staff.del.cancel")}
      </Button>
    </ResponsiveDialog>
  );
}
