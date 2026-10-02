"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { Info } from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { toast } from "sonner";
import { z } from "zod";
import { Shake, SlidingPill } from "@/components/motion";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { Button } from "@/components/ui/button";
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { Textarea } from "@/components/ui/textarea";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { newIdempotencyKey } from "../../lib/api";
import { lp } from "../../lib/locale";
import { t, tf, type MessageKey } from "../../lib/t";
import { useRoom } from "../stay/hooks";
import { useReportDamage } from "./hooks";

const CATEGORIES = [
  "AIR_CONDITIONER",
  "HOT_WATER",
  "PLUMBING",
  "POWER_LIGHTS",
  "TV",
  "DOOR_LOCK",
  "MISSING_ITEMS",
  "OTHER",
] as const;
const chip =
  "h-10 rounded-full! border bg-card px-4 font-bold data-[state=on]:border-transparent data-[state=on]:bg-primary data-[state=on]:text-primary-foreground";

const schema = z.object({
  category: z.enum(CATEGORIES),
  description: z.string().trim().min(1, "damage.descRequired").max(500),
  severity: z.enum(["STILL_RENTABLE", "LOCK_ROOM"]),
});
type Values = z.infer<typeof schema>;

// Board P49. Photos arrive with the upload API; the ticket itself is created server-side.
export function DamageView() {
  const params = useSearchParams();
  const roomId = params.get("room") ?? "";
  const router = useRouter();
  const room = useRoom(roomId);
  const report = useReportDamage(roomId);
  const [key] = useState(newIdempotencyKey);
  const [tries, setTries] = useState(0);
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      category: "HOT_WATER",
      description: "",
      severity: params.get("lock") ? "LOCK_ROOM" : "STILL_RENTABLE",
    },
  });

  const submit = form.handleSubmit(
    (v) =>
      report.mutate(
        { key, body: v },
        {
          onSuccess: () => {
            toast.success(t("damage.sent"));
            router.back();
          },
        },
      ),
    () => setTries((n) => n + 1),
  );

  return (
    <AppFrame tabs={false}>
      <main className="mx-auto flex w-full max-w-[480px] flex-1 flex-col">
        <TopBar
          title={t("damage.title")}
          subtitle={room.data ? tf("damage.room", { code: room.data.code }) : undefined}
          back="/rooms"
        />
        <Form {...form}>
          <form onSubmit={submit} noValidate className="flex flex-1 flex-col gap-4 px-5 pb-8">
            <FormField
              control={form.control}
              name="category"
              render={({ field }) => (
                <FormItem>
                  <FormLabel className="font-bold">{t("damage.what")}</FormLabel>
                  <ToggleGroup
                    type="single"
                    value={field.value}
                    onValueChange={(v) => v && field.onChange(v)}
                    className="flex flex-wrap justify-start gap-2"
                  >
                    {CATEGORIES.map((c) => (
                      <ToggleGroupItem key={c} value={c} className={chip}>
                        {t(`damage.${c}` as MessageKey)}
                      </ToggleGroupItem>
                    ))}
                  </ToggleGroup>
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="description"
              render={({ field, fieldState }) => (
                <FormItem>
                  <FormLabel className="font-bold">{t("damage.desc")}</FormLabel>
                  <Shake trigger={fieldState.error ? tries : 0}>
                    <FormControl>
                      <Textarea
                        rows={4}
                        maxLength={500}
                        className="rounded-[10px] bg-card px-4 text-base"
                        {...field}
                      />
                    </FormControl>
                  </Shake>
                  <FormMessage>
                    {fieldState.error && t(fieldState.error.message as "damage.descRequired")}
                  </FormMessage>
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="severity"
              render={({ field }) => (
                <FormItem>
                  <FormLabel className="font-bold">{t("damage.severity")}</FormLabel>
                  <ToggleGroup
                    type="single"
                    value={field.value}
                    onValueChange={(v) => v && field.onChange(v)}
                    className="w-full gap-0 rounded-xl bg-secondary p-1"
                  >
                    {(["STILL_RENTABLE", "LOCK_ROOM"] as const).map((s) => (
                      <ToggleGroupItem
                        key={s}
                        value={s}
                        className="relative h-11 flex-1 rounded-[9px]! text-sm font-bold data-[state=on]:bg-transparent"
                      >
                        {field.value === s && (
                          <SlidingPill
                            id="damage-severity"
                            className="absolute inset-0 rounded-[9px] bg-card shadow-sm"
                          />
                        )}
                        <span className="relative">
                          {t(s === "LOCK_ROOM" ? "damage.lock" : "damage.rentable")}
                        </span>
                      </ToggleGroupItem>
                    ))}
                  </ToggleGroup>
                </FormItem>
              )}
            />
            <p className="flex items-start gap-2.5 rounded-xl border border-info-line bg-info-bg p-3.5 text-[13px] text-info">
              <Info className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
              {t("damage.info")}
            </p>
            {report.isError && (
              <p role="alert" className="text-sm font-semibold text-warn">
                {report.error.message.includes(" ") ? report.error.message : t("damage.failed")}
              </p>
            )}
            <Button type="submit" size="lg" className="mt-auto" loading={report.isPending}>
              {t("damage.send")}
            </Button>
          </form>
        </Form>
      </main>
    </AppFrame>
  );
}
