import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { components } from "@/api/generated/schema";
import { api } from "@/lib/api";

export type RatePlan = components["schemas"]["RatePlan"];
export type UnitTypeRates = components["schemas"]["UnitTypeRates"];

export function useRatePlans() {
  return useQuery({
    queryKey: ["rate-plans"],
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/owner/rate-plans");
      if (error || !data) throw new Error("listRatePlans failed");
      return data.items;
    },
  });
}

export function useSaveRatePlan() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: { code: string; plan: RatePlan }) => {
      const { data, error } = await api.PUT("/v1/owner/unit-types/{unitTypeCode}/rate-plan", {
        params: { path: { unitTypeCode: v.code } },
        body: v.plan,
      });
      if (error || !data) throw new Error("updateRatePlan failed");
      return data;
    },
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["rate-plans"] }),
  });
}

// Quote for a fixed stay on the plan being edited; priced by the server (domain/pricing), never here.
export function usePricePreview(
  plan: RatePlan | null,
  rentalType: "HOURLY" | "OVERNIGHT" | "DAILY",
  checkIn: string,
  checkOut: string,
) {
  return useQuery({
    queryKey: ["price-preview", plan, rentalType, checkIn, checkOut],
    enabled: !!plan,
    placeholderData: (prev) => prev,
    queryFn: async () => {
      const { data, error } = await api.POST("/v1/owner/rate-plans/preview", {
        body: { rentalType, checkIn, checkOut, ratePlan: plan! },
      });
      if (error || !data) throw new Error("previewPrice failed");
      return data;
    },
  });
}
