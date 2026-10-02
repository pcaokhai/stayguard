import { useInfiniteQuery } from "@tanstack/react-query";
import { api } from "../../lib/api";

// One page per cursor; `date` is a single day (YYYY-MM-DD), `q` matches room, guest name or phone.
export function useStays(date: string, q: string) {
  return useInfiniteQuery({
    queryKey: ["stays", date, q],
    initialPageParam: undefined as string | undefined,
    queryFn: async ({ pageParam }) => {
      const { data, error } = await api.GET("/v1/stays", {
        params: { query: { date, q: q || undefined, cursor: pageParam } },
      });
      if (error || !data) throw new Error("listStays failed");
      return data;
    },
    getNextPageParam: (last) => last.nextCursor ?? undefined,
  });
}
