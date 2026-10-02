import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";

export type StaysFilter = { from: string; to: string; q: string };

export function useOwnerStays(f: StaysFilter) {
  return useInfiniteQuery({
    queryKey: ["owner-stays", f],
    initialPageParam: undefined as string | undefined,
    queryFn: async ({ pageParam }) => {
      const { data, error } = await api.GET("/v1/stays", {
        params: {
          query: {
            from: f.from,
            to: f.to,
            ...(f.q && { q: f.q }),
            ...(pageParam && { cursor: pageParam }),
          },
        },
      });
      if (error || !data) throw new Error("listStays failed");
      return data;
    },
    getNextPageParam: (last) => last.nextCursor ?? undefined,
  });
}

export function useTimeline(stayId: string | null) {
  return useQuery({
    queryKey: ["stay-timeline", stayId],
    enabled: !!stayId,
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/owner/stays/{stayId}/timeline", {
        params: { path: { stayId: stayId! } },
      });
      if (error || !data) throw new Error("getStayTimeline failed");
      return data.items;
    },
  });
}
