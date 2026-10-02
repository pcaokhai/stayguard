"use client";

import { Download, EyeOff, Search, Trash2, User } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import type { components } from "../../api/generated/schema";
import { ResponsiveDialog } from "@/components/ResponsiveDialog";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { toast } from "sonner";
import { t, tf } from "../../lib/t";
import { clockLocale, formatClock } from "../../lib/time";
import {
  fetchPhoto,
  useDeleteGuestId,
  useGuestIdRecord,
  useRevealIdNumber,
  type Side,
} from "./hooks";

type Meta = components["schemas"]["IdPhotoMeta"];
const REVEAL_SECONDS = 30;
const dayMonth = (iso: string) =>
  new Date(iso).toLocaleDateString(clockLocale(), { day: "2-digit", month: "2-digit" });

// Owner and manager only. Every reveal, view, download and delete is audited by the server; here
// nothing is cached: the number hides again after 30 s, photos load into object URLs that are
// revoked when the viewer closes.
export function GuestIdPanel({ stayId, guestName }: { stayId: string; guestName: string }) {
  const record = useGuestIdRecord(stayId);
  const reveal = useRevealIdNumber(stayId);
  const del = useDeleteGuestId(stayId);
  const [shown, setShown] = useState<string | null>(null);
  const [viewing, setViewing] = useState<Side | null>(null);
  const [confirm, setConfirm] = useState<Side | "NUMBER" | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);
  const hide = () => {
    clearTimeout(timer.current);
    setShown(null);
  };
  useEffect(() => hide, []); // the number never outlives the panel

  if (record.isError)
    return (
      <Card className="gap-2 p-5 shadow-none">
        <h2 className="text-[13px] font-bold uppercase tracking-wide text-ink-2">
          {t("guestId.panelTitle")}
        </h2>
        <p role="alert" className="text-sm text-warn">
          {t("guestId.loadFailed")}
        </p>
      </Card>
    );
  const r = record.data;
  if (!r) return <Skeleton className="h-48 rounded-card" />;

  const show = () =>
    reveal.mutate(undefined, {
      onSuccess: (n) => {
        setShown(n);
        timer.current = setTimeout(hide, REVEAL_SECONDS * 1000);
      },
      onError: () => toast.error(t("guestId.revealFailed")),
    });
  const confirmDelete = () => {
    const target = confirm;
    if (!target) return;
    (target === "NUMBER" ? del.number : del.photo).mutate(target as Side & undefined, {
      onSuccess: () => {
        toast.success(t("guestId.deleted"));
        setConfirm(null);
        setViewing(null);
        hide();
      },
      onError: () => toast.error(t("guestId.deleteFailed")),
    });
  };

  return (
    <Card className="gap-3 p-5 shadow-none">
      <h2 className="text-[13px] font-bold uppercase tracking-wide text-ink-2">
        {t("guestId.panelTitle")}
      </h2>
      <div className="flex items-center justify-between gap-3 rounded-xl bg-secondary p-3">
        <div className="min-w-0">
          <p className="text-xs text-muted-foreground">{t("guestId.number")}</p>
          <p className="truncate font-mono text-base font-bold" aria-live="polite">
            {r.indicators.hasIdNumber
              ? (shown ?? r.idNumberMasked ?? "•••")
              : t("guestId.noNumber")}
          </p>
        </div>
        {r.indicators.hasIdNumber && (
          <div className="flex gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              loading={reveal.isPending}
              onClick={shown ? hide : show}
            >
              {shown ? <EyeOff aria-hidden="true" /> : <Search aria-hidden="true" />}
              {shown ? t("guestId.hide") : t("guestId.show")}
            </Button>
            <Button
              type="button"
              variant="outline"
              size="icon-sm"
              aria-label={t("guestId.deleteNumber")}
              onClick={() => setConfirm("NUMBER")}
              className="border-destructive/30 text-destructive"
            >
              <Trash2 />
            </Button>
          </div>
        )}
      </div>
      {shown && (
        <p className="text-[13px] text-muted-foreground">
          {tf("guestId.revealNote", { s: REVEAL_SECONDS })}
        </p>
      )}
      <div className="grid grid-cols-2 gap-2.5">
        {(["FRONT", "BACK"] as const).map((side) => (
          <Thumb
            key={side}
            side={side}
            meta={side === "FRONT" ? r.front : r.back}
            onView={() => setViewing(side)}
            onDelete={() => setConfirm(side)}
            stayId={stayId}
          />
        ))}
      </div>
      <p className="text-[13px] text-muted-foreground">
        {r.deleteAfter
          ? tf("guestId.loggedDate", { date: dayMonth(r.deleteAfter) })
          : tf("guestId.logged", { days: 30 })}
      </p>
      {viewing && (
        <PhotoViewer
          stayId={stayId}
          name={guestName}
          side={viewing}
          metas={{ FRONT: r.front ?? null, BACK: r.back ?? null }}
          deleteAfter={r.deleteAfter ?? null}
          onSide={setViewing}
          onDelete={() => setConfirm(viewing)}
          onClose={() => setViewing(null)}
        />
      )}
      <ResponsiveDialog
        open={!!confirm}
        onOpenChange={(o) => !o && setConfirm(null)}
        title={t(confirm === "NUMBER" ? "guestId.confirmNumber" : "guestId.confirmPhoto")}
        description={t("guestId.confirmBody")}
      >
        <div className="grid grid-cols-2 gap-2">
          <Button variant="outline" onClick={() => setConfirm(null)}>
            {t("guestId.cancel")}
          </Button>
          <Button
            variant="destructive"
            loading={del.photo.isPending || del.number.isPending}
            onClick={confirmDelete}
          >
            {t("guestId.delete")}
          </Button>
        </div>
      </ResponsiveDialog>
    </Card>
  );
}

