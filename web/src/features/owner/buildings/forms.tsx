"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useState } from "react";
import { useForm, useWatch, type Resolver } from "react-hook-form";
import { toast } from "sonner";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { Form } from "@/components/ui/form";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { newIdempotencyKey } from "@/lib/api";
import { localized } from "@/lib/locale";
import { t, tf, type MessageKey } from "@/lib/t";
import { cn } from "@/lib/utils";
import { FormSheet, SegmentField, SwitchField, TextField } from "../FormFields";
import type { UnitTypeRates } from "../rates/hooks";
import { between, buildingCodes, codeRange, summarise } from "./codes";
import {
  statusOf,
  useCreateBuilding,
  useCreateFloor,
  useCreateRooms,
  useUpdateBuilding,
  useUpdateRoom,
  type Room,
  type RoomFeature,
} from "./hooks";

type Types = UnitTypeRates[];
const typeOptions = (types: Types) =>
  types.map((x) => ({ value: x.code, label: localized(x.name) }));
const FEATURES: RoomFeature[] = ["DOUBLE_BED", "TWIN_BEDS", "WINDOW", "BATHTUB"];
const req = "buildings.required";
const count = (min: number, max: number) =>
  z
    .string()
    .trim()
    .regex(/^\d+$/, req)
    .refine((v) => Number(v) >= min && Number(v) <= max, req);
const Actions = ({
  onClose,
  pending,
  label,
}: {
  onClose: () => void;
  pending: boolean;
  label: string;
}) => (
  <div className="mt-1 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
    <Button type="button" variant="outline" size="lg" onClick={onClose}>
      {t("buildings.cancel")}
    </Button>
    <Button type="submit" size="lg" disabled={pending}>
      {label}
    </Button>
  </div>
);
const note = "rounded-card border border-info-line bg-info-bg p-3 text-[13px] text-info";

// ---- Add building (ThemToa) ----
const buildingSchema = z.object({
  name: z.string().trim().min(1, req),
  code: z
    .string()
    .trim()
    .regex(/^[A-Z]{1,3}$/, req),
  floors: count(1, 30),
  perFloor: count(0, 50),
  type: z.string().min(1),
});
type BuildingValues = z.infer<typeof buildingSchema>;

export function BuildingForm({
  types,
  buildingsNow,
  onClose,
}: {
  types: Types;
  buildingsNow: number;
  onClose: () => void;
}) {
  const create = useCreateBuilding();
  const [key] = useState(newIdempotencyKey);
  const form = useForm<BuildingValues>({
    resolver: zodResolver(buildingSchema) as Resolver<BuildingValues>,
    defaultValues: { name: "", code: "", floors: "3", perFloor: "5", type: types[0]?.code ?? "" },
  });
  const v = useWatch({ control: form.control });
  const codes = buildingCodes(v.code || "?", Number(v.floors) || 0, Number(v.perFloor) || 0);
  const submit = form.handleSubmit((x) =>
    create.mutate(
      {
        key,
        body: {
          code: x.code,
          name: x.name,
          floors: Number(x.floors),
          roomsPerFloor: Number(x.perFloor),
          unitTypeCode: x.type,
        },
      },
      {
        onSuccess: () => {
          toast.success(t("buildings.buildingCreated"));
          onClose();
        },
        onError: (e) =>
          toast.error(statusOf(e) === 409 ? t("buildings.codeTaken") : t("buildings.saveFailed")),
      },
    ),
  );
  return (
    <FormSheet
      open
      onClose={onClose}
      title={t("buildings.newBuilding")}
      description={tf("buildings.buildingsNow", { n: buildingsNow })}
    >
      <Form {...form}>
        <form onSubmit={submit} noValidate className="flex flex-col gap-3 px-4 pb-6">
          <TextField control={form.control} name="name" label={t("buildings.buildingName")} />
          <TextField
            control={form.control}
            name="code"
            label={t("buildings.buildingCode")}
            hint={tf("buildings.buildingCodeHint", { code: v.code || "E" })}
          />
          <div className="grid grid-cols-2 gap-3">
            <TextField
              control={form.control}
              name="floors"
              label={t("buildings.floors")}
              inputMode="numeric"
            />
            <TextField
              control={form.control}
              name="perFloor"
              label={t("buildings.perFloor")}
              inputMode="numeric"
            />
          </div>
          <SegmentField
            control={form.control}
            name="type"
            label={t("buildings.defaultType")}
            options={typeOptions(types)}
          />
          <dl className="grid gap-1 rounded-card border border-border p-3.5 text-[14px]">
            <div className="flex justify-between gap-3">
              <dt className="text-ink-2">{t("buildings.willCreate")}</dt>
              <dd className="text-right font-bold">
                {tf("buildings.willCreateText", { n: codes.length, list: summarise(codes) })}
              </dd>
            </div>
            <div className="flex justify-between gap-3">
              <dt className="text-ink-2">{t("buildings.staffAccess")}</dt>
              <dd className="text-right font-bold">{t("buildings.staffAccessText")}</dd>
            </div>
          </dl>
          <p className="text-[12px] text-muted-foreground">{t("buildings.buildingNote")}</p>
          <Actions
            onClose={onClose}
            pending={create.isPending}
            label={t("buildings.createBuilding")}
          />
        </form>
      </Form>
    </FormSheet>
  );
}

