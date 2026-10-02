"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect, useState } from "react";
import { useForm, type Resolver } from "react-hook-form";
import { Plus, RefreshCw, Star, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { z } from "zod";
import { FadeIn } from "@/components/motion";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { QueryError } from "@/components/StateView";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Form } from "@/components/ui/form";
import { Skeleton } from "@/components/ui/skeleton";
import { t, tf } from "@/lib/t";
import { clockOf, formatDayMonth } from "../format";
import { OwnerPinDialog } from "../OwnerPinDialog";
import { TextField } from "../FormFields";
import { BankSheet } from "./BankSheet";
import {
  statusOf,
  useBankAccounts,
  useMakeDefaultBank,
  useProperty,
  useRemoveBank,
  useSepayStatus,
  useUpdateProperty,
  type BankAccount,
} from "./hooks";

const int = (min: number, max: number) =>
  z
    .string()
    .trim()
    .regex(/^\d+$/, "property.required")
    .refine((v) => Number(v) >= min && Number(v) <= max, "property.range");
const schema = z.object({
  name: z.string().trim().min(1, "property.required"),
  address: z.string().trim(),
  phone: z.string().trim(),
  qrExpiryMinutes: int(5, 240),
  idRetentionDays: int(1, 365),
  frontDeskHistoryDays: int(1, 90),
});
type Values = z.infer<typeof schema>;
const heading = "text-[13px] font-bold uppercase tracking-wide text-ink-2";

type Pending = { kind: "default" | "remove"; account: BankAccount } | null;

