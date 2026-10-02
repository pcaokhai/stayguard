"use client";

import { useRef, useState } from "react";
import { toast } from "sonner";
import { ResponsiveDialog } from "@/components/ResponsiveDialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { newIdempotencyKey } from "../../lib/api";
import { t, tf, type MessageKey } from "../../lib/t";
import { clockLocale } from "../../lib/time";
import { today } from "../history/dates";
import { dayMonth, leaveTitle, shiftName } from "./format";
import { useCancelLeave, type LeaveRequest } from "./hooks";

const STATUS_VARIANT = {
  PENDING: "warn",
  APPROVED: "ok",
  DECLINED: "overdue",
  CANCELLED: "maintenance",
  CANCEL_REQUESTED: "warn",
  TAKEN: "maintenance",
} as const;

const stamp = (iso: string) =>
  new Intl.DateTimeFormat(clockLocale(), {
    day: "2-digit",
    month: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(iso));

// The meta line under each request: when sent, when approved, why declined, or that it was taken.
function meta(r: LeaveRequest): string {
  if (r.status === "APPROVED")
    return r.decidedAt ? tf("leave.approvedOn", { date: dayMonth(r.decidedAt.slice(0, 10)) }) : "";
  if (r.status === "DECLINED") return tf("leave.declined", { reason: r.declineReason ?? "" });
  if (r.status === "TAKEN") return t("leave.taken");
  if (r.status === "CANCEL_REQUESTED") return t("leave.cancelSent");
  return tf("leave.sent", { time: stamp(r.createdAt) });
}

export function LeaveList({
  items,
  filter = "all",
}: {
  items: LeaveRequest[];
  filter?: "all" | "upcoming" | "past";
}) {
  const cancel = useCancelLeave();
  const [target, setTarget] = useState<LeaveRequest | null>(null);
  // One key per request: a retried tap on the same request reuses it.
  const keys = useRef(new Map<string, string>());
  const now = today();
  const shown = items.filter((r) =>
    filter === "all" ? true : filter === "upcoming" ? r.toDate >= now : r.toDate < now,
  );

  const confirm = () => {
    if (!target) return;
    const pending = target.status === "PENDING";
    if (!keys.current.has(target.id)) keys.current.set(target.id, newIdempotencyKey());
    cancel.mutate(
      { leaveId: target.id, key: keys.current.get(target.id)! },
      {
        onSuccess: () => {
          toast.success(t(pending ? "leave.cancelled" : "leave.askSent"));
          setTarget(null);
        },
      },
    );
  };

  if (!shown.length)
    return <p className="py-6 text-center text-muted-foreground">{t("leave.empty")}</p>;
  const date = target ? dayMonth(target.fromDate) : "";
  return (
    <>
      <ul className="grid gap-3 lg:grid-cols-2">
        {shown.map((r) => (
          <li key={r.id}>
            <Card className="h-full gap-1.5 p-4 shadow-none">
              <div className="flex items-start justify-between gap-3">
                <h3 className="text-[17px] font-bold">{leaveTitle(r)}</h3>
                <Badge variant={STATUS_VARIANT[r.status]} className="px-3 py-1 font-bold">
                  {t(`leave.${r.status}` as MessageKey)}
                </Badge>
              </div>
              <p className="text-sm">
                {[t(`leave.${r.kind}` as MessageKey), r.reason].filter(Boolean).join(" · ")}
              </p>
              <p className="text-[13px] text-muted-foreground">{meta(r)}</p>
              {(r.status === "PENDING" || r.status === "APPROVED") && (
                <Button
                  type="button"
                  variant="outline"
                  className="mt-1.5 w-full lg:w-fit"
                  onClick={() => setTarget(r)}
                >
                  {t(r.status === "PENDING" ? "leave.cancel" : "leave.askCancel")}
                </Button>
              )}
            </Card>
          </li>
        ))}
      </ul>
      <ResponsiveDialog
        open={!!target}
        onOpenChange={(o) => !o && setTarget(null)}
        title={tf(target?.status === "PENDING" ? "leave.confirmTitle" : "leave.confirmAskTitle", {
          date,
        })}
        description={
          target?.status === "PENDING"
            ? tf("leave.confirmPending", {
                date,
                shift: target.shift ? shiftName(target.shift).toLowerCase() : t("leave.wholeDay"),
              })
            : t("leave.confirmApproved")
        }
      >
        {cancel.isError && (
          <p role="alert" className="text-sm text-warn">
            {t("leave.actionFailed")}
          </p>
        )}
        <Button
          variant="outline"
          loading={cancel.isPending}
          onClick={confirm}
          className="border-destructive/30 text-destructive hover:text-destructive"
        >
          {t(target?.status === "PENDING" ? "leave.cancel" : "leave.askConfirm")}
        </Button>
        <Button variant="outline" onClick={() => setTarget(null)}>
          {t("leave.keep")}
        </Button>
      </ResponsiveDialog>
    </>
  );
}