// ---- Edit building name (Sửa tòa) ----
export function BuildingNameForm({
  id,
  name,
  onClose,
}: {
  id: string;
  name: string;
  onClose: () => void;
}) {
  const update = useUpdateBuilding();
  const form = useForm<{ name: string }>({
    resolver: zodResolver(z.object({ name: z.string().trim().min(1, req) })) as Resolver<{
      name: string;
    }>,
    defaultValues: { name },
  });
  const submit = form.handleSubmit((x) =>
    update.mutate(
      { id, name: x.name },
      { onSuccess: onClose, onError: () => toast.error(t("buildings.saveFailed")) },
    ),
  );
  return (
    <FormSheet open onClose={onClose} title={t("buildings.editBuilding")}>
      <Form {...form}>
        <form onSubmit={submit} noValidate className="flex flex-col gap-3 px-4 pb-6">
          <TextField control={form.control} name="name" label={t("buildings.buildingName")} />
          <Actions onClose={onClose} pending={update.isPending} label={t("buildings.save")} />
        </form>
      </Form>
    </FormSheet>
  );
}

// ---- Add floor (ThemTang) ----
const floorSchema = z.object({
  name: z.string().trim().min(1, req),
  withRooms: z.boolean(),
  count: z.string().trim(),
  start: z.string().trim(),
  type: z.string().min(1),
});
type FloorValues = z.infer<typeof floorSchema>;