export function PropertyView() {
  const property = useProperty();
  const banks = useBankAccounts();
  const sepay = useSepayStatus();
  const update = useUpdateProperty();
  const makeDefault = useMakeDefaultBank();
  const removeBank = useRemoveBank();
  const [adding, setAdding] = useState(false);
  const [pending, setPending] = useState<Pending>(null);
  const [error, setError] = useState("");
  const [shake, setShake] = useState(0);

  const form = useForm<Values>({
    resolver: zodResolver(schema) as Resolver<Values>,
    defaultValues: {
      name: "",
      address: "",
      phone: "",
      qrExpiryMinutes: "30",
      idRetentionDays: "30",
      frontDeskHistoryDays: "7",
    },
  });
  const p = property.data;
  useEffect(() => {
    if (!p) return;
    form.reset({
      name: p.name,
      address: p.address ?? "",
      phone: p.phone ?? "",
      qrExpiryMinutes: String(p.qrExpiryMinutes),
      idRetentionDays: String(p.idRetentionDays ?? 30),
      frontDeskHistoryDays: String(p.frontDeskHistoryDays ?? 7),
    });
  }, [p, form]);

  if (property.isError || banks.isError)
    return (
      <AppFrame tabs={false}>
        <QueryError onRetry={() => void Promise.all([property.refetch(), banks.refetch()])} />
      </AppFrame>
    );

  const save = form.handleSubmit((v) =>
    update.mutate(
      {
        name: v.name,
        address: v.address || null,
        phone: v.phone || null,
        qrExpiryMinutes: Number(v.qrExpiryMinutes),
        idRetentionDays: Number(v.idRetentionDays),
        frontDeskHistoryDays: Number(v.frontDeskHistoryDays),
      },
      {
        onSuccess: () => toast.success(t("property.saved")),
        onError: () => toast.error(t("property.saveFailed")),
      },
    ),
  );

  const confirm = (pin: string) => {
    if (!pending) return;
    const vars = { accountId: pending.account.id, ownerPin: pin };
    const done = (msg: string) => ({
      onSuccess: () => {
        toast.success(msg);
        setPending(null);
      },
      onError: (e: unknown) => {
        const s = statusOf(e);
        setShake((n) => n + 1);
        setError(
          s === 403 || s === 422
            ? t("property.wrongPin")
            : s === 409
              ? t("property.isDefaultBlock")
              : t("property.actionFailed"),
        );
      },
    });
    setError("");
    if (pending.kind === "default") makeDefault.mutate(vars, done(t("property.defaultDone")));
    else removeBank.mutate(vars, done(t("property.removed")));
  };

  const saveButton = (cls: string) => (
    <Button
      type="submit"
      form="property-form"
      size="lg"
      className={cls}
      disabled={update.isPending}
    >
      {t("property.saveChanges")}
    </Button>
  );

  const bankCard = (a: BankAccount) => (
    <Card key={a.id} className="gap-2 p-4 shadow-none">
      <b className="text-[17px]">
        {a.bankName} · {a.accountNoMasked}
      </b>
      <p className="text-[13px] text-ink-2">{a.accountName}</p>
      <div className="flex flex-wrap gap-1.5">
        {a.isDefault && <Badge variant="info">{t("property.default")}</Badge>}
        <Badge variant={a.sepayStatus === "CONNECTED" ? "ok" : "warn"}>
          {a.sepayStatus === "CONNECTED" ? t("property.connected") : t("property.pending")}
        </Badge>
      </div>
      {a.isDefault ? (
        <p className="text-[13px] text-muted-foreground">{t("property.defaultFirst")}</p>
      ) : (
        <>
          <div className="flex flex-wrap gap-2">
            <Button
              variant="outline"
              size="lg"
              className="flex-1 font-bold"
              disabled={a.sepayStatus !== "CONNECTED"}
              onClick={() => setPending({ kind: "default", account: a })}
            >
              <Star aria-hidden="true" />
              {t("property.makeDefault")}
            </Button>
            <Button
              variant="outline"
              size="lg"
              className="border-destructive/40 font-bold text-destructive hover:bg-destructive/10"
              onClick={() => setPending({ kind: "remove", account: a })}
            >
              <Trash2 aria-hidden="true" />
              {t("property.remove")}
            </Button>
          </div>
          {a.sepayStatus !== "CONNECTED" && (
            <p className="text-[13px] text-muted-foreground">{t("property.pendingHint")}</p>
          )}
        </>
      )}
    </Card>
  );

  return (
    <AppFrame tabs={false}>
      <main className="mx-auto flex w-full max-w-[1280px] flex-col gap-3 pb-10 lg:px-3">
        <TopBar
          title={t("property.title")}
          subtitle={t("property.subPc")}
          back="/owner/settings"
          right={<span className="hidden lg:block">{saveButton("")}</span>}
        />
        {property.isLoading ? (
          <Skeleton className="mx-5 h-72 rounded-card" aria-busy="true" />
        ) : (
          <FadeIn className="grid gap-3 px-5 lg:grid-cols-2 lg:items-start lg:gap-4">
            <Form {...form}>
              <form
                id="property-form"
                onSubmit={save}
                noValidate
                className="flex flex-col gap-3 lg:order-1"
              >
                <Card className="gap-3 p-5 shadow-none">
                  <h2 className={heading}>{t("property.info")}</h2>
                  <TextField control={form.control} name="name" label={t("property.name")} />
                  <TextField control={form.control} name="address" label={t("property.address")} />
                  <TextField
                    control={form.control}
                    name="phone"
                    label={t("property.phone")}
                    type="tel"
                  />
                  <TextField
                    control={form.control}
                    name="qrExpiryMinutes"
                    label={t("property.qrExpiry")}
                    suffix={t("property.minutes")}
                    inputMode="numeric"
                    hint={t("property.qrHint")}
                  />
                  <TextField
                    control={form.control}
                    name="idRetentionDays"
                    label={t("property.retention")}
                    suffix={t("property.retentionUnit")}
                    inputMode="numeric"
                  />
                  <TextField
                    control={form.control}
                    name="frontDeskHistoryDays"
                    label={t("property.history")}
                    suffix={t("property.historyUnit")}
                    inputMode="numeric"
                  />
                </Card>
                <div className="lg:hidden">{saveButton("w-full")}</div>
              </form>
            </Form>

            <div className="flex flex-col gap-3 max-lg:order-first lg:order-2">
              <Card className="gap-3 p-5 shadow-none">
                <h2 className={heading}>{t("property.banks")}</h2>
                {banks.isLoading && <Skeleton className="h-28 rounded-card" />}
                {banks.data?.length === 0 && (
                  <p className="text-sm text-muted-foreground">{t("property.noBanks")}</p>
                )}
                {banks.data?.map(bankCard)}
                <Button
                  variant="outline"
                  size="lg"
                  className="border-dashed font-bold"
                  onClick={() => setAdding(true)}
                >
                  <Plus aria-hidden="true" />
                  {t("property.add")}
                </Button>
                <p className="text-[12px] text-muted-foreground">{t("property.bankNote")}</p>
              </Card>
              <Card className="gap-2 p-5 shadow-none">
                <h2 className={heading}>{t("property.sepay")}</h2>
                <div className="flex items-center justify-between">
                  <span className="text-ink-2">{t("property.sepayName")}</span>
                  {sepay.data && (
                    <Badge variant={sepay.data.status === "CONNECTED" ? "ok" : "warn"}>
                      {sepay.data.status === "CONNECTED"
                        ? t("property.sepayOn")
                        : t("property.sepayOff")}
                    </Badge>
                  )}
                </div>
                <div className="flex justify-between gap-3 text-[14px]">
                  <span className="text-ink-2">{t("property.lastWebhook")}</span>
                  <b>
                    {sepay.data?.lastWebhookAt
                      ? `${formatDayMonth(sepay.data.lastWebhookAt)} ${clockOf(sepay.data.lastWebhookAt)}`
                      : t("property.never")}
                  </b>
                </div>
                <Button
                  variant="outline"
                  size="lg"
                  className="font-bold"
                  disabled={sepay.isFetching}
                  onClick={() =>
                    void sepay.refetch().then(() => toast.success(t("property.checked")))
                  }
                >
                  <RefreshCw aria-hidden="true" />
                  {t("property.check")}
                </Button>
                <p className="text-[12px] text-muted-foreground">{t("property.help")}</p>
              </Card>
            </div>
          </FadeIn>
        )}
      </main>
      {adding && <BankSheet onClose={() => setAdding(false)} />}
      {pending && (
        <OwnerPinDialog
          key={pending.account.id + pending.kind}
          title={tf(pending.kind === "default" ? "property.defaultTitle" : "property.removeTitle", {
            bank: `${pending.account.bankName} ${pending.account.accountNoMasked}`,
          })}
          body={pending.kind === "default" ? t("property.defaultBody") : t("property.removeBody")}
          confirmLabel={t("property.confirm")}
          tone={pending.kind === "remove" ? "danger" : "primary"}
          pending={makeDefault.isPending || removeBank.isPending}
          error={error}
          shake={shake}
          onConfirm={confirm}
          onClose={() => {
            setPending(null);
            setError("");
          }}
        />
      )}
    </AppFrame>
  );
}
