"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useState } from "react";
import { useForm, useWatch, type Resolver } from "react-hook-form";
import { toast } from "sonner";
import { z } from "zod";
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { newIdempotencyKey } from "@/lib/api";
import { parseVnd, vndNumber } from "@/lib/money";
import { t, tf, type MessageKey } from "@/lib/t";
import { useBuildings } from "../../rooms/hooks";
import { messageFor } from "../../problem/problem";
import { useCreateStaff, useUpdateStaff, type Level, type Staff } from "./hooks";
import type { PinReveal } from "./PinDialog";

const POSITIONS = [
  "FRONT_DESK",
  "HOUSEKEEPING",
  "SECURITY",
  "MANAGER",
  "MAINTENANCE",
  "OTHER",
] as const;
const APPS = ["NONE", "RECEPTIONIST", "HOUSEKEEPING", "MANAGER"] as const;
const PAY = ["MONTHLY", "PER_SHIFT", "HOURLY"] as const;
const num = z
  .string()
  .trim()
  .regex(/^[\d.,\s]+$/, "staff.form.required");

// Zod messages are keys; the form renders them through t() so they follow the route language.
const schema = z
  .object({
    name: z.string().trim().min(1, "staff.form.required").max(80),
    phone: z.string().trim(),
    position: z.enum(POSITIONS),
    appAccess: z.enum(APPS),
    username: z.string().trim(),
    payType: z.enum(PAY),
    rate: num,
    fixedAllowance: num,
    standardShifts: z.string().trim().regex(/^\d+$/, "staff.form.required"),
    startDate: z.string().min(1, "staff.form.required"),
    annualLeaveDays: z.string().trim().regex(/^\d+$/, "staff.form.required"),
  })
  .refine((v) => v.appAccess === "NONE" || /^[a-z0-9_.]{2,32}$/.test(v.username), {
    path: ["username"],
    message: "staff.form.usernameRule",
  });
type Values = z.infer<typeof schema>;

const field = "h-12 rounded-[10px] bg-card text-[15px]";
const groupTitle = "mt-3 text-[13px] font-bold uppercase tracking-wide text-ink-2";

function defaults(s?: Staff): Values {
  return {
    name: s?.name ?? "",
    phone: s?.phone ?? "",
    position: s?.position ?? "FRONT_DESK",
    appAccess: s?.appAccess ?? "RECEPTIONIST",
    username: s?.username ?? "",
    payType: s?.contract.payType ?? "MONTHLY",
    rate: vndNumber(s?.contract.rate ?? 0),
    fixedAllowance: vndNumber(s?.contract.fixedAllowance ?? 0),
    standardShifts: String(s?.contract.standardShifts ?? 26),
    startDate: s?.contract.startDate ?? new Date().toISOString().slice(0, 10),
    annualLeaveDays: String(s?.contract.annualLeaveDays ?? 12),
  };
}

