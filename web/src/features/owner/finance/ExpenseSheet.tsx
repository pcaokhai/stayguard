"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useState } from "react";
import { Controller, useForm, type Resolver } from "react-hook-form";
import { toast } from "sonner";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { Form, FormItem, FormLabel } from "@/components/ui/form";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { newIdempotencyKey } from "@/lib/api";
import { parseVnd, vndNumber } from "@/lib/money";
import { t, type MessageKey } from "@/lib/t";
import { FormSheet, inputClass, SwitchField, TextField } from "../FormFields";
import {
  type Category,
  type Expense,
  useCreateExpense,
  useDeleteExpense,
  useUpdateExpense,
} from "./hooks";

export const CATEGORIES: Category[] = [
  "STAFF_PAY",
  "RENT",
  "ELECTRICITY",
  "WATER",
  "LAUNDRY",
  "MAINTENANCE",
  "SUPPLIES",
  "COST_OF_GOODS",
  "TAX_FEES",
  "INTERNET_TV",
  "PAYMENT_FEES",
  "OTHER",
];
// Pay, repairs and stock lines are posted by their own flows; the owner edits the rest.
export const AUTO = new Set<string>(["PAYROLL", "MAINTENANCE", "STOCK"]);

const schema = z.object({
  category: z.string().min(1),
  amount: z.string().trim().regex(/\d/, "expense.required"),
  month: z.string().regex(/^\d{4}-(0[1-9]|1[0-2])$/, "expense.required"),
  paidOn: z.string(),
  note: z.string().trim().max(300),
  recurring: z.boolean(),
});
type Values = z.infer<typeof schema>;

// Add (no `expense`) or edit one manual expense (boards ThemChiPhi, ThemChiPhiPC).
export function ExpenseSheet({
  month,
  expense,
  onClose,
}: {
  month: string;
  expense?: Expense;
  onClose: () => void;
}) {
  const create = useCreateExpense();
  const update = useUpdateExpense();
  const remove = useDeleteExpense();
  const [key] = useState(newIdempotencyKey);
  const auto = !!expense && AUTO.has(expense.source);
  const form = useForm<Values>({
    resolver: zodResolver(schema) as Resolver<Values>,
    defaultValues: {
      category: expense?.category ?? "ELECTRICITY",
      amount: vndNumber(expense?.amount ?? 0) === "0" ? "" : vndNumber(expense?.amount ?? 0),
      month: expense?.month ?? month,
      paidOn: expense?.paidOn ?? "",
      note: expense?.note ?? "",
      recurring: expense?.recurring ?? false,
    },
  });
  const done = (msg: string) => ({
    onSuccess: () => {
      toast.success(msg);
      onClose();
    },
    onError: () => toast.error(t("expense.saveFailed")),
  });
  const submit = form.handleSubmit((v) => {
    const body = {
      category: v.category as Category,
      amount: parseVnd(v.amount),
      month: v.month,
      paidOn: v.paidOn || null,
      note: v.note || null,
      recurring: v.recurring,
    };
    if (expense) update.mutate({ id: expense.id, body }, done(t("expense.saved")));
    else create.mutate({ key, body }, done(t("expense.saved")));
  });
  return (
    <FormSheet
      open
      onClose={onClose}
      title={expense ? t("expense.editTitle") : t("expense.addTitle")}
    >
      <Form {...form}>
        <form onSubmit={submit} noValidate className="flex flex-col gap-3 px-4 pb-6">
          {auto && (
            <p className="rounded-card border border-info-line bg-info-bg p-3 text-[13px] text-info">
              {t("expense.auto")}
            </p>
          )}
          <Controller
            control={form.control}
            name="category"
            render={({ field }) => (
              <FormItem>
                <FormLabel className="text-[13px] font-bold">{t("expense.category")}</FormLabel>
                <Select value={field.value} onValueChange={field.onChange} disabled={auto}>
                  <SelectTrigger
                    className={`${inputClass} w-full`}
                    aria-label={t("expense.category")}
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {CATEGORIES.map((c) => (
                      <SelectItem key={c} value={c}>
                        {t(`expense.cat.${c}` as MessageKey)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </FormItem>
            )}
          />
          <div className="grid grid-cols-2 gap-3">
            <TextField
              control={form.control}
              name="amount"
              label={t("expense.amount")}
              suffix="đ"
              inputMode="numeric"
              disabled={auto}
            />
            <TextField
              control={form.control}
              name="month"
              label={t("expense.forMonth")}
              type="month"
              disabled={auto}
            />
          </div>
          <TextField
            control={form.control}
            name="paidOn"
            label={t("expense.paidOn")}
            type="date"
            disabled={auto}
          />
          <Controller
            control={form.control}
            name="note"
            render={({ field }) => (
              <FormItem>
                <FormLabel className="text-[13px] font-bold">{t("expense.note")}</FormLabel>
                <Textarea
                  {...field}
                  disabled={auto}
                  className="min-h-24 rounded-[10px] bg-card text-[15px]"
                />
              </FormItem>
            )}
          />
          <SwitchField
            control={form.control}
            name="recurring"
            label={t("expense.recurring")}
            hint={t("expense.recurringHint")}
          />
          <div className="mt-1 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            <Button type="button" variant="outline" size="lg" onClick={onClose}>
              {t("expense.cancel")}
            </Button>
            {!auto && (
              <Button type="submit" size="lg" disabled={create.isPending || update.isPending}>
                {t("expense.save")}
              </Button>
            )}
          </div>
          {expense && !auto && (
            <Button
              type="button"
              variant="outline"
              size="lg"
              className="border-destructive/40 font-bold text-destructive hover:bg-destructive/10"
              disabled={remove.isPending}
              onClick={() => remove.mutate(expense.id, done(t("expense.deleted")))}
            >
              {t("expense.delete")}
            </Button>
          )}
        </form>
      </Form>
    </FormSheet>
  );
}
