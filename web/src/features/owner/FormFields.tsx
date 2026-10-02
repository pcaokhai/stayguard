"use client";

import type { ReactNode } from "react";
import type { Control, FieldValues, Path } from "react-hook-form";
import { FormControl, FormField, FormItem, FormLabel, FormMessage } from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { Switch } from "@/components/ui/switch";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { t, type MessageKey } from "@/lib/t";
import { cn } from "@/lib/utils";

export const inputClass = "h-12 min-w-0 rounded-[10px] bg-card text-[15px]";

// Right sheet; full width on phones (the designs' phone pages), 520 px from md.
export function FormSheet({
  open,
  onClose,
  title,
  description,
  children,
}: {
  open: boolean;
  onClose: () => void;
  title: string;
  description?: string;
  children: ReactNode;
}) {
  return (
    <Sheet open={open} onOpenChange={(o) => !o && onClose()}>
      <SheetContent side="right" className="w-full gap-0 overflow-y-auto bg-card sm:max-w-[520px]">
        <SheetHeader>
          <SheetTitle className="text-[22px]">{title}</SheetTitle>
          <SheetDescription className={description ? "text-[13px]" : "sr-only"}>
            {description ?? title}
          </SheetDescription>
        </SheetHeader>
        {children}
      </SheetContent>
    </Sheet>
  );
}

// Zod messages are message keys; they render through t() so they follow the route language.
export function TextField<T extends FieldValues>({
  control,
  name,
  label,
  suffix,
  type,
  inputMode,
  hint,
  className,
  disabled,
}: {
  control: Control<T>;
  name: Path<T>;
  label: string;
  suffix?: string;
  type?: string;
  inputMode?: "numeric" | "text";
  hint?: string;
  className?: string;
  disabled?: boolean;
}) {
  return (
    <FormField
      control={control}
      name={name}
      render={({ field, fieldState }) => (
        <FormItem className={cn("min-w-0 grid-cols-[minmax(0,1fr)]", className)}>
          <FormLabel className="text-[13px] font-bold">{label}</FormLabel>
          <div className="relative">
            <FormControl>
              <Input
                {...field}
                value={field.value ?? ""}
                type={type}
                inputMode={inputMode}
                disabled={disabled}
                className={inputClass}
              />
            </FormControl>
            {suffix && (
              <span className="pointer-events-none absolute right-3.5 top-1/2 -translate-y-1/2 text-sm text-muted-foreground">
                {suffix}
              </span>
            )}
          </div>
          {hint && !fieldState.error && <p className="text-[12px] text-muted-foreground">{hint}</p>}
          <FormMessage>
            {fieldState.error?.message ? t(fieldState.error.message as MessageKey) : null}
          </FormMessage>
        </FormItem>
      )}
    />
  );
}

export function SwitchField<T extends FieldValues>({
  control,
  name,
  label,
  hint,
}: {
  control: Control<T>;
  name: Path<T>;
  label: string;
  hint?: string;
}) {
  return (
    <FormField
      control={control}
      name={name}
      render={({ field }) => (
        <FormItem className="flex flex-row items-center justify-between gap-3">
          <div>
            <FormLabel className="text-[15px] font-bold">{label}</FormLabel>
            {hint && <p className="text-[12px] text-muted-foreground">{hint}</p>}
          </div>
          <FormControl>
            <Switch checked={!!field.value} onCheckedChange={field.onChange} />
          </FormControl>
        </FormItem>
      )}
    />
  );
}

// Segmented choice such as the room type (Phòng thường | VIP).
export function SegmentField<T extends FieldValues>({
  control,
  name,
  label,
  options,
}: {
  control: Control<T>;
  name: Path<T>;
  label: string;
  options: { value: string; label: string }[];
}) {
  return (
    <FormField
      control={control}
      name={name}
      render={({ field }) => (
        <FormItem>
          <FormLabel className="text-[13px] font-bold">{label}</FormLabel>
          <ToggleGroup
            type="single"
            value={field.value}
            onValueChange={(v) => v && field.onChange(v)}
            aria-label={label}
            className="flex w-full gap-0 rounded-card bg-secondary p-1"
          >
            {options.map((o) => (
              <ToggleGroupItem
                key={o.value}
                value={o.value}
                className="h-11 flex-1 rounded-[10px]! text-[15px] font-bold data-[state=on]:bg-card data-[state=on]:shadow-sm"
              >
                {o.label}
              </ToggleGroupItem>
            ))}
          </ToggleGroup>
        </FormItem>
      )}
    />
  );
}
