"use client";

import { useEffect, useState } from "react";
import { Check, Copy } from "lucide-react";
import { toast } from "sonner";
import { ResponsiveDialog } from "@/components/ResponsiveDialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { t, tf } from "@/lib/t";
import { clockOf } from "../format";
import type { OneTimePin } from "./hooks";

export type PinReveal = {
  pin: OneTimePin;
  name: string;
  username?: string | null;
  guesthouseCode?: string;
  created: boolean;
  summary?: { position: string; access?: string };
};

const p2 = (n: number) => String(n).padStart(2, "0");
// "23:59:41" until the PIN expires; display only.
function useCountdown(expiresAt: string) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, []);
  const s = Math.max(0, Math.floor((Date.parse(expiresAt) - now) / 1000));
  return `${p2(Math.floor(s / 3600))}:${p2(Math.floor((s % 3600) / 60))}:${p2(s % 60)}`;
}

// The PIN lives only in this dialog's props (shown once, 24 h); it is never stored or logged.
export function PinDialog({ reveal, onClose }: { reveal: PinReveal | null; onClose: () => void }) {
  if (!reveal) return null;
  return <Reveal reveal={reveal} onClose={onClose} />;
}

function Reveal({ reveal, onClose }: { reveal: PinReveal; onClose: () => void }) {
  const left = useCountdown(reveal.pin.expiresAt);
  const [copied, setCopied] = useState(false);
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(reveal.pin.pin);
      setCopied(true);
      toast.success(t("staff.pin.copied"));
    } catch {
      // Clipboard can be blocked; the PIN stays on screen.
    }
  };
  return (
    <ResponsiveDialog
      open
      onOpenChange={(o) => !o && onClose()}
      title={reveal.created ? t("staff.pin.created") : t("staff.pin.title")}
    >
      <div className="flex items-center gap-2">
        <p className="flex-1 text-sm text-ink-2">{t("staff.pin.hand")}</p>
        {reveal.created && <Badge variant="ok">{t("staff.pin.new")}</Badge>}
      </div>
      <ol className="grid grid-cols-6 gap-2" aria-label={t("staff.pin.title")}>
        {[...reveal.pin.pin].map((d, i) => (
          <li
            key={i}
            className="flex h-14 items-center justify-center rounded-[10px] border border-border bg-card text-[26px] font-bold"
          >
            {d}
          </li>
        ))}
      </ol>
      <dl className="grid gap-1 rounded-card border border-border bg-card p-3.5 text-[14px]">
        <div className="flex justify-between">
          <dt className="text-ink-2">{t("staff.pin.name")}</dt>
          <dd className="font-bold">{reveal.name}</dd>
        </div>
        {reveal.guesthouseCode && (
          <div className="flex justify-between">
            <dt className="text-ink-2">{t("staff.pin.code")}</dt>
            <dd className="font-bold">{reveal.guesthouseCode}</dd>
          </div>
        )}
        {reveal.username && (
          <div className="flex justify-between">
            <dt className="text-ink-2">{t("staff.pin.user")}</dt>
            <dd className="font-bold">{reveal.username}</dd>
          </div>
        )}
        <div className="flex justify-between">
          <dt className="text-ink-2">{t("staff.pin.expires")}</dt>
          <dd className="font-bold">
            {clockOf(reveal.pin.expiresAt)} · {tf("staff.pin.left", { time: left })}
          </dd>
        </div>
      </dl>
      <p className="rounded-card border border-info-line bg-info-bg p-3 text-[13px] text-info">
        {t("staff.pin.info")}
      </p>
      <Button variant="outline" size="lg" onClick={() => void copy()}>
        {copied ? <Check aria-hidden="true" /> : <Copy aria-hidden="true" />}
        {t("staff.pin.copy")}
      </Button>
      <Button size="lg" onClick={onClose}>
        {t("staff.pin.done")}
      </Button>
    </ResponsiveDialog>
  );
}
