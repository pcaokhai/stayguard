"use client";

import { Info } from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";
import { useState } from "react";
import { toast } from "sonner";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { newIdempotencyKey } from "../../lib/api";
import { t, type MessageKey } from "../../lib/t";
import { useBuildings, useRooms } from "../rooms/hooks";
import { useReportUsage } from "./hooks";

const SIGNS = ["bed", "towels", "trash", "other"] as const;
const chip =
  "h-10 rounded-full! border bg-card px-4 font-bold data-[state=on]:border-transparent data-[state=on]:bg-primary data-[state=on]:text-primary-foreground";

// Board P27: a room that looks used but has no open stay; the server raises the owner alert.
export function UsedRoomView() {
  const router = useRouter();
  const preset = useSearchParams().get("room");
  const buildings = useBuildings().data?.filter((b) => b.level === "EDIT");
  const [roomId, setRoomId] = useState(preset ?? "");
  const [signs, setSigns] = useState<string[]>([]);
  const [note, setNote] = useState("");
  const [key] = useState(newIdempotencyKey);
  const report = useReportUsage(roomId);
  // Rooms without an open stay in the buildings the caller can edit (one query per building).
  const first = useRooms(buildings?.[0]?.id);
  const second = useRooms(buildings?.[1]?.id);
  const rooms = [...(first.data ?? []), ...(second.data ?? [])].filter((r) => !r.activeStay);

  const submit = () =>
    report.mutate(
      {
        key,
        note: [signs.map((s) => t(`used.${s}` as MessageKey)).join(", "), note.trim()]
          .filter(Boolean)
          .join(" · "),
      },
      {
        onSuccess: () => {
          toast.success(t("used.sent"));
          router.back();
        },
      },
    );

  return (
    <AppFrame tabs={false}>
      <main className="mx-auto flex w-full max-w-[480px] flex-1 flex-col">
        <TopBar title={t("used.title")} subtitle={t("used.sub")} back="/housekeeping" />
        <div className="flex flex-1 flex-col gap-4 px-5 pb-8">
          <div className="flex flex-col gap-2">
            <Label className="font-bold">{t("used.room")}</Label>
            <ToggleGroup
              type="single"
              value={roomId}
              onValueChange={setRoomId}
              className="flex flex-wrap justify-start gap-2"
            >
              {rooms.map((r) => (
                <ToggleGroupItem key={r.id} value={r.id} className={chip}>
                  {r.code}
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
          </div>
          <div className="flex flex-col gap-2">
            <Label className="font-bold">{t("used.signs")}</Label>
            <ToggleGroup
              type="multiple"
              value={signs}
              onValueChange={setSigns}
              className="flex flex-wrap justify-start gap-2"
            >
              {SIGNS.map((s) => (
                <ToggleGroupItem key={s} value={s} className={chip}>
                  {t(`used.${s}` as MessageKey)}
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
          </div>
          <label className="flex flex-col gap-1.5 text-sm font-bold">
            {t("used.note")}
            <Textarea
              rows={3}
              maxLength={400}
              value={note}
              onChange={(e) => setNote(e.target.value)}
              className="rounded-[10px] bg-card px-4 text-base font-normal"
            />
          </label>
          <p className="flex items-start gap-2.5 rounded-xl border border-info-line bg-info-bg p-3.5 text-[13px] text-info">
            <Info className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
            {t("used.info")}
          </p>
          {report.isError && (
            <p role="alert" className="text-sm font-semibold text-warn">
              {t("used.failed")}
            </p>
          )}
          <Button
            size="lg"
            className="mt-auto"
            disabled={!roomId}
            loading={report.isPending}
            onClick={submit}
          >
            {roomId ? t("used.send") : t("used.pickRoom")}
          </Button>
        </div>
      </main>
    </AppFrame>
  );
}