function Thumb({
  side,
  meta,
  onView,
  onDelete,
  stayId,
}: {
  side: Side;
  meta: Meta | null | undefined;
  onView: () => void;
  onDelete: () => void;
  stayId: string;
}) {
  const label = t(side === "FRONT" ? "guestId.photoFront" : "guestId.photoBack");
  return (
    <div className="flex flex-col gap-2 rounded-xl border p-2.5">
      {/* A placeholder: loading the real image is a logged view, so it only happens in the viewer. */}
      <div className="flex h-[78px] items-center justify-center rounded-lg bg-gradient-to-br from-info-bg to-secondary text-ink-2">
        <User className="size-6" aria-hidden="true" />
      </div>
      <b className="text-sm">{label}</b>
      {meta ? (
        <div className="flex gap-1.5">
          <Button
            type="button"
            variant="outline"
            size="icon-sm"
            aria-label={`${t("guestId.view")} ${label}`}
            onClick={onView}
          >
            <Search />
          </Button>
          <DownloadButton stayId={stayId} side={side} label={`${t("guestId.download")} ${label}`} />
          <Button
            type="button"
            variant="outline"
            size="icon-sm"
            aria-label={`${t("guestId.delete")} ${label}`}
            onClick={onDelete}
            className="border-destructive/30 text-destructive"
          >
            <Trash2 />
          </Button>
        </div>
      ) : (
        <p className="text-xs text-muted-foreground">{t("guestId.noPhoto")}</p>
      )}
    </div>
  );
}

// One audited download (download=true); the object URL lives only for the click.
function useDownload(stayId: string, side: Side) {
  const [busy, setBusy] = useState(false);
  const run = async () => {
    setBusy(true);
    try {
      const url = URL.createObjectURL(await fetchPhoto(stayId, side, true));
      const a = document.createElement("a");
      a.href = url;
      a.download = `id-${side.toLowerCase()}.jpg`;
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      toast.error(t("guestId.photoFailed"));
    } finally {
      setBusy(false);
    }
  };
  return { busy, run };
}

function DownloadButton({ stayId, side, label }: { stayId: string; side: Side; label: string }) {
  const { busy, run } = useDownload(stayId, side);
  return (
    <Button
      type="button"
      variant="outline"
      size="icon-sm"
      aria-label={label}
      loading={busy}
      onClick={run}
    >
      <Download />
    </Button>
  );
}

