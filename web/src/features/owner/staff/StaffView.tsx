"use client";

import Link from "next/link";
import { useRef, useState } from "react";
import { KeyRound, Lock, LockOpen, Pencil, Plus, Shield, Trash2, User } from "lucide-react";
import { toast } from "sonner";
import { FadeIn, SlidingPill, StaggerList } from "@/components/motion";
import { AppFrame } from "@/components/shell/AppFrame";
import { isReady } from "@/components/shell/nav";
import { TopBar } from "@/components/shell/TopBar";
import { EmptyState, QueryError } from "@/components/StateView";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { newIdempotencyKey } from "@/lib/api";
import { lp } from "@/lib/locale";
import { t, tf, type MessageKey } from "@/lib/t";
import { cn } from "@/lib/utils";
import { useMe } from "../../session/useMe";
import { clockOf } from "../format";
import { useLockToggle, useResetPin, useStaff, type Staff } from "./hooks";
import { PinDialog, type PinReveal } from "./PinDialog";
import { RemoveDialog } from "./RemoveDialog";
import { StaffForm } from "./StaffForm";

const FILTERS = ["ALL", "FRONT_DESK", "HOUSEKEEPING", "MANAGER", "SECURITY"] as const;
type Filter = (typeof FILTERS)[number];
const pill =
  "inline-block rounded-full border px-2.5 py-0.5 text-[12px] font-bold whitespace-nowrap";

function StatusPill({ s }: { s: Staff }) {
  const tone =
    s.appAccess === "NONE"
      ? "border-line bg-maint-bg text-maint"
      : s.status === "LOCKED"
        ? "border-warn-line bg-warn-bg text-warn-ink"
        : "border-ok-line bg-ok-bg text-ok";
  return (
    <span className={cn(pill, tone)}>
      {s.appAccess === "NONE" ? t("staff.noApp") : t(`staff.status.${s.status}` as MessageKey)}
    </span>
  );
}

function lastLine(s: Staff) {
  if (s.appAccess === "NONE") return t("staff.payrollOnly");
  if (s.status === "LOCKED" && s.lockedUntil)
    return tf("staff.lockedUntil", { time: clockOf(s.lockedUntil) });
  return s.lastActivityAt
    ? tf("staff.lastAt", { time: clockOf(s.lastActivityAt) })
    : t("staff.neverSignedIn");
}

