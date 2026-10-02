"use client";

import { useSearchParams } from "next/navigation";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { EmptyState, ForbiddenState, OfflineState, ServerErrorState } from "@/components/StateView";
import { t } from "@/lib/t";

// Not in the navigation: renders the shared states (?kind=offline|error|forbidden|empty) so
// boards MatMang, LoiMayChu, KhongCoQuyen and DanhSachTrong can be checked in the browser.
export function StatesPreview() {
  const kind = useSearchParams().get("kind") ?? "offline";
  const retry = () => window.location.reload();
  return (
    <AppFrame tabs={false}>
      <TopBar title={kind === "forbidden" ? "Tòa B" : t("nav.main")} back="/rooms" />
      {kind === "error" && <ServerErrorState onRetry={retry} code="7f3a…c21" />}
      {kind === "forbidden" && <ForbiddenState building="Tòa B" />}
      {kind === "empty" && <EmptyState title={t("state.retry")} body={t("state.backRooms")} />}
      {kind === "offline" && <OfflineState onRetry={retry} />}
    </AppFrame>
  );
}
