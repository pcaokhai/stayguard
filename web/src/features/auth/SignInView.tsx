"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { Info, Lock, Shield } from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";
import { useState } from "react";
import { Controller, useForm } from "react-hook-form";
import { z } from "zod";
import { FadeIn, Shake } from "@/components/motion";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { lp } from "@/lib/locale";
import { t } from "@/lib/t";
import { DemoPicker } from "../session/RolePicker";
import { homeFor } from "./home";
import { LockedView } from "./LockedView";
import { LocaleSwitch } from "./LocaleSwitch";
import { LockedError, rememberPin, useSignIn, WrongCredentialsError } from "./hooks";
import { isSixDigits } from "./pin";
import { PinInput } from "./PinInput";

// Messages are keys; the form renders them through t() so they follow the route language.
const schema = z.object({
  guesthouseCode: z.string().trim().min(3, "auth.codeRequired").max(16, "auth.codeRequired"),
  username: z.string().trim().min(1, "auth.usernameRequired").max(32),
  pin: z.string().refine(isSixDigits, "auth.pinRequired"),
});
type Values = z.infer<typeof schema>;

export function SignInView() {
  const router = useRouter();
  const params = useSearchParams();
  const expired = params.get("reason") === "expired";
  // Return to where the 401 interrupted, only inside this app and this language.
  const next = params.get("next");
  const back = next && next.startsWith(`${lp("/")}/`) && !next.startsWith("//") ? next : null;
  const signIn = useSignIn();
  const [shake, setShake] = useState(0);
  const [locked, setLocked] = useState<{ until: Date; account: string } | null>(null);
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { guesthouseCode: "", username: "", pin: "" },
  });
  const { errors } = form.formState;

  const submit = form.handleSubmit(
    (v) =>
      signIn.mutate(
        { guesthouseCode: v.guesthouseCode.toUpperCase(), username: v.username, pin: v.pin },
        {
          onSuccess: (s) => {
            if (s.mustChangePin) {
              rememberPin(v.pin);
              router.replace(lp("/set-pin"));
            } else router.replace(back ?? lp(homeFor(s.user.role)));
          },
          onError: (e) => {
            form.setValue("pin", "");
            setShake((n) => n + 1);
            if (e instanceof LockedError)
              setLocked({
                until: e.until,
                account: `${v.username} · ${v.guesthouseCode.toUpperCase()}`,
              });
          },
        },
      ),
    () => setShake((n) => n + 1),
  );

  if (locked) return <LockedView {...locked} onBack={() => setLocked(null)} />;

  return (
    <main className="mx-auto flex min-h-dvh w-full max-w-[1120px] flex-col px-5 pb-6 pt-4 lg:grid lg:grid-cols-2 lg:items-center lg:gap-16 lg:px-10 lg:py-20">
      <div className="flex justify-end lg:fixed lg:right-10 lg:top-6">
        <LocaleSwitch className="w-[110px]" />
      </div>
      <FadeIn className="hidden flex-col gap-4 lg:flex">
        <p className="text-sm font-bold uppercase tracking-wider text-primary">{t("app.title")}</p>
        <h2 className="text-[44px] font-bold leading-[1.1]">{t("auth.heroTitle")}</h2>
        <p className="max-w-[460px] text-base text-ink-2">{t("auth.heroLead")}</p>
      </FadeIn>
      <FadeIn className="mx-auto flex w-full max-w-[480px] flex-1 flex-col gap-4 pt-6 lg:max-w-[440px] lg:flex-none lg:justify-self-center lg:rounded-3xl lg:border lg:bg-card lg:p-8 lg:pt-8">
        <span className="flex size-[52px] items-center justify-center rounded-[14px] bg-primary text-primary-foreground">
          <Shield className="size-6" aria-hidden="true" />
        </span>
        <div>
          <h1 className="text-[28px] font-bold leading-tight">{t("auth.title")}</h1>
          <p className="pt-1 text-[15px] text-ink-2 lg:hidden">{t("auth.lead")}</p>
        </div>
        {expired && (
          <p
            role="status"
            className="rounded-xl border border-warn-line bg-warn-bg p-3 text-sm font-semibold text-warn-ink"
          >
            {t("auth.sessionEnded")}
          </p>
        )}
        <form onSubmit={submit} noValidate className="flex flex-col gap-4">
          <Field
            id="code"
            label={t("auth.code")}
            error={errors.guesthouseCode?.message}
            shake={shake}
          >
            <Input
              id="code"
              autoCapitalize="characters"
              autoComplete="organization"
              maxLength={16}
              aria-invalid={!!errors.guesthouseCode}
              className="h-[52px] rounded-[10px] bg-card px-4 font-mono text-base uppercase"
              {...form.register("guesthouseCode")}
            />
          </Field>
          <Field
            id="username"
            label={t("auth.username")}
            error={errors.username?.message}
            shake={shake}
          >
            <Input
              id="username"
              autoCapitalize="none"
              autoComplete="username"
              maxLength={32}
              aria-invalid={!!errors.username}
              className="h-[52px] rounded-[10px] bg-card px-4 text-base"
              {...form.register("username")}
            />
          </Field>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="pin" className="font-bold">
              {t("auth.pin")}
            </Label>
            <Controller
              control={form.control}
              name="pin"
              render={({ field }) => (
                <PinInput
                  id="pin"
                  value={field.value}
                  onChange={field.onChange}
                  shake={shake}
                  invalid={!!errors.pin}
                />
              )}
            />
            {errors.pin && (
              <p className="text-sm text-destructive">
                {t(errors.pin.message as "auth.pinRequired")}
              </p>
            )}
          </div>
          {signIn.isError && (
            <p role="alert" className="text-sm font-semibold text-warn">
              {signIn.error instanceof WrongCredentialsError ? t("auth.wrong") : t("auth.failed")}
            </p>
          )}
          <Button type="submit" size="lg" loading={signIn.isPending}>
            {t("auth.submit")}
          </Button>
        </form>
        <p className="mt-auto flex gap-2 pt-6 text-[13px] leading-snug text-muted-foreground lg:mt-0 lg:pt-0">
          <Info className="mt-0.5 size-3.5 shrink-0 lg:hidden" aria-hidden="true" />
          <Lock className="mt-0.5 hidden size-3.5 shrink-0 lg:block" aria-hidden="true" />
          <span className="lg:hidden">{t("auth.forgot")}</span>
          <span className="hidden lg:inline">{t("auth.shared")}</span>
        </p>
        <DemoPicker />
      </FadeIn>
    </main>
  );
}

function Field({
  id,
  label,
  error,
  shake,
  children,
}: {
  id: string;
  label: string;
  error?: string;
  shake: number;
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={id} className="font-bold">
        {label}
      </Label>
      <Shake trigger={error ? shake : 0}>{children}</Shake>
      {error && <p className="text-sm text-destructive">{t(error as "auth.codeRequired")}</p>}
    </div>
  );
}
