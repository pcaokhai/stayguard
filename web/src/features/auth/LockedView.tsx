"use client";

import { Bell, Lock } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { StateView } from "@/components/StateView";
import { t, tf } from "@/lib/t";
import { formatClock } from "@/lib/time";

// Board P3: five wrong PINs. Shown by the sign-in page; "back" returns to the form.
export function LockedView({
  until,
  account,
  onBack,
}: {
  until: Date;
  account: string;
  onBack: () => void;
}) {
  const time = formatClock(until.toISOString());
  return (
    <main className="mx-auto w-full max-w-[480px]">
      <StateView
        icon={Lock}
        tone="warn"
        title={t("locked.title")}
        body={tf("locked.body", { time })}
      >
        <Button variant="outline" size="lg" onClick={onBack}>
          {t("locked.back")}
        </Button>
        <p className="flex items-center gap-2 pt-1 text-[13px] text-muted-foreground">
          <Bell className="size-3.5" aria-hidden="true" />
          {t("locked.notified")}
        </p>
      </StateView>
    </main>
  );
}