// Add (no `staff`) or edit one staff member in a right sheet; full width on phones (boards ThemNhanVien*).
export function StaffForm({
  open,
  staff,
  guesthouseCode,
  onClose,
  onReveal,
}: {
  open: boolean;
  staff?: Staff;
  guesthouseCode?: string;
  onClose: () => void;
  onReveal: (r: PinReveal) => void;
}) {
  const buildings = useBuildings().data ?? [];
  const create = useCreateStaff();
  const update = useUpdateStaff();
  const [key] = useState(newIdempotencyKey);
  const [access, setAccess] = useState<Record<string, Level>>({});
  const form = useForm<Values>({
    resolver: zodResolver(schema) as Resolver<Values>,
    defaultValues: defaults(staff),
  });
  const app = useWatch({ control: form.control, name: "appAccess" });

  const submit = form.handleSubmit((v) => {
    const contract = {
      payType: v.payType,
      rate: parseVnd(v.rate),
      fixedAllowance: parseVnd(v.fixedAllowance),
      standardShifts: Number(v.standardShifts),
      startDate: v.startDate,
      annualLeaveDays: Number(v.annualLeaveDays),
    };
    const base = {
      name: v.name,
      phone: v.phone || null,
      position: v.position,
      appAccess: v.appAccess,
      contract,
    };
    const failed = (e: unknown) =>
      toast.error(
        messageFor(e, { CONFLICT: "staff.usernameTaken", VALIDATION_FAILED: "staff.saveFailed" }),
      );
    if (staff)
      return update.mutate(
        { userId: staff.id, body: base },
        {
          onSuccess: () => {
            toast.success(t("staff.saved"));
            onClose();
          },
          onError: failed,
        },
      );
    create.mutate(
      {
        key,
        body: {
          ...base,
          username: v.appAccess === "NONE" ? null : v.username,
          buildingAccess: buildings.map((b) => ({
            buildingId: b.id,
            level: access[b.id] ?? "NONE",
          })),
        },
      },
      {
        onSuccess: (r) => {
          onClose();
          if (r.oneTimePin)
            onReveal({
              pin: r.oneTimePin,
              name: r.staff.name,
              username: r.staff.username,
              guesthouseCode,
              created: true,
              summary: { position: t(`staff.position.${r.staff.position}` as MessageKey) },
            });
          else toast.success(t("staff.saved"));
        },
        onError: failed,
      },
    );
  });

  const pending = create.isPending || update.isPending;
  const text = (
    name: keyof Values,
    label: MessageKey,
    extra?: { suffix?: string; type?: string; inputMode?: "numeric" },
  ) => (
    <FormField
      control={form.control}
      name={name}
      render={({ field: f }) => (
        <FormItem>
          <FormLabel className="text-[13px] font-bold">{t(label)}</FormLabel>
          <div className="relative">
            <FormControl>
              <Input {...f} type={extra?.type} inputMode={extra?.inputMode} className={field} />
            </FormControl>
            {extra?.suffix && (
              <span className="pointer-events-none absolute right-3.5 top-1/2 -translate-y-1/2 text-sm text-muted-foreground">
                {extra.suffix}
              </span>
            )}
          </div>
          <FormMessage>
            {form.formState.errors[name]?.message &&
              t(form.formState.errors[name]!.message as MessageKey)}
          </FormMessage>
        </FormItem>
      )}
    />
  );
  const select = (
    name: "position" | "appAccess" | "payType",
    label: MessageKey,
    values: readonly string[],
    prefix: string,
  ) => (
    <FormField
      control={form.control}
      name={name}
      render={({ field: f }) => (
        <FormItem>
          <FormLabel className="text-[13px] font-bold">{t(label)}</FormLabel>
          <Select value={f.value} onValueChange={f.onChange}>
            <FormControl>
              <SelectTrigger className={`${field} w-full`}>
                <SelectValue />
              </SelectTrigger>
            </FormControl>
            <SelectContent>
              {values.map((v) => (
                <SelectItem key={v} value={v}>
                  {t(`${prefix}.${v}` as MessageKey)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </FormItem>
      )}
    />
  );

  return (
    <Sheet open={open} onOpenChange={(o) => !o && onClose()}>
      <SheetContent side="right" className="w-full gap-0 overflow-y-auto bg-card sm:max-w-[520px]">
        <SheetHeader>
          <SheetTitle className="text-[22px]">
            {staff ? t("staff.editTitle") : t("staff.addTitle")}
          </SheetTitle>
          <SheetDescription className="sr-only">{t("staff.form.details")}</SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form onSubmit={submit} noValidate className="flex flex-col gap-3 px-4 pb-6">
            <h3 className={groupTitle}>{t("staff.form.details")}</h3>
            <div className="grid gap-3 sm:grid-cols-2">
              {text("name", "staff.form.name")}
              {text("phone", "staff.form.phone", { type: "tel" })}
            </div>
            <h3 className={groupTitle}>{t("staff.form.positionApp")}</h3>
            <div className="grid gap-3 sm:grid-cols-2">
              {select("position", "staff.form.position", POSITIONS, "staff.position")}
              {select("appAccess", "staff.form.appAccess", APPS, "staff.app")}
            </div>
            {!staff && app !== "NONE" && text("username", "staff.form.username")}
            <p className="text-[12px] text-ink-2">{t("staff.form.appHint")}</p>
            <h3 className={groupTitle}>{t("staff.form.contract")}</h3>
            <div className="grid gap-3 sm:grid-cols-2">
              {select("payType", "staff.form.payType", PAY, "staff.payType")}
              {text("rate", "staff.form.rate", { suffix: "đ", inputMode: "numeric" })}
              {text("fixedAllowance", "staff.form.allowance", {
                suffix: "đ",
                inputMode: "numeric",
              })}
              {text("standardShifts", "staff.form.standardShifts", { inputMode: "numeric" })}
              {text("startDate", "staff.form.startDate", { type: "date" })}
              {text("annualLeaveDays", "staff.form.leaveDays", { inputMode: "numeric" })}
            </div>
            {!staff && (
              <>
                <h3 className={groupTitle}>{t("staff.form.buildings")}</h3>
                <p className="-mt-1 text-[12px] text-ink-2">{t("staff.form.buildingsHint")}</p>
                {buildings.map((b) => (
                  <div key={b.id} className="flex items-center gap-3">
                    <span className="w-[72px] text-[14px] font-bold">
                      {tf("staff.form.building", { code: b.code })}
                    </span>
                    <ToggleGroup
                      type="single"
                      value={access[b.id] ?? "NONE"}
                      onValueChange={(v) => v && setAccess({ ...access, [b.id]: v as Level })}
                      aria-label={b.name}
                      disabled={app === "NONE"}
                      className="flex-1 gap-0 rounded-card bg-secondary p-1"
                    >
                      {(["NONE", "VIEW", "EDIT"] as const).map((l) => (
                        <ToggleGroupItem
                          key={l}
                          value={l}
                          className="h-10 flex-1 rounded-[10px]! text-[14px] font-bold data-[state=on]:bg-card data-[state=on]:shadow-sm"
                        >
                          {t(
                            `staff.form.${l === "NONE" ? "none" : l === "VIEW" ? "view" : "editLevel"}` as MessageKey,
                          )}
                        </ToggleGroupItem>
                      ))}
                    </ToggleGroup>
                  </div>
                ))}
              </>
            )}
            <div className="mt-2 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
              <Button type="button" variant="outline" size="lg" onClick={onClose}>
                {t("staff.form.cancel")}
              </Button>
              <Button type="submit" size="lg" disabled={pending}>
                {t("staff.form.save")}
              </Button>
            </div>
            {!staff && <p className="text-[12px] text-ink-2">{t("staff.form.pinHint")}</p>}
          </form>
        </Form>
      </SheetContent>
    </Sheet>
  );
}
