import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, idempotencyHeader } from "@/lib/api";
import { addDays } from "../range";
import { localDay } from "../format";

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

// A bill still waiting for money. The contract's stay list has no invoice id yet, so a row without
// one cannot be chosen (see the report: StayListItem needs invoiceId and billCode).
export type Candidate = {
  invoiceId?: string;
  billCode?: string;
  roomCode: string;
  checkOutAt?: string | null;
  total?: number | null;
};

export function useUnpaidBills(enabled: boolean) {
  return useQuery({
    queryKey: ["unpaid-bills"],
    enabled,
    queryFn: async (): Promise<Candidate[]> => {
      const now = new Date();
      const { data, error } = await api.GET("/v1/stays", {
        params: { query: { from: localDay(addDays(now, -7)), to: localDay(now), state: "UNPAID" } },
      });
      if (error || !data) throw new Error("listStays failed");
      return data.items.map((s) => ({
        ...(s as { invoiceId?: string; billCode?: string }),
        roomCode: s.roomCode,
        checkOutAt: s.checkOutAt,
        total: s.total,
      }));
    },
  });
}

// One Idempotency-Key per link action, reused when the same action is retried.
export function useLinkTransfer() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: { eventId: string; invoiceId: string; key: string }) => {
      const { error, response } = await api.POST("/v1/owner/payment-events/{eventId}/link", {
        params: { path: { eventId: v.eventId }, header: idempotencyHeader(v.key) },
        body: { invoiceId: v.invoiceId },
      });
      if (error)
        throw Object.assign(new Error("linkTransferToInvoice failed"), { status: response.status });
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["transactions"] });
      void qc.invalidateQueries({ queryKey: ["unpaid-bills"] });
      void qc.invalidateQueries({ queryKey: ["owner-overview"] });
    },
  });
}
