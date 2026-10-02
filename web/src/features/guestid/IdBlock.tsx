"use client";

import { Check, Plus } from "lucide-react";
import { useRef } from "react";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import { t } from "../../lib/t";
import type { Side } from "./hooks";

export type IdPhotos = Record<Side, File | null>;
const MAX_BYTES = 5 * 1024 * 1024;
export const photoProblem = (f: File) =>
  !["image/jpeg", "image/png"].includes(f.type)
    ? "guestId.unsupported"
    : f.size > MAX_BYTES
      ? "guestId.tooLarge"
      : null;

// Check-in side of the guest ID (docs/15 rules 21-25): optional, collected for the stay declaration, write-only.
// The photos stay in memory until the stay exists; nothing is previewed or stored on this device.
export function IdBlock({
  photos,
  onPhoto,
}: {
  photos: IdPhotos;
  onPhoto: (side: Side, file: File | null) => void;
}) {
  return (
    <Card className="gap-3 p-4 shadow-none">
      <div className="flex items-center justify-between gap-3">
        <h3 className="font-bold">{t("guestId.title")}</h3>
        <Badge variant="maintenance" className="px-2.5 py-1 font-bold">
          {t("guestId.ownerOnly")}
        </Badge>
      </div>
      <div className="grid grid-cols-2 gap-2.5">
        <Tile side="FRONT" label={t("guestId.front")} file={photos.FRONT} onPhoto={onPhoto} />
        <Tile side="BACK" label={t("guestId.back")} file={photos.BACK} onPhoto={onPhoto} />
      </div>
      <p className="text-[13px] text-muted-foreground">{t("guestId.notice")}</p>
    </Card>
  );
}

function Tile({
  side,
  label,
  file,
  onPhoto,
}: {
  side: Side;
  label: string;
  file: File | null;
  onPhoto: IdBlockProps["onPhoto"];
}) {
  const input = useRef<HTMLInputElement>(null);
  const bad = file && photoProblem(file);
  return (
    <>
      <input
        ref={input}
        type="file"
        accept="image/jpeg,image/png"
        capture="environment"
        hidden
        onChange={(e) => {
          const f = e.target.files?.[0] ?? null;
          onPhoto(side, f);
          e.target.value = "";
        }}
      />
      <button
        type="button"
        onClick={() => input.current?.click()}
        className={cn(
          "flex min-h-[92px] flex-col items-center justify-center gap-1 rounded-xl border p-2 text-center transition-colors",
          file && !bad ? "border-ok-line bg-ok-bg text-ok" : "border-dashed bg-card",
          bad && "border-destructive text-destructive",
        )}
      >
        {file && !bad ? (
          <Check className="size-4" aria-hidden="true" />
        ) : (
          <Plus className="size-4" aria-hidden="true" />
        )}
        <b className="text-sm">{label}</b>
        <span className="text-xs text-muted-foreground">
          {bad ? t(bad as "guestId.tooLarge") : file ? t("guestId.captured") : t("guestId.take")}
        </span>
      </button>
    </>
  );
}
type IdBlockProps = Parameters<typeof IdBlock>[0];
