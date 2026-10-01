"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { useState, type FormEvent } from "react";
import { ScreenHeader } from "../../components/ScreenHeader";
import { newIdempotencyKey } from "../../lib/api";
import { parseVnd, vndNumber } from "../../lib/money";
import { t } from "../../lib/t";
import { useBuildings } from "../rooms/hooks";
import { useCreateStay, useRoom } from "./hooks";
import { rentalLabel, type RentalType } from "./labels";

const RENTAL_TYPES: RentalType[] = ["HOURLY", "OVERNIGHT", "DAILY"];
const DEFAULT_DEPOSIT = "100000";
const input = "h-11 w-full rounded-[10px] border border-line bg-surface px-4 text-base";

export function CheckinForm() {
  const roomId = useSearchParams().get("room");
  const router = useRouter();
  const room = useRoom(roomId);
  const building = useBuildings().data?.find((b) => b.id === room.data?.buildingId);
  const create = useCreateStay(roomId ?? "");
  const [rentalType, setRentalType] = useState<RentalType>("HOURLY");
  const [guestName, setGuestName] = useState("");
  const [guestPhone, setGuestPhone] = useState("");
  const [deposit, setDeposit] = useState(DEFAULT_DEPOSIT);
  // One key per user action: a retry after a failure reuses it.
  const [key] = useState(newIdempotencyKey);

  const submit = (e: FormEvent) => {
    e.preventDefault();
    create.mutate(
      { key, body: { rentalType, guestName, guestPhone, deposit: parseVnd(deposit) } },
      { onSuccess: (stay) => router.replace(`/vi/stay?id=${stay.id}`) },
    );
  };

  return (
    <main className="mx-auto flex min-h-dvh max-w-[480px] flex-col">
      <ScreenHeader
        title={`${t("stay.checkinTitle")} ${room.data?.code ?? ""}`}
        subtitle={[building?.name, room.data?.unitType.name.vi].filter(Boolean).join(" · ")}
      />
      <form onSubmit={submit} className="flex flex-1 flex-col gap-4 px-5 pb-8">
        <fieldset className="flex flex-col gap-2">
          <legend className="mb-2 text-base font-semibold">{t("stay.rentalType")}</legend>
          {RENTAL_TYPES.map((r) => (
            <label
              key={r}
              className={`flex min-h-14 items-center rounded-card bg-surface px-4 font-semibold ${
                rentalType === r ? "border-2 border-brand bg-brand/10" : "border border-line"
              }`}
            >
              <input
                type="radio"
                name="rental"
                className="sr-only"
                checked={rentalType === r}
                onChange={() => setRentalType(r)}
              />
              {rentalLabel(r)}
            </label>
          ))}
        </fieldset>
        <label className="flex flex-col gap-1.5 font-semibold">
          {t("stay.guestName")}
          <input
            required
            maxLength={120}
            value={guestName}
            onChange={(e) => setGuestName(e.target.value)}
            className={input}
          />
        </label>
        <label className="flex flex-col gap-1.5 font-semibold">
          {t("stay.guestPhone")}
          <input
            required
            minLength={6}
            maxLength={20}
            inputMode="tel"
            value={guestPhone}
            onChange={(e) => setGuestPhone(e.target.value)}
            className={input}
          />
        </label>
        <label className="flex flex-col gap-1.5 font-semibold">
          {t("stay.deposit")}
          <input
            inputMode="numeric"
            value={deposit}
            onChange={(e) => setDeposit(e.target.value)}
            className={input}
          />
        </label>
        <p className="rounded-[10px] bg-sunken p-3 text-[13px] text-ink-2">
          {t("stay.serverClock")}
        </p>
        {create.isError && (
          <p role="alert" className="text-sm text-warn">
            {t("stay.checkinFailed")}
          </p>
        )}
        <button
          type="submit"
          disabled={create.isPending || !room.data}
          className="mt-auto h-14 whitespace-nowrap rounded-card bg-brand text-lg font-bold text-white disabled:opacity-60"
        >
          {t("stay.confirmCheckin")}
        </button>
      </form>
    </main>
  );
}
