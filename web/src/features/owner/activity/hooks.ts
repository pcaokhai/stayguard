import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";

export type LogFilter = { from: string; to: string; who: string; cat: string; q: string };

export function useAuditLog(f: LogFilter) {
  return useInfiniteQuery({
    queryKey: ["audit-logs", f],
    initialPageParam: undefined as string | undefined,
    queryFn: async ({ pageParam }) => {
      const { data, error } = await api.GET("/v1/owner/audit-logs", {
        params: {
          query: {
            from: f.from,
            to: f.to,
            ...(f.who && { actorId: f.who }),
            ...(f.cat && { category: f.cat }),
            ...(f.q && { q: f.q }),
            ...(pageParam && { cursor: pageParam }),
          },
        },
      });
      if (error || !data) throw new Error("listAuditLogs failed");
      return data;
    },
    getNextPageParam: (last) => last.nextCursor ?? undefined,
  });
}

export function useStaffNames() {
  return useQuery({
    queryKey: ["staff-names"],
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/owner/staff", { params: { query: {} } });
      if (error || !data) throw new Error("listStaff failed");
      return data.items.map((s) => ({ id: s.id, name: s.name }));
    },
  });
}