export function StaffView() {
  const me = useMe();
  const isOwner = me.data?.user.role === "OWNER";
  const q = useStaff();
  const [filter, setFilter] = useState<Filter>("ALL");
  const [form, setForm] = useState<{ staff?: Staff } | null>(null);
  const [removing, setRemoving] = useState<Staff | null>(null);
  const [reveal, setReveal] = useState<PinReveal | null>(null);
  const reset = useResetPin();
  const lock = useLockToggle();
  const keys = useRef(new Map<string, string>());

  const all = (q.data ?? []).filter((s) => s.status !== "REMOVED");
  const rows = all.filter((s) => filter === "ALL" || s.position === filter);
  const noApp = all.filter((s) => s.appAccess === "NONE").length;

  if (q.isError)
    return (
      <AppFrame tabs={false}>
        <QueryError onRetry={() => void q.refetch()} />
      </AppFrame>
    );

  const doReset = (s: Staff) => {
    const key = keys.current.get(s.id) ?? newIdempotencyKey();
    keys.current.set(s.id, key);
    reset.mutate(
      { userId: s.id, key },
      {
        onSuccess: (pin) => {
          keys.current.delete(s.id);
          setReveal({ pin, name: s.name, username: s.username, created: false });
        },
        onError: () => toast.error(t("staff.actionFailed")),
      },
    );
  };
  const doLock = (s: Staff) =>
    lock.mutate(
      { userId: s.id, lock: s.status !== "LOCKED" },
      {
        onSuccess: () =>
          toast.success(s.status === "LOCKED" ? t("staff.unlocked") : t("staff.locked")),
        onError: () => toast.error(t("staff.actionFailed")),
      },
    );

  const actions = (s: Staff, wide: boolean) => (
    <div className={cn("flex gap-2", wide ? "justify-end" : "flex-col")}>
      <div className="flex gap-2">
        {s.appAccess !== "NONE" && (
          <Button
            variant="outline"
            size="lg"
            className="flex-1 font-bold"
            disabled={reset.isPending}
            onClick={() => doReset(s)}
          >
            <KeyRound aria-hidden="true" />
            {t("staff.resetPin")}
          </Button>
        )}
        {isOwner && wide && (
          <Button
            variant="outline"
            size="icon-lg"
            aria-label={`${t("staff.editShort")} ${s.name}`}
            onClick={() => setForm({ staff: s })}
          >
            <Pencil aria-hidden="true" />
          </Button>
        )}
        {s.appAccess !== "NONE" && (
          <Button
            variant="outline"
            size={wide ? "icon-lg" : "lg"}
            className={wide ? undefined : "flex-1 font-bold"}
            aria-label={`${s.status === "LOCKED" ? t("staff.unlock") : t("staff.lock")} ${s.name}`}
            onClick={() => doLock(s)}
          >
            {s.status === "LOCKED" ? <LockOpen aria-hidden="true" /> : <Lock aria-hidden="true" />}
            {!wide && (s.status === "LOCKED" ? t("staff.unlock") : t("staff.lock"))}
          </Button>
        )}
        {isOwner && wide && (
          <Button
            variant="outline"
            size="icon-lg"
            aria-label={`${t("staff.remove")} ${s.name}`}
            className="border-destructive/40 text-destructive hover:bg-destructive/10"
            onClick={() => setRemoving(s)}
          >
            <Trash2 aria-hidden="true" />
          </Button>
        )}
      </div>
      {isOwner && !wide && (
        <>
          <Button variant="outline" size="lg" onClick={() => setForm({ staff: s })}>
            <Pencil aria-hidden="true" />
            {t("staff.edit")}
          </Button>
          <Button
            variant="outline"
            size="lg"
            className="border-destructive/40 font-bold text-destructive hover:bg-destructive/10"
            onClick={() => setRemoving(s)}
          >
            <Trash2 aria-hidden="true" />
            {t("staff.remove")}
          </Button>
        </>
      )}
    </div>
  );

  const add = isOwner && (
    <Button size="lg" className="w-full md:w-auto" onClick={() => setForm({})}>
      <Plus aria-hidden="true" />
      {t("staff.add")}
    </Button>
  );

  return (
    <AppFrame tabs={false}>
      <main className="mx-auto flex w-full max-w-[1280px] flex-col gap-3 pb-10 lg:px-3">
        <TopBar
          title={t("staff.title")}
          subtitle={
            noApp
              ? tf("staff.sub", { n: all.length, m: noApp })
              : tf("staff.subPlain", { n: all.length })
          }
          back="/owner"
          right={<span className="hidden md:block">{add}</span>}
        />
        <div className="flex flex-col gap-3 px-5">
          <ToggleGroup
            type="single"
            value={filter}
            onValueChange={(v) => v && setFilter(v as Filter)}
            aria-label={t("staff.title")}
            className="hidden flex-wrap justify-start gap-2 md:flex"
          >
            {FILTERS.map((f) => (
              <ToggleGroupItem
                key={f}
                value={f}
                className="relative h-11 rounded-full! border border-border bg-card px-4 text-[15px] font-bold data-[state=on]:border-transparent data-[state=on]:bg-transparent data-[state=on]:text-primary-foreground"
              >
                {filter === f && (
                  <SlidingPill
                    id="staff-pill"
                    className="absolute inset-0 rounded-full bg-primary"
                  />
                )}
                <span className="relative whitespace-nowrap">
                  {f === "ALL"
                    ? tf("staff.all", { n: all.length })
                    : t(`staff.position.${f}` as MessageKey)}
                </span>
              </ToggleGroupItem>
            ))}
          </ToggleGroup>

          {q.isLoading && <Skeleton className="h-64 rounded-card" aria-busy="true" />}
          {!q.isLoading && rows.length === 0 && (
            <EmptyState title={t("staff.title")} body={t("staff.empty")} />
          )}

          <StaggerList className="flex flex-col gap-3 md:hidden">
            {rows.map((s) => (
              <Card key={s.id} className="gap-2.5 p-3.5 shadow-none">
                <div className="flex items-center gap-3">
                  <span
                    className="flex size-10 items-center justify-center rounded-full bg-sunken"
                    aria-hidden="true"
                  >
                    <User className="size-5 text-brand" />
                  </span>
                  <div className="min-w-0 flex-1 leading-tight">
                    <b className="block text-[17px]">{s.name}</b>
                    <span className="text-[13px] text-ink-2">
                      {t(`staff.position.${s.position}` as MessageKey)} · {s.username ?? "—"}
                    </span>
                  </div>
                  <StatusPill s={s} />
                </div>
                <p className="text-[13px] text-ink-2">{lastLine(s)}</p>
                {actions(s, false)}
              </Card>
            ))}
          </StaggerList>

          {rows.length > 0 && (
            <FadeIn className="hidden md:block">
              <Card className="gap-0 overflow-x-auto p-0 shadow-none">
                <table className="w-full min-w-[820px] text-[14px]">
                  <thead>
                    <tr className="h-11 text-left text-[12px] uppercase tracking-wide text-ink-2">
                      {(["colStaff", "colPosition", "colStatus", "colLast"] as const).map((k) => (
                        <th key={k} scope="col" className="px-3 font-bold first:pl-5">
                          {t(`staff.${k}`)}
                        </th>
                      ))}
                      <th scope="col" className="sr-only">
                        {t("staff.title")}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {rows.map((s) => (
                      <tr
                        key={s.id}
                        className="border-t border-border transition-colors duration-100 hover:bg-sunken/50"
                      >
                        <td className="px-3 py-3 pl-5 leading-tight">
                          <b className="block text-[15px]">{s.name}</b>
                          <span className="font-mono text-[12px] text-muted-foreground">
                            {s.username ?? t("staff.noLogin")}
                          </span>
                        </td>
                        <td className="px-3">{t(`staff.position.${s.position}` as MessageKey)}</td>
                        <td className="px-3">
                          <StatusPill s={s} />
                        </td>
                        <td className="px-3 text-ink-2">{lastLine(s)}</td>
                        <td className="px-3 pr-5">{actions(s, true)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </Card>
            </FadeIn>
          )}

          <div className="flex flex-wrap items-center gap-3">
            {isReady("/owner/access") && isOwner && (
              <Button asChild variant="outline" size="lg" className="font-bold">
                <Link href={lp("/owner/access")}>
                  <Shield aria-hidden="true" />
                  {t("staff.access")}
                </Link>
              </Button>
            )}
            {isReady("/owner/roster") && (
              <Button asChild variant="outline" size="lg" className="font-bold">
                <Link href={lp("/owner/roster")}>{t("staff.roster")}</Link>
              </Button>
            )}
            <p className="min-w-[240px] flex-1 text-[13px] text-muted-foreground">
              <span className="md:hidden">{t("staff.footPhone")}</span>
              <span className="hidden md:inline">{t("staff.footPc")}</span>
            </p>
          </div>
          <div className="md:hidden">{add}</div>
        </div>
      </main>
      {form && (
        <StaffForm
          key={form.staff?.id ?? "new"}
          open
          staff={form.staff}
          onClose={() => setForm(null)}
          onReveal={setReveal}
        />
      )}
      <RemoveDialog key={removing?.id} staff={removing} onClose={() => setRemoving(null)} />
      <PinDialog reveal={reveal} onClose={() => setReveal(null)} />
    </AppFrame>
  );
}
