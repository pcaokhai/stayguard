import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { components } from "@/api/generated/schema";
import { problem } from "../../problem/problem";
import { api, idempotencyHeader } from "@/lib/api";

export function useTransactions(q: string) {
  return useQuery({
    queryKey: ["transactions", q],
    refetchInterval: 30_000,
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/owner/transactions", {
        params: { query: { filter: "ALL", ...(q && { q }) } },
      });
      if (error || !data) throw new Error("listTransactions failed");
      return data.items;
    },
  });
}

export type Candidate = components["schemas"]["InvoiceCandidate"];

// Unpaid invoices for the transfer's amount; bills whose balance equals it come first.
export function useUnpaidBills(enabled: boolean, amount: number) {
  return useQuery({
    queryKey: ["unpaid-bills", amount],
    enabled,
    queryFn: async (): Promise<Candidate[]> => {
      const { data, error } = await api.GET("/v1/owner/invoices", {
        params: { query: { amount } },
      });
      if (error || !data) throw new Error("listInvoices failed");
      return [...data.items].sort(
        (a, b) => Number(b.balance === amount) - Number(a.balance === amount),
      );
    },
  });
}

export type LinkVars = { eventId: string; invoiceId: string; key: string };

// One Idempotency-Key per link action, reused when the same action is retried.
export async function linkTransfer(v: LinkVars, client = api) {
  const { error, response } = await client.POST("/v1/owner/payment-events/{eventId}/link", {
    params: { path: { eventId: v.eventId }, header: idempotencyHeader(v.key) },
    body: { invoiceId: v.invoiceId },
  });
  if (error) throw problem("linkTransferToInvoice", error, response);
}

export function useLinkTransfer() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (v: LinkVars) => linkTransfer(v),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["transactions"] });
      void qc.invalidateQueries({ queryKey: ["unpaid-bills"] });
      void qc.invalidateQueries({ queryKey: ["owner-overview"] });
    },
  });
}
