"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { Info } from "lucide-react";
import { useState } from "react";
import { useForm, useWatch } from "react-hook-form";
import { toast } from "sonner";
import { z } from "zod";
import { Shake, SlidingPill } from "@/components/motion";
import { Button } from "@/components/ui/button";
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
import { t, type MessageKey } from "../../lib/t";
import { today } from "../history/dates";
import { useCreateLeave, type ShiftCode } from "./hooks";

const KINDS = ["PAID", "SICK", "UNPAID"] as const;
const chip =
  "h-10 rounded-full! border bg-card px-4 font-bold data-[state=on]:border-transparent data-[state=on]:bg-primary data-[state=on]:text-primary-foreground";

const schema = z
  .object({
    fromDate: z.string().min(1, "leave.dateRequired"),
    toDate: z.string(),
    span: z.enum(["SHIFT", "DAY", "RANGE"]),
    kind: z.enum(KINDS),
    reason: z.string().trim().max(300),
  })
  .refine((v) => v.span !== "RANGE" || v.toDate >= v.fromDate, {
    path: ["toDate"],
    message: "leave.toBefore",
  });
type Values = z.infer<typeof schema>;

// Boards P52 and the PC panel. `shift` is the caller's rostered shift for the chosen day when known;
// the cover colleague is not offered because staff lists are owner-only.
export function LeaveForm({
  shiftFor,
  onDone,
}: {
  shiftFor?: (iso: string) => ShiftCode | undefined;
  onDone?: () => void;
}) {
  const create = useCreateLeave();
  const [key] = useState(newIdempotencyKey);
  const [tries, setTries] = useState(0);
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { fromDate: today(), toDate: today(), span: "DAY", kind: "PAID", reason: "" },
  });
  const span = useWatch({ control: form.control, name: "span" });

  const submit = form.handleSubmit(
    (v) =>
      create.mutate(
        {
          key,
          body: {
            fromDate: v.fromDate,
            toDate: v.span === "RANGE" ? v.toDate : v.fromDate,
            shift: v.span === "SHIFT" ? (shiftFor?.(v.fromDate) ?? null) : null,
            kind: v.kind,
            reason: v.reason || null,
          },
        },
        {
          onSuccess: () => {
            toast.success(t("leave.formSent"));
            onDone?.();
          },
        },
      ),
    () => setTries((n) => n + 1),
  );

  return (
    <Form {...form}>
      <form onSubmit={submit} noValidate className="flex flex-1 flex-col gap-4">
        <FormField
          control={form.control}
          name="fromDate"
          render={({ field, fieldState }) => (
            <FormItem>
              <FormLabel className="font-bold">{t("leave.date")}</FormLabel>
              <Shake trigger={fieldState.error ? tries : 0}>
                <FormControl>
                  <Input
                    type="date"
                    min={today()}
                    className="h-12 rounded-[10px] bg-card px-4 text-base"
                    {...field}
                  />
                </FormControl>
              </Shake>
              <FormMessage>
                {fieldState.error && t(fieldState.error.message as "leave.dateRequired")}
              </FormMessage>
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name="span"
          render={({ field }) => (
            <FormItem>
              <FormLabel className="font-bold">{t("leave.timeOff")}</FormLabel>
              <ToggleGroup
                type="single"
                value={field.value}
                onValueChange={(v) => v && field.onChange(v)}
                className="w-full gap-0 rounded-xl bg-secondary p-1"
              >
                {(
                  [
                    ["SHIFT", "leave.oneShift"],
                    ["DAY", "leave.whole"],
                    ["RANGE", "leave.several"],
                  ] as const
                ).map(([v, label]) => (
                  <ToggleGroupItem
                    key={v}
                    value={v}
                    className="relative h-10 flex-1 rounded-[9px]! px-2 text-sm font-bold data-[state=on]:bg-transparent"
                  >
                    {field.value === v && (
                      <SlidingPill
                        id="leave-span"
                        className="absolute inset-0 rounded-[9px] bg-card shadow-sm"
                      />
                    )}
                    <span className="relative">{t(label)}</span>
                  </ToggleGroupItem>
                ))}
              </ToggleGroup>
            </FormItem>
          )}
        />
        {span === "RANGE" && (
          <FormField
            control={form.control}
            name="toDate"
            render={({ field, fieldState }) => (
              <FormItem>
                <FormLabel className="font-bold">{t("leave.toDate")}</FormLabel>
                <FormControl>
                  <Input
                    type="date"
                    min={form.getValues("fromDate")}
                    className="h-12 rounded-[10px] bg-card px-4 text-base"
                    {...field}
                  />
                </FormControl>
                <FormMessage>
                  {fieldState.error && t(fieldState.error.message as "leave.toBefore")}
                </FormMessage>
              </FormItem>
            )}
          />
        )}
        <FormField
          control={form.control}
          name="kind"
          render={({ field }) => (
            <FormItem>
              <FormLabel className="font-bold">{t("leave.type")}</FormLabel>
              <ToggleGroup
                type="single"
                value={field.value}
                onValueChange={(v) => v && field.onChange(v)}
                className="flex flex-wrap justify-start gap-2"
              >
                {KINDS.map((k) => (
                  <ToggleGroupItem key={k} value={k} className={chip}>
                    {t(`leave.${k}` as MessageKey)}
                  </ToggleGroupItem>
                ))}
              </ToggleGroup>
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name="reason"
          render={({ field }) => (
            <FormItem>
              <FormLabel className="font-bold">{t("leave.reason")}</FormLabel>
              <FormControl>
                <Textarea
                  rows={3}
                  maxLength={300}
                  className="rounded-[10px] bg-card px-4 text-base"
                  {...field}
                />
              </FormControl>
            </FormItem>
          )}
        />
        <p className="flex items-start gap-2.5 rounded-xl border border-info-line bg-info-bg p-3.5 text-[13px] text-info">
          <Info className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
          {t("leave.notify")}
        </p>
        {create.isError && (
          <p role="alert" className="text-sm font-semibold text-warn">
            {t("leave.formFailed")}
          </p>
        )}
        <Button type="submit" size="lg" className="mt-auto" loading={create.isPending}>
          {t("leave.send")}
        </Button>
      </form>
    </Form>
  );
}
