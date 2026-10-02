"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useRouter, useSearchParams } from "next/navigation";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { Shake } from "@/components/motion";
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
import { Input } from "@/components/ui/input";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { newIdempotencyKey } from "../../lib/api";
import { lp } from "../../lib/locale";
import { parseVnd } from "../../lib/money";
import { t } from "../../lib/t";
import { useBuildings } from "../rooms/hooks";
import { useCreateStay, useRoom } from "./hooks";
import { rentalLabel, type RentalType } from "./labels";

const RENTAL_TYPES: RentalType[] = ["HOURLY", "OVERNIGHT", "DAILY"];
const DEFAULT_DEPOSIT = "100000";

// Zod messages are keys; the form renders them through t() so they follow the route language.
const schema = z.object({
  rentalType: z.enum(["HOURLY", "OVERNIGHT", "DAILY"]),
  guestName: z.string().trim().min(1, "stay.guestNameRequired").max(120),
  guestPhone: z.string().trim().min(6, "stay.guestPhoneInvalid").max(20, "stay.guestPhoneInvalid"),
  deposit: z.string(),
});
type Values = z.infer<typeof schema>;

export function CheckinForm() {
  const roomId = useSearchParams().get("room");
  const router = useRouter();
  const room = useRoom(roomId);
  const building = useBuildings().data?.find((b) => b.id === room.data?.buildingId);
  const create = useCreateStay(roomId ?? "");
  // One key per user action: a retry after a failure reuses it.
  const [key] = useState(newIdempotencyKey);
  const [tries, setTries] = useState(0);
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      rentalType: "HOURLY",
      guestName: "",
      guestPhone: "",
      deposit: DEFAULT_DEPOSIT,
    },
  });

  const submit = form.handleSubmit(
    (v) =>
      create.mutate(
        {
          key,
          body: {
            rentalType: v.rentalType,
            guestName: v.guestName,
            guestPhone: v.guestPhone,
            deposit: parseVnd(v.deposit),
          },
        },
        { onSuccess: (stay) => router.replace(lp(`/stay?id=${stay.id}`)) },
      ),
    () => setTries((n) => n + 1),
  );

  return (
    <AppFrame tabs={false}>
      <main className="mx-auto flex w-full max-w-[480px] flex-col">
        <TopBar
          title={`${t("stay.checkinTitle")} ${room.data?.code ?? ""}`}
          subtitle={[building?.name, room.data?.unitType.name.vi].filter(Boolean).join(" · ")}
          back="/rooms"
        />
        <Form {...form}>
          <form onSubmit={submit} noValidate className="flex flex-1 flex-col gap-4 px-5 pb-8">
            <FormField
              control={form.control}
              name="rentalType"
              render={({ field }) => (
                <FormItem>
                  <FormLabel className="text-base font-semibold">{t("stay.rentalType")}</FormLabel>
                  <ToggleGroup
                    type="single"
                    variant="outline"
                    value={field.value}
                    onValueChange={(v) => v && field.onChange(v)}
                    className="grid w-full grid-cols-3 gap-2"
                  >
                    {RENTAL_TYPES.map((r) => (
                      <ToggleGroupItem
                        key={r}
                        value={r}
                        className="h-14 rounded-[10px] text-[15px] font-semibold data-[state=on]:border-2 data-[state=on]:border-primary data-[state=on]:bg-primary/10"
                      >
                        {rentalLabel(r)}
                      </ToggleGroupItem>
                    ))}
                  </ToggleGroup>
                </FormItem>
              )}
            />
            <TextField
              methods={form}
              name="guestName"
              label={t("stay.guestName")}
              tries={tries}
              maxLength={120}
            />
            <TextField
              methods={form}
              name="guestPhone"
              label={t("stay.guestPhone")}
              tries={tries}
              maxLength={20}
              inputMode="tel"
            />
            <TextField
              methods={form}
              name="deposit"
              label={t("stay.deposit")}
              tries={tries}
              inputMode="numeric"
            />
            <p className="rounded-[10px] bg-secondary p-3 text-[13px] text-ink-2">
              {t("stay.serverClock")}
            </p>
            {create.isError && (
              <p role="alert" className="text-sm text-warn">
                {t("stay.checkinFailed")}
              </p>
            )}
            <Button
              type="submit"
              size="lg"
              className="mt-auto"
              loading={create.isPending}
              disabled={!room.data}
            >
              {t("stay.confirmCheckin")}
            </Button>
          </form>
        </Form>
      </main>
    </AppFrame>
  );
}

function TextField({
  methods,
  name,
  label,
  tries,
  ...input
}: {
  methods: ReturnType<typeof useForm<Values>>;
  name: "guestName" | "guestPhone" | "deposit";
  label: string;
  tries: number;
} & Omit<React.ComponentProps<"input">, "form" | "name">) {
  return (
    <FormField
      control={methods.control}
      name={name}
      render={({ field, fieldState }) => (
        <FormItem>
          <FormLabel className="font-semibold">{label}</FormLabel>
          <Shake trigger={fieldState.error ? tries : 0}>
            <FormControl>
              <Input className="h-11 rounded-[10px] bg-card px-4 text-base" {...input} {...field} />
            </FormControl>
          </Shake>
          <FormMessage>
            {fieldState.error && t(fieldState.error.message as "stay.guestNameRequired")}
          </FormMessage>
        </FormItem>
      )}
    />
  );
}
