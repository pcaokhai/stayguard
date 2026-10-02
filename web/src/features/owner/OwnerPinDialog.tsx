"use client";

import { useState, type ReactNode } from "react";
import { ResponsiveDialog } from "@/components/ResponsiveDialog";
import { Button } from "@/components/ui/button";
import { t } from "@/lib/t";
import { PinInput } from "../auth/PinInput";
import { isSixDigits } from "../auth/pin";

// Sensitive change behind the owner's PIN (bank accounts, docs/15 rule 13). The PIN is sent once, never kept.
export function OwnerPinDialog({
  title,
  body,
  confirmLabel,
  tone = "primary",
  pending,
  error,
  shake,
  onConfirm,
  onClose,
}: {
  title: string;
  body: ReactNode;
  confirmLabel: string;
  tone?: "primary" | "danger";
  pending: boolean;
  error: string;
  shake: number;
  onConfirm: (pin: string) => void;
  onClose: () => void;
}) {
  const [pin, setPin] = useState("");
  const submit = () => isSixDigits(pin) && onConfirm(pin);
  return (
    <ResponsiveDialog open onOpenChange={(o) => !o && onClose()} title={title}>
      <div className="text-[14px] text-ink-2">{body}</div>
      <label className="flex flex-col gap-1.5 text-[13px] font-bold">
        {t("property.ownerPin")}
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
        size="lg"
        variant={tone === "danger" ? "outline" : "default"}
        className={
          tone === "danger"
            ? "border-destructive/40 font-bold text-destructive hover:bg-destructive/10"
            : undefined
        }
        disabled={!isSixDigits(pin) || pending}
        onClick={submit}
      >
        {confirmLabel}
      </Button>
      <Button variant="outline" size="lg" onClick={onClose}>
        {t("property.cancel")}
      </Button>
    </ResponsiveDialog>
  );
}
