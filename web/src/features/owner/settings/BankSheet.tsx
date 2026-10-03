"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useState } from "react";
import { Controller, useForm, type Resolver } from "react-hook-form";
import { toast } from "sonner";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { Form, FormItem, FormLabel, FormMessage } from "@/components/ui/form";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { newIdempotencyKey } from "@/lib/api";
import { t, type MessageKey } from "@/lib/t";
import { PinInput } from "../../auth/PinInput";
import { FormSheet, inputClass, SwitchField, TextField } from "../FormFields";
import { messageFor } from "../../problem/problem";
import { useCreateBank } from "./hooks";

// ponytail: the common Vietnamese banks by VietQR BIN; add rows here when an owner banks elsewhere.
export const BANKS = [
  { bin: "970436", name: "Vietcombank" },
  { bin: "970415", name: "VietinBank" },
  { bin: "970418", name: "BIDV" },
  { bin: "970405", name: "Agribank" },
  { bin: "970407", name: "Techcombank" },
  { bin: "970422", name: "MB Bank" },
  { bin: "970416", name: "ACB" },
  { bin: "970432", name: "VPBank" },
  { bin: "970423", name: "TPBank" },
  { bin: "970403", name: "Sacombank" },
];

const schema = z.object({
  bankBin: z.string().min(1, "property.required"),
  accountNo: z
    .string()
    .trim()
    .regex(/^[0-9]{6,20}$/, "property.accountRule"),
  accountName: z.string().trim().min(1, "property.required").max(60),
  makeDefaultWhenConnected: z.boolean(),
  ownerPin: z.string().regex(/^[0-9]{6}$/, "property.required"),
});
type Values = z.infer<typeof schema>;

export function BankSheet({ onClose }: { onClose: () => void }) {
  const create = useCreateBank();
  const [key] = useState(newIdempotencyKey);
  const [shake, setShake] = useState(0);
  const form = useForm<Values>({
    resolver: zodResolver(schema) as Resolver<Values>,
    defaultValues: {
      bankBin: BANKS[0].bin,
      accountNo: "",
      accountName: "",
      makeDefaultWhenConnected: false,
      ownerPin: "",
    },
  });
  const submit = form.handleSubmit((v) =>
    create.mutate(
      { key, body: { ...v, accountName: v.accountName.toUpperCase() } },
      {
        onSuccess: () => {
          toast.success(t("property.added"));
          onClose();
        },
        onError: (e) => {
          form.setValue("ownerPin", "");
          setShake((n) => n + 1);
          toast.error(
            messageFor(e, {
              OWNER_PIN_INVALID: "property.wrongPin",
              CONFLICT: "property.duplicate",
              VALIDATION_FAILED: "property.actionFailed",
            }),
          );
        },
      },
    ),
  );
  return (
    <FormSheet open onClose={onClose} title={t("property.addTitle")}>
      <Form {...form}>
        <form onSubmit={submit} noValidate className="flex flex-col gap-3 px-4 pb-6">
          <Controller
            control={form.control}
            name="bankBin"
            render={({ field }) => (
              <FormItem>
                <FormLabel className="text-[13px] font-bold">{t("property.bank")}</FormLabel>
                <Select value={field.value} onValueChange={field.onChange}>
                  <SelectTrigger className={`${inputClass} w-full`} aria-label={t("property.bank")}>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {BANKS.map((b) => (
                      <SelectItem key={b.bin} value={b.bin}>
                        {b.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </FormItem>
            )}
          />
          <TextField
            control={form.control}
            name="accountNo"
            label={t("property.accountNo")}
            inputMode="numeric"
          />
          <TextField
            control={form.control}
            name="accountName"
            label={t("property.accountName")}
            hint={t("property.accountNameHint")}
          />
          <SwitchField
            control={form.control}
            name="makeDefaultWhenConnected"
            label={t("property.makeDefaultWhen")}
          />
          <Controller
            control={form.control}
            name="ownerPin"
            render={({ field, fieldState }) => (
              <FormItem>
                <FormLabel className="text-[13px] font-bold">{t("property.ownerPin")}</FormLabel>
                <PinInput
                  value={field.value}
                  onChange={field.onChange}
                  shake={shake}
                  invalid={!!fieldState.error}
                />
                <FormMessage>
                  {fieldState.error?.message ? t(fieldState.error.message as MessageKey) : null}
                </FormMessage>
              </FormItem>
            )}
          />
          <p className="rounded-card border border-info-line bg-info-bg p-3 text-[13px] text-info">
            {t("property.addInfo")}
          </p>
          <div className="mt-1 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            <Button type="button" variant="outline" size="lg" onClick={onClose}>
              {t("property.cancel")}
            </Button>
            <Button type="submit" size="lg" disabled={create.isPending}>
              {t("property.addSubmit")}
            </Button>
          </div>
        </form>
      </Form>
    </FormSheet>
  );
}
