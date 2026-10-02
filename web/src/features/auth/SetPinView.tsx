"use client";

import { Check, Circle, Clock, KeyRound } from "lucide-react";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { toast } from "sonner";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { lp } from "@/lib/locale";
import { loadSession } from "@/lib/session";
import { t, tf } from "@/lib/t";
import { cn } from "@/lib/utils";
import { homeFor } from "./home";
import { forgetPin, peekPin, useChangePin } from "./hooks";
import { isSixDigits, isWeakPin } from "./pin";
import { PinInput } from "./PinInput";

// Boards P2 (first sign-in) and the account "Change PIN" action: the same form. The one-time PIN
// from sign-in is remembered in memory; when it is gone (reload, or a normal change) ask for it.
export function SetPinView() {
  const router = useRouter();
  const change = useChangePin();
  const session = loadSession();
  const [current] = useState(peekPin);
  const [typedCurrent, setTypedCurrent] = useState("");
  const [pin, setPin] = useState("");
  const [again, setAgain] = useState("");
  const [shake, setShake] = useState(0);
  const needsCurrent = current === null;
  const first = !needsCurrent;

  const six = isSixDigits(pin);
  const strong = six && !isWeakPin(pin);
  const same = pin === again;
  const ready = strong && same && (!needsCurrent || isSixDigits(typedCurrent));

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!ready) return setShake((n) => n + 1);
    change.mutate(
      { currentPin: current ?? typedCurrent, newPin: pin },
      {
        onSuccess: () => {
          forgetPin();
          toast.success(t("setPin.changed"));
          router.replace(lp(session ? homeFor(session.user.role) : "/sign-in"));
        },
        onError: () => {
          setPin("");
          setAgain("");
          setShake((n) => n + 1);
        },
      },
    );
  };

  const rules = [
    { ok: six, label: t("setPin.ruleDigits"), icon: six ? Check : Circle },
    { ok: strong, label: t("setPin.ruleRun"), icon: strong ? Check : Circle },
    { ok: false, label: t("setPin.ruleOwn"), icon: Clock }, // checked by the server
  ];

  return (
    <AppFrame tabs={false}>
      <main className="mx-auto flex w-full max-w-[480px] flex-1 flex-col">
        <TopBar
          title={t("setPin.title")}
          subtitle={first && session ? tf("setPin.first", { name: session.user.name }) : undefined}
          back={first ? undefined : "/account"}
        />
        <form onSubmit={submit} noValidate className="flex flex-1 flex-col gap-4 px-5 pb-8">
          {first && (
            <p className="flex items-start gap-2.5 rounded-xl border border-info-line bg-info-bg p-3.5 text-sm text-info">
              <KeyRound className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
              {t("setPin.notice")}
            </p>
          )}
          {needsCurrent && (
            <PinField
              id="current"
              label={t("setPin.current")}
              value={typedCurrent}
              onChange={setTypedCurrent}
              shake={shake}
            />
          )}
          <PinField id="new" label={t("setPin.new")} value={pin} onChange={setPin} shake={shake} />
          <PinField
            id="again"
            label={t("setPin.repeat")}
            value={again}
            onChange={setAgain}
            shake={shake}
            invalid={again.length === 6 && !same}
          />
          {again.length === 6 && !same && (
            <p className="text-sm text-destructive">{t("setPin.mismatch")}</p>
          )}
          <Card className="gap-2.5 p-4 text-sm shadow-none">
            {rules.map((r) => (
              <p
                key={r.label}
                className={cn("flex items-center gap-2.5", r.ok ? "text-ok" : "text-ink-2")}
              >
                <r.icon className="size-4 shrink-0" aria-hidden="true" />
                {r.label}
              </p>
            ))}
          </Card>
          {change.isError && (
            <p role="alert" className="text-sm font-semibold text-warn">
              {t("setPin.failed")}
            </p>
          )}
          <Button type="submit" size="lg" className="mt-auto" loading={change.isPending}>
            {t("setPin.save")}
          </Button>
        </form>
      </main>
    </AppFrame>
  );
}

function PinField({
  id,
  label,
  value,
  onChange,
  shake,
  invalid,
}: {
  id: string;
  label: string;
  value: string;
  onChange: (v: string) => void;
  shake: number;
  invalid?: boolean;
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={id} className="font-bold">
        {label}
      </Label>
      <PinInput id={id} value={value} onChange={onChange} shake={shake} invalid={invalid} />
    </div>
  );
}
