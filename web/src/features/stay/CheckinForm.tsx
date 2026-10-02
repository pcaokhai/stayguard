"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { Info } from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";
import { useState } from "react";
import { useForm, useWatch } from "react-hook-form";
import { z } from "zod";
import { Shake } from "@/components/motion";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { Badge } from "@/components/ui/badge";
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
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { newIdempotencyKey } from "../../lib/api";
import { localized, lp } from "../../lib/locale";
import { parseVnd, vndNumber } from "../../lib/money";
import { toast } from "sonner";
import { t, tf } from "../../lib/t";
import { IdBlock, photoProblem, type IdPhotos } from "../guestid/IdBlock";
import { uploadIdPhoto } from "../guestid/hooks";
import { FlowSplit } from "../rooms/FlowSplit";
import { STATUS } from "../rooms/status";
import { useBuildings } from "../rooms/hooks";
import { useCreateStay, useRoom } from "./hooks";
import { rentalLabel, type RentalType } from "./labels";

const RENTAL_TYPES: RentalType[] = ["HOURLY", "OVERNIGHT", "DAILY"];
const DEFAULT_DEPOSIT = 100000;

// Zod messages are keys; the form renders them through t() so they follow the route language.
const schema = z.object({
  rentalType: z.enum(["HOURLY", "OVERNIGHT", "DAILY"]),
  guestName: z.string().trim().min(1, "stay.guestNameRequired").max(120),
  guestPhone: z.string().trim().min(6, "stay.guestPhoneInvalid").max(20, "stay.guestPhoneInvalid"),
  deposit: z.string(),
  // Optional national ID: 9 to 12 digits, encrypted by the server and never shown to the front desk again.
  idNumber: z
    .string()
    .trim()
    .refine((v) => v === "" || /^[0-9]{9,12}$/.test(v), "guestId.idNumberInvalid"),
});
type Values = z.infer<typeof schema>;

