"use client";

import { useState } from "react";
import { Tag } from "lucide-react";
import { toast } from "sonner";
import { FadeIn, SlidingPill } from "@/components/motion";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { QueryError } from "@/components/StateView";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { localized } from "@/lib/locale";
import { formatVnd, parseVnd, vndNumber } from "@/lib/money";
import { t, tf } from "@/lib/t";
import { cn } from "@/lib/utils";
import { formatDayMonth } from "../format";
import { useDebounced } from "../useDebounced";
import {
  useRatePlans,
  useSaveRatePlan,
  usePricePreview,
  type RatePlan,
  type UnitTypeRates,
} from "./hooks";

type Draft = {
  firstHour: string;
  extraHour: string;
  overnight: string;
  daily: string;
  grace: string;
};
const draftOf = (p: RatePlan): Draft => ({
  firstHour: vndNumber(p.hourly.firstHour),
  extraHour: vndNumber(p.hourly.extraHour),
  overnight: vndNumber(p.overnight.price),
  daily: vndNumber(p.daily.price),
  grace: String(p.graceMinutes),
});
const valid = (d: Draft) =>
  [d.firstHour, d.extraHour, d.overnight, d.daily, d.grace].every((v) => /\d/.test(v));
// Merges the edited numbers into the server's plan; windows stay as the server has them.
const planOf = (base: RatePlan, d: Draft): RatePlan => ({
  graceMinutes: Number(d.grace.replace(/\D/g, "")),
  hourly: { firstHour: parseVnd(d.firstHour), extraHour: parseVnd(d.extraHour) },
  overnight: { ...base.overnight, price: parseVnd(d.overnight) },
  daily: { ...base.daily, price: parseVnd(d.daily) },
});

// Fixed stays used for the live check; the server prices them (domain/pricing), the browser never does.
const DAY = "2026-01-15";
const at = (day: string, hm: string) => `${day}T${hm}:00+07:00`;
const SCENARIOS = [
  { key: "pHour1", type: "HOURLY", from: at(DAY, "10:00"), to: at(DAY, "11:10") },
  { key: "pHour2", type: "HOURLY", from: at(DAY, "10:00"), to: at(DAY, "12:35") },
  { key: "pNight", type: "OVERNIGHT", from: at(DAY, "19:30"), to: at("2026-01-16", "11:00") },
  { key: "pDay", type: "HOURLY", from: at(DAY, "10:00"), to: at("2026-01-16", "00:00") },
] as const;

function Preview({ plan, title }: { plan: RatePlan | null; title: string }) {
  const p = useDebounced(plan);
  const q = SCENARIOS.map((s) => usePricePreview(p, s.type, s.from, s.to)); // eslint-disable-line react-hooks/rules-of-hooks
  return (
    <Card className="gap-1 p-5 shadow-none">
      <h2 className="text-[13px] font-bold uppercase tracking-wide text-ink-2">{title}</h2>
      <ul>
        {SCENARIOS.map((s, i) => (
          <li key={s.key} className="flex justify-between gap-3 py-1 text-[15px]">
            <span className="text-ink-2">{t(`rates.${s.key}`)}</span>
            <b>{q[i].data ? formatVnd(q[i].data.total) : "—"}</b>
          </li>
        ))}
      </ul>
      <p className="mt-1 flex items-start gap-2 text-[12px] text-muted-foreground max-lg:hidden">
        <Tag className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
        {t("rates.note")}
      </p>
    </Card>
  );
}

function Money({
  label,
  hint,
  suffix = "đ",
  value,
  onChange,
  className,
}: {
  label: string;
  hint?: string;
  suffix?: string;
  value: string;
  onChange: (v: string) => void;
  className?: string;
}) {
  const bad = !/\d/.test(value);
  return (
    <label className={cn("flex flex-col gap-1 text-[13px] font-bold", className)}>
      {label}
      <span className="relative">
        <Input
          value={value}
          inputMode="numeric"
          aria-invalid={bad}
          onChange={(e) => onChange(e.target.value.replace(/[^\d.,]/g, ""))}
          className="h-12 rounded-[10px] bg-card pr-12 text-[16px] font-normal"
        />
        <span className="pointer-events-none absolute right-3.5 top-1/2 -translate-y-1/2 text-sm font-normal text-muted-foreground">
          {suffix}
        </span>
      </span>
      {bad ? (
        <span className="text-[12px] font-normal text-destructive">{t("rates.required")}</span>
      ) : (
        hint && <span className="text-[12px] font-normal text-muted-foreground">{hint}</span>
      )}
    </label>
  );
}

function Editor({
  type,
  draft,
  set,
}: {
  type: UnitTypeRates;
  draft: Draft;
  set: (d: Draft) => void;
}) {
  const p = type.ratePlan;
  const up = (k: keyof Draft) => (v: string) => set({ ...draft, [k]: v });
  return (
    <div className="flex flex-col gap-3">
      <div className="grid grid-cols-2 gap-3">
        <Money label={t("rates.firstHour")} value={draft.firstHour} onChange={up("firstHour")} />
        <Money label={t("rates.extraHour")} value={draft.extraHour} onChange={up("extraHour")} />
      </div>
      <Money
        label={t("rates.overnight")}
        value={draft.overnight}
        onChange={up("overnight")}
        hint={tf("rates.window", { from: p.overnight.windowStart, to: p.overnight.windowEnd })}
      />
      <Money
        label={t("rates.daily")}
        value={draft.daily}
        onChange={up("daily")}
        hint={tf("rates.dailyWindow", { from: p.daily.windowStart, to: p.daily.windowEnd })}
      />
      <Money
        label={t("rates.grace")}
        suffix={t("rates.minutes")}
        value={draft.grace}
        onChange={up("grace")}
        hint={t("rates.graceHint")}
      />
    </div>
  );
}

