import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";

export function useOverview(enabled = true) {
  return useQuery({
    queryKey: ["owner-overview"],
    enabled,
    refetchInterval: 60_000,
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/owner/overview");
      if (error || !data) throw new Error("getOwnerOverview failed");
      return data;
    },
  });
}