// Boards XemGiayTo and XemGiayToPC: bottom sheet on phones, dialog from 640 px.
function PhotoViewer({
  stayId,
  name,
  side,
  metas,
  deleteAfter,
  onSide,
  onDelete,
  onClose,
}: {
  stayId: string;
  name: string;
  side: Side;
  metas: Record<Side, Meta | null>;
  deleteAfter: string | null;
  onSide: (s: Side) => void;
  onDelete: () => void;
  onClose: () => void;
}) {
  const meta = metas[side];
  const label = t(side === "FRONT" ? "guestId.front" : "guestId.back");

  return (
    <ResponsiveDialog
      open
      onOpenChange={(o) => !o && onClose()}
      title={tf("guestId.viewerTitle", { side: label, name })}
      description={t("guestId.viewLogged")}
    >
      <ToggleGroup
        type="single"
        value={side}
        onValueChange={(v) => v && onSide(v as Side)}
        className="w-full gap-0 rounded-xl bg-secondary p-1"
      >
        {(["FRONT", "BACK"] as const).map((s) => (
          <ToggleGroupItem
            key={s}
            value={s}
            disabled={!metas[s]}
            className="h-10 flex-1 rounded-[9px]! text-sm font-bold data-[state=on]:bg-card data-[state=on]:shadow-sm"
          >
            {t(s === "FRONT" ? "guestId.photoFront" : "guestId.photoBack")}
          </ToggleGroupItem>
        ))}
      </ToggleGroup>
      <PhotoImage key={side} stayId={stayId} side={side} label={label} />
      {meta && (
        <p className="text-[13px] text-muted-foreground">
          {tf(deleteAfter ? "guestId.meta" : "guestId.metaShort", {
            time: formatClock(meta.uploadedAt),
            date: dayMonth(meta.uploadedAt),
            by: meta.uploadedBy,
            deleteDate: deleteAfter ? dayMonth(deleteAfter) : "",
          })}
        </p>
      )}
      <div className="grid grid-cols-[1fr_1fr] gap-2">
        <DownloadLabeled stayId={stayId} side={side} />
        <Button
          variant="outline"
          onClick={onDelete}
          className="border-destructive/30 text-destructive hover:text-destructive"
        >
          <Trash2 aria-hidden="true" />
          {t("guestId.delete")}
        </Button>
      </div>
      <Button onClick={onClose}>{t("guestId.close")}</Button>
      <p className="text-[13px] text-muted-foreground">{t("guestId.viewLogged")}</p>
    </ResponsiveDialog>
  );
}

function DownloadLabeled({ stayId, side }: { stayId: string; side: Side }) {
  const { busy, run } = useDownload(stayId, side);
  return (
    <Button variant="outline" loading={busy} onClick={run}>
      <Download aria-hidden="true" />
      {t("guestId.download")}
    </Button>
  );
}

// Fetches on mount (keyed by side, so switching side starts clean) and revokes the object URL on unmount.
function PhotoImage({ stayId, side, label }: { stayId: string; side: Side; label: string }) {
  const [src, setSrc] = useState<string>();
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    let url: string | undefined;
    let cancelled = false;
    fetchPhoto(stayId, side)
      .then((blob) => {
        if (cancelled) return;
        url = URL.createObjectURL(blob);
        setSrc(url);
      })
      .catch(() => !cancelled && setFailed(true));
    return () => {
      cancelled = true;
      if (url) URL.revokeObjectURL(url); // the image is gone from this device when the viewer closes
    };
  }, [stayId, side]);
  return (
    <div className="flex aspect-[4/3] items-center justify-center overflow-hidden rounded-xl bg-gradient-to-br from-info-bg to-dirty-bg">
      {src ? (
        // eslint-disable-next-line @next/next/no-img-element -- object URL, nothing to optimise
        <img src={src} alt={label} className="size-full object-contain" />
      ) : failed ? (
        <p role="alert" className="px-4 text-center text-sm text-warn">
          {t("guestId.photoFailed")}
        </p>
      ) : (
        <Skeleton className="size-full" />
      )}
    </div>
  );
}
