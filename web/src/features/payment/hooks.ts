import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../../lib/api";

// Poll every 3 s while PENDING (docs/14 §5) and also while EXPIRED: the expiry is derived on read and the stored payment
// stays pending, so a late transfer to the old bill code still settles it and the screen must turn Paid by itself.
export const payRefetchInterval = (status: string | undefined) =>
  status === "PENDING" || status === "EXPIRED" ? 3000 : false;

export function usePayment(paymentId: string | null) {
  return useQuery({
    queryKey: ["payment", paymentId],
    enabled: !!paymentId,
    refetchInterval: (q) => payRefetchInterval(q.state.data?.status),
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/payments/{paymentId}", {
        params: { path: { paymentId: paymentId! } },
      });
      if (error || !data) throw new Error("getPayment failed");
      return data;
    },
  });
}

export function useSimulatePayment(paymentId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      const { error } = await api.POST("/v1/demo/payments/{paymentId}/simulate", {
        params: { path: { paymentId } },
      });
      if (error) throw new Error("simulatePaymentReceived failed");
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["payment", paymentId] }),
  });
}