export function RatesView() {
  const q = useRatePlans();
  const save = useSaveRatePlan();
  // Edits sit on top of what the server sent; nothing is copied into state until the user types.
  const [edits, setEdits] = useState<Record<string, Draft>>({});
  const [code, setCode] = useState<string>();
  const types = q.data ?? [];
  const drafts = Object.fromEntries(
    types.map((x) => [x.code, edits[x.code] ?? draftOf(x.ratePlan)]),
  );
  const setDrafts = (next: Record<string, Draft>) => setEdits(next);

  if (q.isError)
    return (
      <AppFrame tabs={false}>
        <QueryError onRetry={() => void q.refetch()} />
      </AppFrame>
    );
  const current = types.find((x) => x.code === code) ?? types[0];
  const allValid = types.every((x) => !drafts[x.code] || valid(drafts[x.code]));
  const changed = types.filter(
    (x) => drafts[x.code] && JSON.stringify(drafts[x.code]) !== JSON.stringify(draftOf(x.ratePlan)),
  );
  const plan =
    current && drafts[current.code] ? planOf(current.ratePlan, drafts[current.code]) : null;

  const saveAll = async () => {
    try {
      for (const x of changed)
        await save.mutateAsync({ code: x.code, plan: planOf(x.ratePlan, drafts[x.code]) });
      toast.success(t("rates.saved"));
    } catch {
      toast.error(t("rates.saveFailed"));
    }
  };
  const saveBtn = (cls: string) => (
    <Button
      size="lg"
      className={cls}
      disabled={!allValid || save.isPending}
      onClick={() => void saveAll()}
    >
      {t("rates.save")}
    </Button>
  );

  return (
    <AppFrame tabs={false}>
      <main className="mx-auto flex w-full max-w-[1280px] flex-col gap-3 pb-10 lg:px-3">
        <TopBar
          title={t("rates.title")}
          subtitle={
            current
              ? tf("rates.sub", {
                  v: current.ratePlan.version ?? 1,
                  date: formatDayMonth(current.updatedAt),
                })
              : undefined
          }
          back="/owner/settings"
          right={<span className="hidden lg:block">{saveBtn("")}</span>}
        />
        {q.isLoading || !current ? (
          <Skeleton className="mx-5 h-80 rounded-card" aria-busy="true" />
        ) : (
          <FadeIn className="px-5">
            <ToggleGroup
              type="single"
              value={current.code}
              onValueChange={(v) => v && setCode(v)}
              aria-label={t("rates.title")}
              className="mb-3 flex w-full gap-0 rounded-card bg-secondary p-1 lg:hidden"
            >
              {types.map((x) => (
                <ToggleGroupItem
                  key={x.code}
                  value={x.code}
                  className="relative h-11 flex-1 rounded-[10px]! text-[15px] font-bold data-[state=on]:bg-transparent"
                >
                  {current.code === x.code && (
                    <SlidingPill
                      id="rate-pill"
                      className="absolute inset-0 rounded-[10px] bg-card shadow-sm"
                    />
                  )}
                  <span className="relative">{localized(x.name)}</span>
                </ToggleGroupItem>
              ))}
            </ToggleGroup>

            <div className="flex flex-col gap-3 lg:hidden">
              {drafts[current.code] && (
                <Editor
                  type={current}
                  draft={drafts[current.code]}
                  set={(d) => setDrafts({ ...drafts, [current.code]: d })}
                />
              )}
              <Preview plan={plan} title={t("rates.preview")} />
              <p className="rounded-card border border-info-line bg-info-bg p-3 text-[13px] text-info">
                {t("rates.note")}
              </p>
              {saveBtn("w-full")}
            </div>

            <div className="hidden gap-4 lg:grid lg:grid-cols-[1fr_1fr_320px] lg:items-start">
              {types.map((x) => (
                <Card key={x.code} className="gap-3 p-5 shadow-none">
                  <div className="flex items-center justify-between">
                    <h2 className="text-[13px] font-bold uppercase tracking-wide text-ink-2">
                      {localized(x.name)}
                    </h2>
                    <span className="rounded-full bg-sunken px-2.5 py-0.5 text-[12px] font-bold">
                      {tf("rates.version", { v: x.ratePlan.version ?? 1 })}
                    </span>
                  </div>
                  {drafts[x.code] && (
                    <Editor
                      type={x}
                      draft={drafts[x.code]}
                      set={(d) => setDrafts({ ...drafts, [x.code]: d })}
                    />
                  )}
                </Card>
              ))}
              <Preview
                plan={plan}
                title={tf("rates.previewFor", { type: localized(current.name) })}
              />
            </div>
          </FadeIn>
        )}
      </main>
    </AppFrame>
  );
}
