import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../../lib/api";

// Poll every 3 s until the status leaves PENDING (docs/14 §5).
export function usePayment(paymentId: string | null) {
  return useQuery({
    queryKey: ["payment", paymentId],
    enabled: !!paymentId,
    refetchInterval: (q) => (q.state.data?.status === "PENDING" ? 3000 : false),
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
