import { useQuery } from "@tanstack/react-query";
import { api } from "../../lib/api";

export function useBuildings() {
  return useQuery({
    queryKey: ["buildings"],
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/buildings");
      if (error || !data) throw new Error("listBuildings failed");
      return data.items;
    },
  });
}

export function useRooms(buildingId: string | undefined) {
  return useQuery({
    queryKey: ["rooms", buildingId],
    enabled: !!buildingId,
    refetchInterval: 30_000,
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/buildings/{buildingId}/rooms", {
        params: { path: { buildingId: buildingId! } },
      });
      if (error || !data) throw new Error("listRooms failed");
      return data.items;
    },
  });
}
