"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { Info } from "lucide-react";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { useForm, useWatch } from "react-hook-form";
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
import { lp } from "../../lib/locale";
import { formatVnd, parseVnd } from "../../lib/money";
import { t } from "../../lib/t";
import { useCurrentShift, useRecordPayout } from "./hooks";

const PURPOSES = [
  ["ice", "payout.ice"],
  ["water", "payout.water"],
  ["repair", "payout.repair"],
  ["other", "payout.other"],
] as const;

const schema = z.object({
  amount: z.string().refine((v) => parseVnd(v) > 0, "payout.amountInvalid"),
  purpose: z.enum(["ice", "water", "repair", "other"]),
  note: z.string().trim().min(3, "payout.noteShort").max(180),
});
type Values = z.infer<typeof schema>;

// Boards P24 and PC "Chi tiền". Cannot be edited or deleted after saving (server rule).
export function PayoutView() {
  const router = useRouter();
  const shift = useCurrentShift();
  const record = useRecordPayout();
  const [key] = useState(newIdempotencyKey);
  const [tries, setTries] = useState(0);
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { amount: "", purpose: "ice", note: "" },
  });
  const amount = parseVnd(useWatch({ control: form.control, name: "amount" }));
  const s = shift.data;

  if (shift.isError)
    return (
      <AppFrame tabs={false}>
        <QueryError onRetry={() => void shift.refetch()} />
      </AppFrame>
    );

  const submit = form.handleSubmit(
    (v) =>
      record.mutate(
        {
          key,
          body: {
            amount: parseVnd(v.amount),
            description: `${t(`payout.${v.purpose}`)}: ${v.note}`,
          },
        },
        {
          onSuccess: () => {
            toast.success(t("payout.saved"));
            router.replace(lp("/shift"));
          },
        },
      ),
    () => setTries((n) => n + 1),
  );

  return (
    <AppFrame tabs={false}>
      <main className="mx-auto flex w-full max-w-[480px] flex-1 flex-col">
        <TopBar title={t("payout.title")} subtitle={t("payout.sub")} back="/shift" />
        <Form {...form}>
          <form onSubmit={submit} noValidate className="flex flex-1 flex-col gap-4 px-5 pb-8">
            <FormField
              control={form.control}
              name="amount"
              render={({ field, fieldState }) => (
                <FormItem>
                  <FormLabel className="font-bold">{t("payout.amount")}</FormLabel>
                  <Shake trigger={fieldState.error ? tries : 0}>
                    <FormControl>
                      <Input
                        inputMode="numeric"
                        className="h-12 rounded-[10px] bg-card px-4 text-base"
                        {...field}
                      />
                    </FormControl>
                  </Shake>
                  <FormMessage>
                    {fieldState.error && t(fieldState.error.message as "payout.amountInvalid")}
                  </FormMessage>
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="purpose"
              render={({ field }) => (
                <FormItem>
                  <FormLabel className="font-bold">{t("payout.purpose")}</FormLabel>
                  <ToggleGroup
                    type="single"
                    value={field.value}
                    onValueChange={(v) => v && field.onChange(v)}
                    className="flex flex-wrap justify-start gap-2"
                  >
                    {PURPOSES.map(([code, label]) => (
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
                  <FormLabel className="font-bold">{t("payout.note")}</FormLabel>
                  <Shake trigger={fieldState.error ? tries : 0}>
                    <FormControl>
                      <Textarea
                        rows={3}
                        maxLength={180}
                        className="rounded-[10px] bg-card px-4 text-base"
                        {...field}
                      />
                    </FormControl>
                  </Shake>
                  <FormMessage>
                    {fieldState.error && t(fieldState.error.message as "payout.noteShort")}
                  </FormMessage>
                </FormItem>
              )}
            />
            {s && (
              <Card className="gap-2 p-4 text-sm shadow-none">
                <p className="flex justify-between">
                  <span className="text-muted-foreground">{t("payout.before")}</span>
                  <span>{formatVnd(s.expectedCash)}</span>
                </p>
                {/* Expected cash minus what is being paid out; the server keeps the real ledger. */}
                <p className="flex justify-between">
                  <span className="text-muted-foreground">{t("payout.after")}</span>
                  <b>{formatVnd(s.expectedCash - amount)}</b>
                </p>
              </Card>
            )}
            <p className="flex items-start gap-2 text-[13px] text-muted-foreground">
              <Info className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
              {t("payout.info")}
            </p>
            {record.isError && (
              <p role="alert" className="text-sm font-semibold text-warn">
                {t("payout.failed")}
              </p>
            )}
            <Button type="submit" size="lg" className="mt-auto" loading={record.isPending}>
              {t("payout.save")}
            </Button>
          </form>
        </Form>
      </main>
    </AppFrame>
  );
}
