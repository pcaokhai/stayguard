"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useForm, useWatch, type Resolver } from "react-hook-form";
import { toast } from "sonner";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { Form } from "@/components/ui/form";
import { formatVnd, parseVnd, vndNumber } from "@/lib/money";
import { t, tf, type MessageKey } from "@/lib/t";
import { cn } from "@/lib/utils";
import { clockOf, formatDayMonth } from "../format";
import { FormSheet, SegmentField, SwitchField, TextField } from "../FormFields";
import { messageFor } from "../../problem/problem";
import { type Ticket, useUpdateTicket } from "./hooks";

const money = z
  .string()
  .trim()
  .regex(/^[\d.,\s]*$/, "maint.required");
const schema = z.object({
  status: z.enum(["NEW", "IN_REPAIR", "DONE"]),
  roomLocked: z.boolean(),
  expectedDoneOn: z.string(),
  partsCost: money,
  labourCost: money,
  repairer: z.string().trim(),
});
type Values = z.infer<typeof schema>;

export const STATUS_TONE: Record<Ticket["status"], string> = {
  NEW: "border-dirty-line bg-dirty-bg text-dirty",
  IN_REPAIR: "border-info-line bg-info-bg text-info",
  DONE: "border-ok-line bg-ok-bg text-ok",
};
export const StatusPill = ({ s }: { s: Ticket["status"] }) => (
  <span
    className={cn(
      "inline-block rounded-full border px-2.5 py-0.5 text-[12px] font-bold whitespace-nowrap",
      STATUS_TONE[s],
    )}
  >
    {t(`maint.st.${s}` as MessageKey)}
  </span>
);

// Right sheet; the owner sets status, lock, costs and repairer (boards BaoTriChiTiet, BaoTriChiTietPC).
export function TicketSheet({ ticket, onClose }: { ticket: Ticket; onClose: () => void }) {
  const update = useUpdateTicket();
  const form = useForm<Values>({
    resolver: zodResolver(schema) as Resolver<Values>,
    defaultValues: {
      status: ticket.status,
      roomLocked: ticket.roomLocked,
      expectedDoneOn: ticket.expectedDoneOn ?? "",
      partsCost: ticket.partsCost != null ? vndNumber(ticket.partsCost) : "",
      labourCost: ticket.labourCost != null ? vndNumber(ticket.labourCost) : "",
      repairer: ticket.repairer ?? "",
    },
  });
  const v = useWatch({ control: form.control });
  // Shown while typing; the saved total comes back from the API.
  const sum = parseVnd(v.partsCost ?? "") + parseVnd(v.labourCost ?? "");
  const submit = form.handleSubmit((x) =>
    update.mutate(
      {
        id: ticket.id,
        body: {
          status: x.status,
          roomLocked: x.status === "DONE" ? false : x.roomLocked,
          expectedDoneOn: x.expectedDoneOn || null,
          partsCost: x.partsCost ? parseVnd(x.partsCost) : null,
          labourCost: x.labourCost ? parseVnd(x.labourCost) : null,
          repairer: x.repairer || null,
        },
      },
      {
        onSuccess: () => {
          toast.success(t("maint.saved"));
          onClose();
        },
        onError: (e) =>
          toast.error(
            messageFor(e, {
              ROOM_OCCUPIED: "maint.occupied",
              TICKET_DONE: "maint.ticketDone",
              TICKET_STATUS: "maint.statusBad",
              VALIDATION_FAILED: "maint.saveFailed",
            }),
          ),
      },
    ),
  );
  const at = `${formatDayMonth(ticket.reportedAt)} ${clockOf(ticket.reportedAt)}`;
  return (
    <FormSheet
      open
      onClose={onClose}
      title={`${ticket.code} · ${ticket.roomCode}`}
      description={t("maint.ticket")}
    >
      <Form {...form}>
        <form onSubmit={submit} noValidate className="flex flex-col gap-3 px-4 pb-6">
          <div className="flex items-center justify-between">
            <StatusPill s={ticket.status} />
          </div>
          <div className="rounded-card bg-sunken p-3.5 text-[13px]">
            <span className="text-ink-2">
              {tf("maint.reported", { who: ticket.reportedBy, at, text: ticket.description })}
            </span>
          </div>
          <SegmentField
            control={form.control}
            name="status"
            label={t("maint.statusLabel")}
            options={(["NEW", "IN_REPAIR", "DONE"] as const).map((s) => ({
              value: s,
              label: t(`maint.st.${s}` as MessageKey),
            }))}
          />
          <SwitchField
            control={form.control}
            name="roomLocked"
            label={t("maint.lockRoom")}
            hint={t("maint.lockHint")}
          />
          <TextField
            control={form.control}
            name="expectedDoneOn"
            label={t("maint.expectedDone")}
            type="date"
          />
          <div className="grid grid-cols-2 gap-3">
            <TextField
              control={form.control}
              name="partsCost"
              label={t("maint.parts")}
              suffix="đ"
              inputMode="numeric"
            />
            <TextField
              control={form.control}
              name="labourCost"
              label={t("maint.labour")}
              suffix="đ"
              inputMode="numeric"
            />
          </div>
          <TextField control={form.control} name="repairer" label={t("maint.repairer")} />
          <p className="flex justify-between text-[14px] text-ink-2">
            {t("maint.total")}
            <b className="text-ink">{formatVnd(sum)}</b>
          </p>
          <p className="rounded-card border border-info-line bg-info-bg p-3 text-[13px] text-info">
            {t("maint.info")}
          </p>
          <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            <Button type="button" variant="outline" size="lg" onClick={onClose}>
              {t("maint.cancel")}
            </Button>
            <Button type="submit" size="lg" disabled={update.isPending}>
              {t("maint.save")}
            </Button>
          </div>
        </form>
      </Form>
    </FormSheet>
  );
}
