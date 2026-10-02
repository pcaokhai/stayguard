"use client";

import Link from "next/link";
import { Lock, Shield, Users } from "lucide-react";
import { toast } from "sonner";
import { FadeIn } from "@/components/motion";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { QueryError } from "@/components/StateView";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { lp } from "@/lib/locale";
import { t, tf, type MessageKey } from "@/lib/t";
import { cn } from "@/lib/utils";
import { useBuildings } from "../../rooms/hooks";
import { useSetPermission, useStaffPermissions, type Level } from "../staff/hooks";

const LEVELS: Level[] = ["NONE", "VIEW", "EDIT"];
const LABEL: Record<Level, MessageKey> = {
  NONE: "access.none",
  VIEW: "access.view",
  EDIT: "access.edit",
};
// Selected segment colours follow the design: Edit is the filled brand pill, View is info blue.
const ON: Record<Level, string> = {
  NONE: "data-[state=on]:bg-card data-[state=on]:shadow-sm",
  VIEW: "data-[state=on]:border data-[state=on]:border-info-line data-[state=on]:bg-info-bg data-[state=on]:text-info",
  EDIT: "data-[state=on]:bg-primary data-[state=on]:text-primary-foreground",
};

export function AccessView() {
  const perms = useStaffPermissions();
  const buildings = useBuildings();
  const set = useSetPermission();
  const list = (perms.data ?? []).filter((p) => p.role !== "OWNER");
  const bs = buildings.data ?? [];

  if (perms.isError || buildings.isError)
    return (
      <AppFrame tabs={false}>
        <QueryError onRetry={() => void Promise.all([perms.refetch(), buildings.refetch()])} />
      </AppFrame>
    );

  const level = (access: { buildingId: string; level: Level }[], id: string): Level =>
    access.find((a) => a.buildingId === id)?.level ?? "NONE";
  const change = (userId: string, buildingId: string, next: Level) =>
    set.mutate(
      { userId, buildingId, level: next },
      { onError: () => toast.error(t("access.saveFailed")) },
    );

  const segmented = (p: (typeof list)[number], b: (typeof bs)[number]) => (
    <ToggleGroup
      type="single"
      value={level(p.access, b.id)}
      onValueChange={(v) => v && change(p.userId, b.id, v as Level)}
      aria-label={`${p.name} · ${tf("access.building", { code: b.id })}`}
      className="gap-0 rounded-card bg-secondary p-1"
    >
      {LEVELS.map((l) => (
        <ToggleGroupItem
          key={l}
          value={l}
          className={cn(
            "h-10 min-w-[56px] flex-1 rounded-[10px]! px-2.5 text-[14px] font-bold",
            ON[l],
          )}
        >
          {t(LABEL[l])}
        </ToggleGroupItem>
      ))}
    </ToggleGroup>
  );

  return (
    <AppFrame tabs={false}>
      <main className="mx-auto flex w-full max-w-[1280px] flex-col gap-3 pb-10 lg:px-3">
        <TopBar
          title={t("access.title")}
          subtitle={tf("access.subPc", { n: list.length, m: bs.length })}
          back="/owner"
          right={
            <Button asChild variant="outline" size="lg" className="hidden md:inline-flex">
              <Link href={lp("/owner/staff")}>
                <Users aria-hidden="true" />
                {t("access.staffLink")}
              </Link>
            </Button>
          }
        />
        <div className="flex flex-col gap-3 px-5">
          <div className="flex gap-2 rounded-card border border-info-line bg-info-bg p-3.5 text-[13px] text-info md:hidden">
            <Shield className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
            <p>
              <b>{t("access.none")}</b>
              {t("access.legendNone")}. <b>{t("access.view")}</b>
              {t("access.legendView")}. <b>{t("access.edit")}</b>
              {t("access.legendEdit")}.
            </p>
          </div>
          <p className="hidden items-start gap-2 rounded-card border border-info-line bg-info-bg p-3.5 text-[13px] text-info md:flex">
            <Shield className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
            {t("access.legendPc")}
          </p>

          {perms.isLoading && <Skeleton className="h-64 rounded-card" aria-busy="true" />}
          {!perms.isLoading && list.length === 0 && (
            <p className="py-8 text-center text-muted-foreground">{t("access.empty")}</p>
          )}

          <FadeIn className="flex flex-col gap-3 md:hidden">
            {list.map((p) => (
              <Card key={p.userId} className="gap-3 p-4 shadow-none">
                <div className="leading-tight">
                  <b className="block text-[18px]">{p.name}</b>
                  <span className="text-[13px] text-ink-2">
                    {t(`access.role_${p.role}` as MessageKey)}
                  </span>
                </div>
                {bs.map((b) => (
                  <div key={b.id} className="flex items-center gap-3">
                    <span className="w-[64px] shrink-0 whitespace-nowrap text-[15px] font-bold">
                      {tf("access.buildingShort", { code: b.id })}
                    </span>
                    <div className="flex-1">{segmented(p, b)}</div>
                  </div>
                ))}
              </Card>
            ))}
          </FadeIn>

          {list.length > 0 && (
            <FadeIn className="hidden md:block">
              <Card className="gap-0 overflow-x-auto p-0 shadow-none">
                <table className="w-full min-w-[700px] text-[14px]">
                  <thead>
                    <tr className="h-11 text-left text-[12px] uppercase tracking-wide text-ink-2">
                      <th scope="col" className="px-3 pl-5 font-bold">
                        {t("access.staff")}
                      </th>
                      <th scope="col" className="px-3 font-bold">
                        {t("access.role")}
                      </th>
                      {bs.map((b) => (
                        <th key={b.id} scope="col" className="px-3 font-bold">
                          {tf("access.building", { code: b.id })}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {list.map((p) => (
                      <tr key={p.userId} className="border-t border-border">
                        <td className="px-3 py-3 pl-5 text-[15px] font-bold whitespace-nowrap">
                          {p.name}
                        </td>
                        <td className="px-3 text-ink-2">
                          {t(`access.role_${p.role}` as MessageKey)}
                        </td>
                        {bs.map((b) => (
                          <td key={b.id} className="px-3 pr-4">
                            {segmented(p, b)}
                          </td>
                        ))}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </Card>
            </FadeIn>
          )}
          <p className="flex items-start gap-2 text-[13px] text-muted-foreground">
            <Lock className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
            <span className="md:hidden">{t("access.footPhone")}</span>
            <span className="hidden md:inline">{t("access.footPc")}</span>
          </p>
        </div>
      </main>
    </AppFrame>
  );
}
