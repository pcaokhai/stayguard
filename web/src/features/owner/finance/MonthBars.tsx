"use client";

import { useReducedMotion } from "motion/react";
import { Bar, BarChart, ResponsiveContainer, XAxis } from "recharts";
import { formatVnd } from "@/lib/money";
import { t, tf } from "@/lib/t";
import { monthNumber } from "./month";

type Row = { month: string; revenue: number; expenses: number };

// Two bars per month and the month's profit under it. Loaded on demand; Recharts stays off the other routes.
export default function MonthBars({ months, height }: { months: Row[]; height: number }) {
  const reduced = useReducedMotion();
  const byMonth = Object.fromEntries(months.map((m) => [m.month, m]));
  const Tick = ({
    x,
    y,
    payload,
  }: {
    x?: number | string;
    y?: number | string;
    payload?: { value?: string };
  }) => {
    const value = String(payload?.value ?? "");
    const m = byMonth[value];
    const profit = m ? m.revenue - m.expenses : 0;
    return (
      <g transform={`translate(${Number(x)},${Number(y)})`} textAnchor="middle">
        <text dy={16} fontSize={13} fontWeight={700} fill="var(--foreground)">
          {tf("report.monthShort", { n: monthNumber(value) })}
        </text>
        <text dy={32} fontSize={11} fill={profit >= 0 ? "var(--color-ok)" : "var(--destructive)"}>
          {tf(profit >= 0 ? "report.profitOf" : "report.lossOf", {
            amount: formatVnd(Math.abs(profit)),
          })}
        </text>
      </g>
    );
  };
  return (
    <div role="img" aria-label={t("report.byMonthPc")}>
      <ResponsiveContainer width="100%" height={height}>
        <BarChart
          data={months}
          margin={{ top: 8, right: 8, bottom: 40, left: 8 }}
          barGap={4}
          barCategoryGap="28%"
        >
          <XAxis dataKey="month" tickLine={false} axisLine={false} tick={Tick} interval={0} />
          <Bar
            dataKey="revenue"
            fill="var(--chart-1)"
            radius={[8, 8, 0, 0]}
            isAnimationActive={!reduced}
            animationDuration={320}
          />
          <Bar
            dataKey="expenses"
            fill="var(--chart-2)"
            radius={[8, 8, 0, 0]}
            isAnimationActive={!reduced}
            animationDuration={320}
          />
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}