export function FloorForm({
  buildingId,
  buildingName,
  buildingCode,
  floorsNow,
  types,
  onClose,
}: {
  buildingId: string;
  buildingName: string;
  buildingCode: string;
  floorsNow: number;
  types: Types;
  onClose: () => void;
}) {
  const create = useCreateFloor();
  const [key] = useState(newIdempotencyKey);
  const next = floorsNow + 1;
  const form = useForm<FloorValues>({
    resolver: zodResolver(floorSchema) as Resolver<FloorValues>,
    defaultValues: {
      name: tf("buildings.floor", { n: next }),
      withRooms: true,
      count: "6",
      start: `${buildingCode}${next}01`,
      type: types[0]?.code ?? "",
    },
  });
  const v = useWatch({ control: form.control });
  const codes = v.withRooms ? codeRange(v.start ?? "", Number(v.count) || 0) : [];
  const typeName = localized(types.find((x) => x.code === v.type)?.name ?? { vi: "", en: "" });
  const submit = form.handleSubmit((x) => {
    if (
      x.withRooms &&
      (!/^\d+$/.test(x.count) || Number(x.count) < 1 || Number(x.count) > 50 || codes.length === 0)
    ) {
      form.setError("count", { message: req });
      return;
    }
    create.mutate(
      {
        key,
        buildingId,
        body: {
          name: x.name,
          rooms: x.withRooms
            ? { count: Number(x.count), startCode: x.start, unitTypeCode: x.type }
            : null,
        },
      },
      {
        onSuccess: () => {
          toast.success(t("buildings.floorCreated"));
          onClose();
        },
        onError: () => toast.error(t("buildings.saveFailed")),
      },
    );
  });
  return (
    <FormSheet
      open
      onClose={onClose}
      title={t("buildings.newFloor")}
      description={tf("buildings.floorsNow", { name: buildingName, n: floorsNow })}
    >
      <Form {...form}>
        <form onSubmit={submit} noValidate className="flex flex-col gap-3 px-4 pb-6">
          <TextField
            control={form.control}
            name="name"
            label={t("buildings.floorName")}
            hint={t("buildings.floorNameHint")}
          />
          <SwitchField
            control={form.control}
            name="withRooms"
            label={t("buildings.createRoomsNow")}
          />
          {v.withRooms && (
            <>
              <div className="grid grid-cols-2 gap-3">
                <TextField
                  control={form.control}
                  name="count"
                  label={t("buildings.roomCount")}
                  inputMode="numeric"
                />
                <TextField control={form.control} name="start" label={t("buildings.startAt")} />
              </div>
              <SegmentField
                control={form.control}
                name="type"
                label={t("buildings.roomType")}
                options={typeOptions(types)}
              />
              {codes.length > 0 && (
                <p className={note}>
                  {tf("buildings.floorInfo", {
                    from: codes[0],
                    to: codes[codes.length - 1],
                    type: typeName,
                  })}
                </p>
              )}
            </>
          )}
          <Actions
            onClose={onClose}
            pending={create.isPending}
            label={
              v.withRooms
                ? tf("buildings.createFloorRooms", { n: Number(v.count) || 0 })
                : t("buildings.createFloor")
            }
          />
        </form>
      </Form>
    </FormSheet>
  );
}

// ---- Add rooms to a floor (ThemPhong) ----
const roomsSchema = z.object({
  mode: z.enum(["one", "range"]),
  from: z.string().trim().min(1, req),
  to: z.string().trim(),
  type: z.string().min(1),
  availableNow: z.boolean(),
});
type RoomsValues = z.infer<typeof roomsSchema>;

