import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";

// ponytail: first page only (the API default page); add useInfiniteQuery with nextCursor when an owner has more.
export function useAlerts(unread: boolean) {
  return useQuery({
    queryKey: ["alerts", unread],
    refetchInterval: 60_000,
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/owner/alerts", {
        params: { query: unread ? { unread: true } : {} },
      });
      if (error || !data) throw new Error("listAlerts failed");
      return data.items;
    },
  });
}

export function useMarkRead() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (ids: string[]) => {
      // No bulk endpoint: one call per alert.
      for (const id of ids) {
        const { error } = await api.POST("/v1/owner/alerts/{alertId}/read", {
          params: { path: { alertId: id } },
        });
        if (error) throw new Error("markAlertRead failed");
      }
    },
    onSettled: () => {
      void qc.invalidateQueries({ queryKey: ["alerts"] });
      void qc.invalidateQueries({ queryKey: ["owner-overview"] });
    },
  });
}
