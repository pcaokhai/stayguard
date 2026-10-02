"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useState } from "react";
import { useForm, type Resolver } from "react-hook-form";
import { toast } from "sonner";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { Form } from "@/components/ui/form";
import { newIdempotencyKey } from "@/lib/api";
import { parseVnd, vndNumber } from "@/lib/money";
import { t } from "@/lib/t";
import { FormSheet, SwitchField, TextField } from "../FormFields";
import { statusOf, useCreateItem, useUpdateItem, type Service } from "./hooks";

const num = z.string().trim().regex(/\d/, "items.required");
const schema = z.object({
  nameVi: z.string().trim().min(1, "items.required"),
  nameEn: z.string().trim(),
  price: num,
  unitCost: z.string().trim(),
  opening: z.string().trim(),
  unit: z.string().trim().min(1, "items.required"),
  lowAt: z.string().trim().regex(/^\d+$/, "items.required"),
  onSale: z.boolean(),
});
type Values = z.infer<typeof schema>;

// Add (no `item`) or edit one item; stock is never edited here, only restocked or counted (docs/15).
export function ItemForm({ item, onClose }: { item?: Service; onClose: () => void }) {
  const create = useCreateItem();
  const update = useUpdateItem();
  const [key] = useState(newIdempotencyKey);
  const form = useForm<Values>({
    resolver: zodResolver(
      schema
        .refine((v) => !!item || /\d/.test(v.unitCost), {
          path: ["unitCost"],
          message: "items.required",
        })
        .refine((v) => !!item || /^\d+$/.test(v.opening), {
          path: ["opening"],
          message: "items.required",
        }),
    ) as Resolver<Values>,
    defaultValues: {
      nameVi: item?.name.vi ?? "",
      nameEn: item && item.name.en !== item.name.vi ? item.name.en : "",
      price: vndNumber(item?.price ?? 0),
      unitCost: "",
      opening: "0",
      unit: item?.unit ?? "",
      lowAt: String(item?.lowStockAt ?? 5),
      onSale: item?.onSale ?? true,
    },
  });
  const submit = form.handleSubmit((v) => {
    const name = { vi: v.nameVi, en: v.nameEn || v.nameVi };
    const failed = (e: unknown) =>
      toast.error(statusOf(e) === 409 ? t("items.nameTaken") : t("items.saveFailed"));
    const done = () => {
      toast.success(t("items.saved"));
      onClose();
    };
    if (item)
      return update.mutate(
        {
          code: item.code,
          body: {
            name,
            price: parseVnd(v.price),
            unit: v.unit,
            lowStockAt: Number(v.lowAt),
            onSale: v.onSale,
          },
        },
        { onSuccess: done, onError: failed },
      );
    create.mutate(
      {
        key,
        body: {
          name,
          price: parseVnd(v.price),
          unitCost: parseVnd(v.unitCost),
          openingQuantity: Number(v.opening),
          unit: v.unit,
          lowStockAt: Number(v.lowAt),
          onSale: v.onSale,
        },
      },
      { onSuccess: done, onError: failed },
    );
  });
  const c = form.control;
  return (
    <FormSheet open onClose={onClose} title={item ? t("items.editTitlePc") : t("items.addTitle")}>
      <Form {...form}>
        <form onSubmit={submit} noValidate className="flex flex-col gap-3 px-4 pb-6">
          <TextField control={c} name="nameVi" label={t("items.name")} />
          <TextField
            control={c}
            name="nameEn"
            label={t("items.nameEn")}
            hint={t("items.nameEnHint")}
          />
          <div className="grid grid-cols-2 gap-3">
            <TextField
              control={c}
              name="price"
              label={t("items.price")}
              suffix="đ"
              inputMode="numeric"
            />
            {!item && (
              <TextField
                control={c}
                name="unitCost"
                label={t("items.unitCost")}
                suffix="đ"
                inputMode="numeric"
              />
            )}
            {item && <TextField control={c} name="unit" label={t("items.unit")} />}
          </div>
          {!item && (
            <TextField control={c} name="opening" label={t("items.opening")} inputMode="numeric" />
          )}
          <div className="grid grid-cols-2 gap-3">
            {!item && <TextField control={c} name="unit" label={t("items.unit")} />}
            <TextField
              control={c}
              name="lowAt"
              label={t("items.lowAt")}
              inputMode="numeric"
              className={item ? "col-span-2" : undefined}
            />
          </div>
          <SwitchField
            control={c}
            name="onSale"
            label={t("items.onSale")}
            hint={t("items.onSaleHint")}
          />
          <p className="text-[12px] text-muted-foreground">
            {item ? t("items.editNote") : t("items.priceNote")}
          </p>
          <div className="mt-1 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            <Button type="button" variant="outline" size="lg" onClick={onClose}>
              {t("items.cancel")}
            </Button>
            <Button type="submit" size="lg" disabled={create.isPending || update.isPending}>
              {item ? t("items.saveChanges") : t("items.save")}
            </Button>
          </div>
        </form>
      </Form>
    </FormSheet>
  );
}
