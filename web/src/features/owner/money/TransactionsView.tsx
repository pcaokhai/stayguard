"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { Download, Lock, Search } from "lucide-react";
import type { components } from "@/api/generated/schema";
import { FadeIn, SlidingPill, StaggerList } from "@/components/motion";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { QueryError } from "@/components/StateView";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { formatVnd } from "@/lib/money";
import { t, tf, type MessageKey } from "@/lib/t";
import { cn } from "@/lib/utils";
import { useMe } from "../../session/useMe";
import { clockOf } from "../format";
import { useTransactions } from "./hooks";
import { LinkSheet } from "./LinkSheet";
import {
  amountLine,
  inTab,
  methodLine,
  needsAction,
  settledNote,
  shownTime,
  type Tab,
  type Tx,
} from "./rows";

const TABS: Tab[] = ["all", "transfer", "cash", "needs"];

const TONE: Record<Tx["reconciliation"], string> = {
  MATCHED: "border-ok-line bg-ok-bg text-ok",
  MISMATCH: "border-warn-line bg-warn-bg text-warn-ink",
  UNMATCHED: "border-dirty-line bg-dirty-bg text-dirty",
  CASH: "border-line bg-maint-bg text-maint",
};
function Pill({ x }: { x: Tx }) {
  return (
    <span
      className={cn(
        "inline-block rounded-full border px-2.5 py-0.5 text-[13px] font-bold whitespace-nowrap",
        TONE[x.reconciliation],
      )}
    >
      {t(`money.${x.reconciliation}` as MessageKey)}
    </span>
  );
}
const roomBill = (x: Tx) => `${x.roomCode || "—"} · ${x.billCode || "—"}`;

