import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { components } from "../../api/generated/schema";
import { api, idempotencyHeader } from "../../lib/api";

type Schemas = components["schemas"];

// null when the caller has no open shift (the API answers 404).
export function useCurrentShift(enabled = true) {
  return useQuery({
    queryKey: ["shift", "current"],
    enabled,
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/shifts/current");
      // NOT_FOUND here means no shift is open yet.
      if ((error as { code?: string } | undefined)?.code === "NOT_FOUND") return null;
      if (!data) throw new Error("getCurrentShift failed");
      return data;
    },
  });
}

export function useCloseShift() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ body, key }: { body: Schemas["CloseShiftRequest"]; key: string }) => {
      const { data, error } = await api.POST("/v1/shifts/current/close", {
        params: { header: idempotencyHeader(key) },
        body,
      });
      if (error || !data) throw new Error("closeShift failed");
      return data;
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["shift"] }),
  });
}

export function useRecordPayout() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ body, key }: { body: Schemas["CashPayoutRequest"]; key: string }) => {
      const { data, error } = await api.POST("/v1/shifts/current/payouts", {
        params: { header: idempotencyHeader(key) },
        body,
      });
      if (error || !data) throw new Error("recordCashPayout failed");
      return data;
    },
    onSuccess: (shift) => qc.setQueryData(["shift", "current"], shift),
  });
}
