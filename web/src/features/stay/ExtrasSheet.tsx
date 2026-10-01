"use client";

import { useState } from "react";
import { newIdempotencyKey } from "../../lib/api";
import { formatVnd } from "../../lib/money";
import { t, tf } from "../../lib/t";
import { useAddExtras, useServices } from "./hooks";

export function ExtrasSheet({
  stayId,
  roomCode,
  onClose,
}: {
  stayId: string;
  roomCode: string;
  onClose: () => void;
}) {
  const services = useServices();
  const add = useAddExtras(stayId);
  const [qty, setQty] = useState<Record<string, number>>({});
  const [key] = useState(newIdempotencyKey);
  const items = Object.entries(qty)
    .filter(([, n]) => n > 0)
    .map(([serviceCode, quantity]) => ({ serviceCode, quantity }));
  const step = (code: string, d: number, stock: number) =>
    setQty((q) => ({ ...q, [code]: Math.min(stock, Math.max(0, (q[code] ?? 0) + d)) }));

  return (
    <div className="fixed inset-0 z-10 flex items-end bg-black/50" onClick={onClose}>
      <section
        role="dialog"
        aria-modal="true"
        aria-labelledby="extras-title"
        onClick={(e) => e.stopPropagation()}
        className="mx-auto flex max-h-[90dvh] w-full max-w-[480px] flex-col gap-3 overflow-y-auto rounded-t-3xl bg-bg px-5 pb-6 pt-4"
      >
        <div className="flex items-start justify-between">
          <div>
            <h2 id="extras-title" className="text-[22px] font-bold">
              {t("extras.title")}
            </h2>
            <p className="text-sm text-muted">
              {t("stay.roomTitle")} {roomCode}
            </p>
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label={t("extras.close")}
            className="size-11 text-2xl"
          >
            ×
          </button>
        </div>
        <ul className="flex flex-col gap-2">
          {services.data?.map((s) => (
            <li
              key={s.code}
              className="flex items-center justify-between gap-3 rounded-card border border-line-soft bg-surface p-3.5"
            >
              <div>
                <p className="font-semibold">{s.name.vi}</p>
                <p className="text-[13px] text-muted">
                  {formatVnd(s.price)} · {tf("extras.stock", { n: s.stock })}
                </p>
              </div>
              <div className="flex items-center gap-3">
                <button
                  type="button"
                  aria-label={t("extras.less")}
                  onClick={() => step(s.code, -1, s.stock)}
                  className="size-11 rounded-[10px] border border-line text-xl"
                >
                  −
                </button>
                <span className="w-5 text-center text-lg font-bold">{qty[s.code] ?? 0}</span>
                <button
                  type="button"
                  aria-label={t("extras.more")}
                  onClick={() => step(s.code, 1, s.stock)}
                  className="size-11 rounded-[10px] bg-brand/10 text-xl"
                >
                  +
                </button>
              </div>
            </li>
          ))}
        </ul>
        <p className="text-[13px] text-muted">{t("extras.note")}</p>
        {add.isError && (
          <p role="alert" className="text-sm text-warn">
            {t("extras.failed")}
          </p>
        )}
        <button
          type="button"
          disabled={!items.length || add.isPending}
          onClick={() => add.mutate({ key, body: { items } }, { onSuccess: onClose })}
          className="h-14 whitespace-nowrap rounded-card bg-brand text-lg font-bold text-white disabled:opacity-60"
        >
          {t("extras.add")}
        </button>
      </section>
    </div>
  );
}