export function TransactionsView() {
  const [tab, setTab] = useState<Tab>("all");
  const [text, setText] = useState("");
  const [q, setQ] = useState("");
  const [linking, setLinking] = useState<Tx | null>(null);
  useEffect(() => {
    const id = setTimeout(() => setQ(text.trim()), 300);
    return () => clearTimeout(id);
  }, [text]);
  const me = useMe();
  const canLink = me.data?.user.role === "OWNER";
  const tx = useTransactions(q);
  const all = tx.data ?? [];
  const rows = all.filter((x) => inTab(x, tab));
  const needs = all.filter(needsAction).length;

  if (tx.isError)
    return (
      <AppFrame tabs={false}>
        <QueryError onRetry={() => void tx.refetch()} />
      </AppFrame>
    );

  const linkButton = (x: Tx, className?: string) =>
    canLink &&
    x.reconciliation === "UNMATCHED" &&
    x.paymentEventId && (
      <Button variant="outline" size="lg" className={className} onClick={() => setLinking(x)}>
        {t("money.link")}
      </Button>
    );

  const csv = (
    <Button variant="outline" size="lg" disabled={!rows.length} onClick={() => downloadCsv(rows)}>
      <Download aria-hidden="true" />
      <span className="hidden lg:inline">{t("money.csv")}</span>
      <span className="sr-only lg:hidden">{t("money.csv")}</span>
    </Button>
  );

  return (
    <AppFrame tabs={false}>
      <main className="mx-auto flex w-full max-w-[1280px] flex-col gap-3 pb-10 lg:px-3">
        <TopBar title={t("money.title")} subtitle={t("money.sub")} back="/owner" right={csv} />
        <div className="flex flex-col gap-3 px-5">
          <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
            <ToggleGroup
              type="single"
              value={tab}
              onValueChange={(v) => v && setTab(v as Tab)}
              aria-label={t("money.title")}
              className="flex flex-wrap justify-start gap-2"
            >
              {TABS.map((k) => (
                <ToggleGroupItem
                  key={k}
                  value={k}
                  className="relative h-11 shrink-0 rounded-full! border border-border bg-card px-4 text-[15px] font-bold data-[state=on]:border-transparent data-[state=on]:bg-transparent data-[state=on]:text-primary-foreground"
                >
                  {tab === k && (
                    <SlidingPill
                      id="tx-pill"
                      className="absolute inset-0 rounded-full bg-primary"
                    />
                  )}
                  <span className="relative whitespace-nowrap">
                    {k === "needs"
                      ? tf("money.needs", { n: needs })
                      : t(`money.${k}` as MessageKey)}
                  </span>
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
            <div className="relative lg:w-[340px]">
              <Search
                className="pointer-events-none absolute left-3.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
                aria-hidden="true"
              />
              <Input
                value={text}
                onChange={(e) => setText(e.target.value)}
                placeholder={t("money.search")}
                aria-label={t("money.search")}
                className="h-11 rounded-full bg-card pl-10"
              />
            </div>
          </div>

          {tx.isLoading && <Skeleton className="h-64 rounded-card" aria-busy="true" />}
          {!tx.isLoading && rows.length === 0 && (
            <p className="py-10 text-center text-muted-foreground">{t("money.empty")}</p>
          )}

          <StaggerList className="flex flex-col gap-2.5 md:hidden">
            {rows.map((x) => (
              <Card key={x.id} className="gap-1.5 p-4 shadow-none">
                <div className="flex items-center justify-between gap-2">
                  <b className={cn("text-[19px]", x.amount < 0 && "text-destructive")}>
                    {amountLine(x)}
                  </b>
                  <Pill x={x} />
                </div>
                <p className="flex justify-between text-[14px] text-ink-2">
                  <span>
                    {shownTime(x)} · {methodLine(x)}
                  </span>
                  <span>{roomBill(x)}</span>
                </p>
                {x.reconciliation === "UNMATCHED" && x.transferNote && (
                  <p className="text-[13px] text-ink-2">
                    {tf("money.noteText", { note: x.transferNote })}
                  </p>
                )}
                {linkButton(x, "mt-1 w-full font-bold")}
              </Card>
            ))}
          </StaggerList>

          {rows.length > 0 && (
            <FadeIn className="hidden md:block">
              <Card className="gap-0 overflow-hidden p-0 shadow-none">
                <table className="w-full text-[14px]">
                  <thead>
                    <tr className="h-11 text-left text-[12px] uppercase tracking-wide text-ink-2">
                      {(["time", "amount", "roomBill", "recon", "note"] as const).map((k) => (
                        <th key={k} scope="col" className="px-3 font-bold first:pl-5">
                          {t(`money.${k}`)}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {rows.map((x) => (
                      <tr
                        key={x.id}
                        className="h-[54px] border-t border-border transition-colors duration-100 hover:bg-sunken/50"
                      >
                        <td className="px-3 pl-5 leading-tight">
                          {shownTime(x)}
                          {settledNote(x) && (
                            <span className="block text-[12px] text-muted-foreground">
                              {settledNote(x)}
                            </span>
                          )}
                        </td>
                        <td className="px-3">
                          <b
                            className={cn("block text-[15px]", x.amount < 0 && "text-destructive")}
                          >
                            {amountLine(x)}
                          </b>
                          <span className="text-[12px] text-muted-foreground">{methodLine(x)}</span>
                        </td>
                        <td className="px-3 font-mono text-[13px]">{roomBill(x)}</td>
                        <td className="px-3">
                          <Pill x={x} />
                        </td>
                        <td className="px-3 pr-5">
                          <span className="flex items-center justify-between gap-3 text-ink-2">
                            {x.reconciliation === "UNMATCHED" && x.transferNote
                              ? tf("money.noteText", { note: x.transferNote })
                              : ""}
                            {linkButton(x)}
                          </span>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </Card>
            </FadeIn>
          )}
          <p className="flex items-start gap-2 text-[13px] text-muted-foreground">
            <Lock className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
            <span className="md:hidden">{t("money.footPhone")}</span>
            <span className="hidden md:inline">{t("money.footPc")}</span>
          </p>
        </div>
      </main>
      <LinkSheet key={linking?.id} tx={linking} onClose={() => setLinking(null)} />
    </AppFrame>
  );
}

function downloadCsv(rows: Tx[]) {
  const cell = (v: string) => `"${v.replace(/"/g, '""')}"`;
  const lines = rows.map((x) =>
    [x.at, String(x.amount), x.method, x.roomCode ?? "", x.billCode ?? "", x.reconciliation]
      .map(cell)
      .join(","),
  );
  const blob = new Blob(["﻿" + lines.join("\n")], { type: "text/csv;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  Object.assign(document.createElement("a"), { href: url, download: "transactions.csv" }).click();
  URL.revokeObjectURL(url);
}