// The guest ID block (number, photos, consent) arrives with F-W1; rates per rental type are not
// readable by the front desk, so the type cards carry names only.
export function CheckinForm() {
  const roomId = useSearchParams().get("room");
  const router = useRouter();
  const room = useRoom(roomId);
  const building = useBuildings().data?.find((b) => b.id === room.data?.buildingId);
  const create = useCreateStay(roomId ?? "");
  // One key per user action: a retry after a failure reuses it.
  const [key] = useState(newIdempotencyKey);
  const [tries, setTries] = useState(0);
  const [photos, setPhotos] = useState<IdPhotos>({ FRONT: null, BACK: null });
  const [consent, setConsent] = useState(false);
  const [consentError, setConsentError] = useState("");
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      rentalType: "HOURLY",
      guestName: "",
      guestPhone: "",
      deposit: vndNumber(DEFAULT_DEPOSIT),
    },
  });

  const idNumber = useWatch({ control: form.control, name: "idNumber" });
  const hasId = idNumber !== "" || !!photos.FRONT || !!photos.BACK;
  const badPhoto = [photos.FRONT, photos.BACK].some((f) => f && photoProblem(f));

  const submit = form.handleSubmit(
    (v) => {
      // Consent comes first: no number or photo is sent without it.
      if (hasId && !consent)
        return (setConsentError(t("guestId.consentRequired")), setTries((n) => n + 1));
      if (badPhoto) return setTries((n) => n + 1);
      create.mutate(
        {
          key,
          body: {
            rentalType: v.rentalType,
            guestName: v.guestName,
            guestPhone: v.guestPhone,
            deposit: parseVnd(v.deposit),
            ...(v.idNumber ? { idNumber: v.idNumber, idConsent: true } : {}),
          },
        },
        {
          onSuccess: async (stay) => {
            // Photos go up after the stay exists; a failed one is reported, the stay is kept.
            for (const side of ["FRONT", "BACK"] as const) {
              const file = photos[side];
              if (!file) continue;
              try {
                await uploadIdPhoto(stay.id, side, file);
              } catch {
                toast.error(
                  tf("guestId.uploadFailed", {
                    side: t(side === "FRONT" ? "guestId.photoFront" : "guestId.photoBack"),
                  }),
                );
              }
            }
            router.replace(lp(`/stay?id=${stay.id}`));
          },
        },
      );
    },
    () => setTries((n) => n + 1),
  );
  const status = STATUS[room.data?.status ?? "VACANT"];

  return (
    <AppFrame tabs={false}>
      <FlowSplit roomId={roomId}>
        <TopBar
          title={`${t("stay.checkinTitle")} ${room.data?.code ?? ""}`}
          subtitle={
            room.data
              ? tf("stay.checkinPlace", {
                  building: building?.name ?? "",
                  type: localized(room.data.unitType.name),
                })
              : undefined
          }
          back="/rooms"
          right={
            <Badge
              variant={status.variant}
              className="mr-3 hidden h-7 px-3 text-sm font-bold lg:inline-flex"
            >
              {t(status.label)}
            </Badge>
          }
        />
        <Form {...form}>
          <form onSubmit={submit} noValidate className="flex flex-col gap-4 px-5 pb-8 lg:pb-0">
            <FormField
              control={form.control}
              name="rentalType"
              render={({ field }) => (
                <FormItem>
                  <FormLabel className="text-base font-semibold">{t("stay.rentalType")}</FormLabel>
                  <RadioGroup
                    value={field.value}
                    onValueChange={field.onChange}
                    className="grid gap-2 lg:grid-cols-3"
                  >
                    {RENTAL_TYPES.map((r) => (
                      <label
                        key={r}
                        className="flex h-14 cursor-pointer items-center gap-3 rounded-xl border bg-card px-4 font-bold transition-colors has-data-[state=checked]:border-2 has-data-[state=checked]:border-primary has-data-[state=checked]:bg-info-bg/60"
                      >
                        <RadioGroupItem value={r} className="sr-only" />
                        {rentalLabel(r)}
                      </label>
                    ))}
                  </RadioGroup>
                </FormItem>
              )}
            />
            <TextField
              methods={form}
              name="guestName"
              label={t("stay.guestName")}
              tries={tries}
              maxLength={120}
              autoComplete="off"
            />
            <TextField
              methods={form}
              name="guestPhone"
              label={t("stay.guestPhone")}
              tries={tries}
              maxLength={20}
              inputMode="tel"
              autoComplete="off"
            />
            <TextField
              methods={form}
              name="deposit"
              label={t("stay.deposit")}
              tries={tries}
              inputMode="numeric"
            />
            <TextField
              methods={form}
              name="idNumber"
              label={t("guestId.idNumberOptional")}
              tries={tries}
              inputMode="numeric"
              maxLength={12}
              autoComplete="off"
              hint={t("guestId.idNumberHint")}
            />
            <IdBlock
              photos={photos}
              onPhoto={(side, file) => setPhotos((p) => ({ ...p, [side]: file }))}
              consent={consent}
              onConsent={(v) => {
                setConsent(v);
                setConsentError("");
              }}
              error={consentError}
            />
            <p className="flex items-start gap-2.5 rounded-xl bg-secondary p-3 text-[13px] text-ink-2">
              <Info className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
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
              className="mt-6"
              loading={create.isPending}
              disabled={!room.data}
            >
              {t("stay.confirmCheckin")}
            </Button>
          </form>
        </Form>
      </FlowSplit>
    </AppFrame>
  );
}

function TextField({
  methods,
  name,
  label,
  tries,
  hint,
  ...input
}: {
  methods: ReturnType<typeof useForm<Values>>;
  name: "guestName" | "guestPhone" | "deposit" | "idNumber";
  label: string;
  tries: number;
  hint?: string;
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
              <Input className="h-12 rounded-[10px] bg-card px-4 text-base" {...input} {...field} />
            </FormControl>
          </Shake>
          {hint && !fieldState.error && <p className="text-[13px] text-muted-foreground">{hint}</p>}
          <FormMessage>
            {fieldState.error && t(fieldState.error.message as "stay.guestNameRequired")}
          </FormMessage>
        </FormItem>
      )}
    />
  );
}