export function RoomsForm({
  buildingId,
  buildingName,
  floor,
  floorId,
  nextCode,
  types,
  onClose,
}: {
  buildingId: string;
  buildingName: string;
  floor: number;
  floorId?: string;
  nextCode: string;
  types: Types;
  onClose: () => void;
}) {
  const create = useCreateRooms();
  const [key] = useState(newIdempotencyKey);
  const [features, setFeatures] = useState<RoomFeature[]>([]);
  const form = useForm<RoomsValues>({
    resolver: zodResolver(roomsSchema) as Resolver<RoomsValues>,
    defaultValues: {
      mode: "range",
      from: nextCode,
      to: codeRange(nextCode, 3)[2] ?? nextCode,
      type: types[0]?.code ?? "",
      availableNow: true,
    },
  });
  const v = useWatch({ control: form.control });
  const codes = v.mode === "one" ? [v.from ?? ""] : between(v.from ?? "", v.to ?? "");
  const submit = form.handleSubmit((x) => {
    if (!floorId) return toast.error(t("buildings.noFloorId"));
    if (!codes.length) return form.setError("to", { message: "buildings.rangeInvalid" });
    create.mutate(
      {
        key,
        body: {
          buildingId,
          floorId,
          fromCode: x.from,
          toCode: x.mode === "one" ? x.from : x.to,
          unitTypeCode: x.type,
          features,
          availableNow: x.availableNow,
        },
      },
      {
        onSuccess: () => {
          toast.success(t("buildings.roomsCreated"));
          onClose();
        },
        onError: (e) =>
          toast.error(statusOf(e) === 409 ? t("buildings.codeTaken") : t("buildings.saveFailed")),
      },
    );
  });
  return (
    <FormSheet
      open
      onClose={onClose}
      title={t("buildings.newRooms")}
      description={tf("buildings.roomsWhere", { name: buildingName, n: floor })}
    >
      <Form {...form}>
        <form onSubmit={submit} noValidate className="flex flex-col gap-3 px-4 pb-6">
          <SegmentField
            control={form.control}
            name="mode"
            label={t("buildings.newRooms")}
            options={[
              { value: "one", label: t("buildings.one") },
              { value: "range", label: t("buildings.range") },
            ]}
          />
          <div className="grid grid-cols-2 gap-3">
            <TextField control={form.control} name="from" label={t("buildings.from")} />
            {v.mode === "range" && (
              <TextField control={form.control} name="to" label={t("buildings.to")} />
            )}
          </div>
          {codes.length > 0 && (
            <div className="flex flex-col gap-1.5">
              <b className="text-[13px]">{tf("buildings.willCreateRooms", { n: codes.length })}</b>
              <ul className="flex flex-wrap gap-1.5">
                {codes.slice(0, 12).map((c) => (
                  <li
                    key={c}
                    className="rounded-[10px] border border-ok-line bg-ok-bg px-3 py-1.5 text-[13px] font-bold text-ok"
                  >
                    {c}
                  </li>
                ))}
              </ul>
            </div>
          )}
          <SegmentField
            control={form.control}
            name="type"
            label={t("buildings.roomTypePrice")}
            options={typeOptions(types)}
          />
          <p className="-mt-1 text-[12px] text-muted-foreground">{t("buildings.roomTypeHint")}</p>
          <div className="flex flex-col gap-2">
            <b className="text-[13px]">{t("buildings.features")}</b>
            <ToggleGroup
              type="multiple"
              value={features}
              onValueChange={(f) => setFeatures(f as RoomFeature[])}
              aria-label={t("buildings.features")}
              className="flex flex-wrap justify-start gap-2"
            >
              {FEATURES.map((f) => (
                <ToggleGroupItem
                  key={f}
                  value={f}
                  className={cn(
                    "h-11 rounded-full! border border-border bg-card px-4 text-[14px] font-bold data-[state=on]:border-transparent data-[state=on]:bg-primary data-[state=on]:text-primary-foreground",
                  )}
                >
                  {t(`buildings.feature.${f}` as MessageKey)}
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
            <p className="text-[12px] text-muted-foreground">{t("buildings.featuresHint")}</p>
          </div>
          <SwitchField
            control={form.control}
            name="availableNow"
            label={t("buildings.availableNow")}
            hint={t("buildings.availableHint")}
          />
          {!floorId && (
            <p className="text-[13px] font-bold text-destructive">{t("buildings.noFloorId")}</p>
          )}
          <Actions
            onClose={onClose}
            pending={create.isPending || !floorId}
            label={
              codes.length > 1
                ? tf("buildings.createRooms", { n: codes.length })
                : t("buildings.createOne")
            }
          />
        </form>
      </Form>
    </FormSheet>
  );
}

// ---- Edit room (SuaPhong) ----
const roomSchema = z.object({
  code: z.string().trim().min(1, req),
  type: z.string().min(1),
  maintenance: z.boolean(),
  reason: z.string().trim(),
  backOn: z.string(),
});
type RoomValues = z.infer<typeof roomSchema>;

export function RoomEditForm({
  room,
  buildingName,
  types,
  onClose,
}: {
  room: Room;
  buildingName: string;
  types: Types;
  onClose: () => void;
}) {
  const update = useUpdateRoom();
  const [retiring, setRetiring] = useState(false);
  const form = useForm<RoomValues>({
    resolver: zodResolver(roomSchema) as Resolver<RoomValues>,
    defaultValues: {
      code: room.code,
      type: room.unitType.code,
      maintenance: room.status === "MAINTENANCE",
      reason: room.note ?? "",
      backOn: "",
    },
  });
  const maint = useWatch({ control: form.control, name: "maintenance" });
  const send = (body: Parameters<typeof update.mutate>[0]["body"], typeChanged = false) =>
    update.mutate(
      { id: room.id, body },
      {
        onSuccess: () => {
          toast.success(body.retired ? t("buildings.retired") : t("buildings.saved"));
          onClose();
        },
        onError: (e) =>
          toast.error(
            statusOf(e) === 409
              ? typeChanged || body.retired
                ? t("buildings.occupied")
                : t("buildings.codeTaken")
              : t("buildings.saveFailed"),
          ),
      },
    );
  const submit = form.handleSubmit((x) =>
    send(
      {
        code: x.code,
        unitTypeCode: x.type,
        maintenance: x.maintenance
          ? { on: true, reason: x.reason || null, expectedBackOn: x.backOn || null }
          : room.status === "MAINTENANCE"
            ? { on: false }
            : undefined,
      },
      x.type !== room.unitType.code,
    ),
  );
  return (
    <FormSheet
      open
      onClose={onClose}
      title={tf("buildings.editRoom", { code: room.code })}
      description={`${buildingName} · ${tf("buildings.floor", { n: room.floor })}`}
    >
      <Form {...form}>
        <form onSubmit={submit} noValidate className="flex flex-col gap-3 px-4 pb-6">
          <TextField control={form.control} name="code" label={t("buildings.roomNumber")} />
          <SegmentField
            control={form.control}
            name="type"
            label={t("buildings.roomType")}
            options={typeOptions(types)}
          />
          <h3 className="mt-1 text-[13px] font-bold uppercase tracking-wide text-ink-2">
            {t("buildings.status")}
          </h3>
          <div className="flex flex-col gap-3 rounded-card border border-border p-3.5">
            <SwitchField
              control={form.control}
              name="maintenance"
              label={t("buildings.maintenance")}
              hint={t("buildings.maintenanceHint")}
            />
            {maint && (
              <>
                <TextField control={form.control} name="reason" label={t("buildings.reason")} />
                <TextField
                  control={form.control}
                  name="backOn"
                  label={t("buildings.backOn")}
                  type="date"
                />
              </>
            )}
          </div>
          <p className="text-[12px] text-muted-foreground">{t("buildings.roomNote")}</p>
          <Actions onClose={onClose} pending={update.isPending} label={t("buildings.save")} />
          <Button
            type="button"
            variant="outline"
            size="lg"
            className="border-destructive/40 font-bold text-destructive hover:bg-destructive/10"
            onClick={() => setRetiring(true)}
          >
            {t("buildings.retire")}
          </Button>
        </form>
      </Form>
      {retiring && (
        <FormSheet
          open
          onClose={() => setRetiring(false)}
          title={tf("buildings.retireTitle", { code: room.code })}
          description={t("buildings.retireBody")}
        >
          <div className="flex flex-col gap-2 px-4 pb-6">
            <p className="text-[14px] text-ink-2">{t("buildings.retireBody")}</p>
            <Button
              size="lg"
              variant="outline"
              className="border-destructive/40 font-bold text-destructive hover:bg-destructive/10"
              disabled={update.isPending}
              onClick={() => send({ retired: true })}
            >
              {t("buildings.retireConfirm")}
            </Button>
            <Button size="lg" variant="outline" onClick={() => setRetiring(false)}>
              {t("buildings.cancel")}
            </Button>
          </div>
        </FormSheet>
      )}
    </FormSheet>
  );
}
