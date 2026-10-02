import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { components } from "@/api/generated/schema";
import { api, idempotencyHeader } from "@/lib/api";

type S = components["schemas"];
export type ExpenseMonth = S["ExpenseMonth"];
export type Expense = S["Expense"];
export type Category = S["ExpenseCategory"];
export type Source = S["ExpenseSource"];
export type CreateExpense = S["CreateExpenseRequest"];
export type Payroll = S["Payroll"];
export type PayrollLine = S["PayrollLine"];
export type IncomeCostReport = S["IncomeCostReport"];

const fail = (op: string, status: number) => Object.assign(new Error(`${op} failed`), { status });
export const statusOf = (e: unknown) => (e as { status?: number }).status;

export function useExpenseMonth(month: string) {
  return useQuery({
    queryKey: ["expenses", month],
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/owner/expenses", { params: { query: { month } } });
      if (error || !data) throw new Error("getExpenseMonth failed");
      return data;
    },
  });
}

function useRefreshExpenses() {
  const qc = useQueryClient();
  return () => {
    void qc.invalidateQueries({ queryKey: ["expenses"] });
    void qc.invalidateQueries({ queryKey: ["report"] });
  };
}

export function useCreateExpense() {
  const refresh = useRefreshExpenses();
  return useMutation({
    mutationFn: async (v: { key: string; body: CreateExpense }) => {
      const { data, error, response } = await api.POST("/v1/owner/expenses", {
        params: { header: idempotencyHeader(v.key) },
        body: v.body,
      });
      if (error || !data) throw fail("createExpense", response.status);
      return data;
    },
    onSuccess: refresh,
  });
}

export function useUpdateExpense() {
  const refresh = useRefreshExpenses();
  return useMutation({
    mutationFn: async (v: { id: string; body: CreateExpense }) => {
      const { data, error, response } = await api.PATCH("/v1/owner/expenses/{expenseId}", {
        params: { path: { expenseId: v.id } },
        body: v.body,
      });
      if (error || !data) throw fail("updateExpense", response.status);
      return data;
    },
    onSuccess: refresh,
  });
}

export function useDeleteExpense() {
  const refresh = useRefreshExpenses();
  return useMutation({
    mutationFn: async (id: string) => {
      const { error, response } = await api.DELETE("/v1/owner/expenses/{expenseId}", {
        params: { path: { expenseId: id } },
      });
      if (error) throw fail("deleteExpense", response.status);
    },
    onSuccess: refresh,
  });
}

export function usePayroll(month: string) {
  return useQuery({
    queryKey: ["payroll", month],
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/owner/payroll/{month}", {
        params: { path: { month } },
      });
      if (error || !data) throw new Error("getPayroll failed");
      return data;
    },
  });
}

export function useUpdatePayrollLine(month: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: {
      userId: string;
      bonus?: number;
      deduction?: number;
      note?: string | null;
    }) => {
      const { userId, ...body } = v;
      const { data, error, response } = await api.PATCH(
        "/v1/owner/payroll/{month}/lines/{userId}",
        {
          params: { path: { month, userId } },
          body,
        },
      );
      if (error || !data) throw fail("updatePayrollLine", response.status);
      return data;
    },
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["payroll", month] }),
  });
}

export function useMarkPaid(month: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: { key: string; userIds?: string[] }) => {
      const { data, error, response } = await api.POST("/v1/owner/payroll/{month}/mark-paid", {
        params: { path: { month }, header: idempotencyHeader(v.key) },
        body: v.userIds ? { userIds: v.userIds } : {},
      });
      if (error || !data) throw fail("markPayrollPaid", response.status);
      return data;
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["payroll", month] });
      void qc.invalidateQueries({ queryKey: ["expenses"] });
    },
  });
}

export function useReport(from: string, to: string) {
  return useQuery({
    queryKey: ["report", from, to],
    placeholderData: (prev) => prev,
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/owner/reports/income-costs", {
        params: { query: { from, to } },
      });
      if (error || !data) throw new Error("getIncomeCostReport failed");
      return data;
    },
  });
}
