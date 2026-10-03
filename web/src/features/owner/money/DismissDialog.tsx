"use client";

import { useRef, useState } from "react";
import { toast } from "sonner";
import { ResponsiveDialog } from "@/components/ResponsiveDialog";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { newIdempotencyKey } from "@/lib/api";
import { formatVnd } from "@/lib/money";
import { t, tf } from "@/lib/t";
import { dismissErrorMessage, noteProblem, NOTE_MAX } from "./dismiss";
import { useDismissTransfer } from "./hooks";
import type { Tx } from "./rows";

// "Không phải tiền phòng": the owner closes an unmatched transfer with a required note (max 500).
export function DismissDialog({ tx, onClose }: { tx: Tx; onClose: () => void }) {
  const dismiss = useDismissTransfer();
  const [note, setNote] = useState("");
  const [touched, setTouched] = useState(false);
  const key = useRef(newIdempotencyKey()); // one key per dialog, reused when a failed send is retried
  const problem = noteProblem(note);
  const shown =
    touched && problem
      ? t(problem === "required" ? "money.dismissRequired" : "money.dismissTooLong")
      : "";

  const submit = () => {
    setTouched(true);
    if (problem || !tx.paymentEventId) return;
    dismiss.mutate(
      { eventId: tx.paymentEventId, note, key: key.current },
      {
        onSuccess: () => {
          toast.success(t("money.dismissed"));
          onClose();
        },
        onError: (e) => toast.error(dismissErrorMessage(e)),
      },
    );
  };

  return (
    <ResponsiveDialog
      open
      onOpenChange={(o) => !o && onClose()}
      title={t("money.dismissTitle")}
      description={tf("money.dismissBody", { amount: formatVnd(tx.amount) })}
    >
      <label className="flex flex-col gap-1.5 text-[13px] font-bold">
        {t("money.dismissNote")}
        <Textarea
          value={note}
          onChange={(e) => setNote(e.target.value)}
          onBlur={() => setTouched(true)}
          placeholder={t("money.dismissHint")}
          aria-invalid={!!shown}
          rows={3}
          className="min-h-24 rounded-[10px] bg-card text-[15px] font-normal"
        />
      </label>
      <p className="flex justify-between text-[13px]">
        <span role="alert" className="font-bold text-destructive">
          {shown}
        </span>
        <span className="text-muted-foreground">
          {note.trim().length}/{NOTE_MAX}
        </span>
      </p>
      <Button size="lg" disabled={dismiss.isPending} onClick={submit} className="font-bold">
        {t("money.dismissConfirm")}
      </Button>
      <Button variant="outline" size="lg" onClick={onClose}>
        {t("property.cancel")}
      </Button>
    </ResponsiveDialog>
  );
}
