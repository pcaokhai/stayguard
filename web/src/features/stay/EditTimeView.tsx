"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { Info } from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { toast } from "sonner";
import { z } from "zod";
import { Shake } from "@/components/motion";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { QueryError } from "@/components/StateView";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { newIdempotencyKey } from "../../lib/api";
import { localized, lp } from "../../lib/locale";
import { formatVnd } from "../../lib/money";
import { t, tf } from "../../lib/t";
import { clockLocale, formatClock } from "../../lib/time";
import { useBuildings } from "../rooms/hooks";
import { useEditCheckIn, useRoom, useStay } from "./hooks";
import { rentalLabel } from "./labels";

const REASONS = [
  ["WRONG_TIME", "editTime.wrong"],
  ["LATE_ARRIVAL", "editTime.late"],
  ["OTHER", "editTime.other"],
] as const;
const MAX_LATER_MINUTES = 60;
const p2 = (n: number) => String(n).padStart(2, "0");
const hhmm = (d: Date) => `${p2(d.getHours())}:${p2(d.getMinutes())}`;

const schema = z.object({
  time: z.string().min(1, "editTime.timeRequired"),
  reasonCode: z.enum(["WRONG_TIME", "LATE_ARRIVAL", "OTHER"]),
  note: z.string().trim().min(3, "editTime.noteShort").max(300),
});
type Values = z.infer<typeof schema>;

// Board P4. The server decides what is allowed (at most 60 minutes later, never in the future); the
// time input only bounds the choice. The new room charge comes back with the saved stay.
export function EditTimeView() {
  const id = useSearchParams().get("id");
  const router = useRouter();
  const stay = useStay(id);
  const room = useRoom(stay.data?.roomId ?? null);
  const building = useBuildings().data?.find((b) => b.id === room.data?.buildingId);
  const edit = useEditCheckIn(id ?? "");
  const [key] = useState(newIdempotencyKey);
  const [tries, setTries] = useState(0);
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { time: "", reasonCode: "WRONG_TIME", note: "" },
  });
  const s = stay.data;

  if (stay.isError)
    return (
      <AppFrame tabs={false}>
        <QueryError onRetry={() => void stay.refetch()} />
      </AppFrame>
    );

  const recorded = s ? new Date(s.checkInAt) : null;
  const serverNow = s ? new Date(s.quote.asOf) : null;
  const latest =
    recorded && serverNow
      ? new Date(Math.min(recorded.getTime() + MAX_LATER_MINUTES * 60_000, serverNow.getTime()))
      : null;
  const date = recorded?.toLocaleDateString(clockLocale(), { day: "2-digit", month: "2-digit" });

  const submit = form.handleSubmit(
    (v) => {
      if (!recorded) return;
      const [h, m] = v.time.split(":").map(Number);
      const next = new Date(recorded);
      next.setHours(h, m, 0, 0);
      edit.mutate(
        { key, body: { newCheckInAt: next.toISOString(), reasonCode: v.reasonCode, note: v.note } },
        {
          onSuccess: () => {
            toast.success(t("editTime.saved"));
            router.replace(lp(`/stay?id=${id}`));
          },
        },
      );
    },
    () => setTries((n) => n + 1),
  );

  return (
    <AppFrame tabs={false}>
      <main className="mx-auto flex w-full max-w-[480px] flex-1 flex-col">
        <TopBar
          title={`${t("editTime.title")} · ${s?.roomCode ?? ""}`}
          subtitle={
            s && room.data
              ? [building?.name, localized(room.data.unitType.name), rentalLabel(s.rentalType)]
                  .filter(Boolean)
                  .join(" · ")
              : undefined
          }
          back={`/stay?id=${id}`}
        />
        <Form {...form}>
          <form onSubmit={submit} noValidate className="flex flex-1 flex-col gap-4 px-5 pb-8">
            <Card className="gap-2 p-4 shadow-none">
              <p className="flex justify-between text-sm">
                <span className="text-muted-foreground">{t("editTime.recorded")}</span>
                <b>{s && `${formatClock(s.checkInAt)} · ${date}`}</b>
              </p>
            </Card>
            <FormField
              control={form.control}
              name="time"
              render={({ field, fieldState }) => (
                <FormItem>
                  <FormLabel className="font-bold">{t("editTime.correct")}</FormLabel>
                  <Shake trigger={fieldState.error ? tries : 0}>
                    <FormControl>
                      <Input
                        type="time"
                        min={recorded ? hhmm(recorded) : undefined}
                        max={latest ? hhmm(latest) : undefined}
                        className="h-12 rounded-[10px] bg-card px-4 text-base"
                        {...field}
                      />
                    </FormControl>
                  </Shake>
                  <p className="text-[13px] text-muted-foreground">{t("editTime.hint")}</p>
                  <FormMessage>
                    {fieldState.error && t(fieldState.error.message as "editTime.timeRequired")}
                  </FormMessage>
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="reasonCode"
              render={({ field }) => (
                <FormItem>
                  <FormLabel className="font-bold">{t("editTime.reason")}</FormLabel>
                  <ToggleGroup
                    type="single"
                    value={field.value}
                    onValueChange={(v) => v && field.onChange(v)}
                    className="flex flex-wrap justify-start gap-2"
                  >
                    {REASONS.map(([code, label]) => (
                      <ToggleGroupItem
                        key={code}
                        value={code}
                        className="h-10 rounded-full! border bg-card px-4 font-bold data-[state=on]:border-transparent data-[state=on]:bg-primary data-[state=on]:text-primary-foreground"
                      >
                        {t(label)}
                      </ToggleGroupItem>
                    ))}
                  </ToggleGroup>
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="note"
              render={({ field, fieldState }) => (
                <FormItem>
                  <FormLabel className="font-bold">{t("editTime.note")}</FormLabel>
                  <Shake trigger={fieldState.error ? tries : 0}>
                    <FormControl>
                      <Textarea
                        maxLength={300}
                        rows={3}
                        className="rounded-[10px] bg-card px-4 text-base"
                        {...field}
                      />
                    </FormControl>
                  </Shake>
                  <FormMessage>
                    {fieldState.error && t(fieldState.error.message as "editTime.noteShort")}
                  </FormMessage>
                </FormItem>
              )}
            />
            <Card className="gap-2 p-4 shadow-none">
              <h2 className="text-xs font-bold uppercase tracking-wide text-muted-foreground">
                {t("editTime.effect")}
              </h2>
              <p className="flex justify-between text-sm">
                <span className="text-muted-foreground">{t("editTime.before")}</span>
                <span>{s && formatVnd(s.quote.stayAmount)}</span>
              </p>
              <p className="text-[13px] text-muted-foreground">{t("editTime.afterSaved")}</p>
            </Card>
            <p className="flex items-start gap-2.5 rounded-xl border border-warn-line bg-warn-bg p-3.5 text-[13px] text-warn-ink">
              <Info className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
              {t("editTime.alert")}
            </p>
            {edit.isError && (
              <p role="alert" className="text-sm font-semibold text-warn">
                {t("editTime.failed")}
              </p>
            )}
            <Button type="submit" size="lg" className="mt-auto" loading={edit.isPending}>
              {t("editTime.save")}
            </Button>
          </form>
        </Form>
      </main>
    </AppFrame>
  );
}
