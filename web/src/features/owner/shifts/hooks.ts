import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";

// ponytail: first page of closed shifts (current month by API default); add a cursor when owners need more.
export function useClosedShifts(onlyDifferences: boolean) {
  return useQuery({
    queryKey: ["closed-shifts", onlyDifferences],
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/owner/shifts", {
        params: { query: onlyDifferences ? { onlyDifferences: true } : {} },
      });
      if (error || !data) throw new Error("listClosedShifts failed");
      return data.items;
    },
  });
}

export function useShiftReview(id: string | null) {
  return useQuery({
    queryKey: ["shift-review", id],
    enabled: !!id,
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/owner/shifts/{shiftId}", {
        params: { path: { shiftId: id! } },
      });
      if (error || !data) throw new Error("getShiftReview failed");
      return data;
    },
  });
}
