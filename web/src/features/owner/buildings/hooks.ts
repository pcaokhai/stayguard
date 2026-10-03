import { useMutation, useQueryClient } from "@tanstack/react-query";
import type { components } from "@/api/generated/schema";
import { problem } from "../../problem/problem";
import { api, idempotencyHeader } from "@/lib/api";

type S = components["schemas"];
export type Room = S["Room"] & { floorId?: string };
export type RoomFeature = S["RoomFeature"];

function useRefresh() {
  const qc = useQueryClient();
  return () => {
    void qc.invalidateQueries({ queryKey: ["buildings"] });
    void qc.invalidateQueries({ queryKey: ["rooms"] });
  };
}

export function useCreateBuilding() {
  const refresh = useRefresh();
  return useMutation({
    mutationFn: async (v: { key: string; body: S["CreateBuildingRequest"] }) => {
      const { data, error, response } = await api.POST("/v1/owner/buildings", {
        params: { header: idempotencyHeader(v.key) },
        body: v.body,
      });
      if (error || !data) throw problem("createBuilding", error, response);
      return data;
    },
    onSuccess: refresh,
  });
}

export function useUpdateBuilding() {
  const refresh = useRefresh();
  return useMutation({
    mutationFn: async (v: { id: string; name: string }) => {
      const { error, response } = await api.PATCH("/v1/owner/buildings/{buildingId}", {
        params: { path: { buildingId: v.id } },
        body: { name: v.name },
      });
      if (error) throw problem("updateBuilding", error, response);
    },
    onSuccess: refresh,
  });
}

export function useCreateFloor() {
  const refresh = useRefresh();
  return useMutation({
    mutationFn: async (v: { key: string; buildingId: string; body: S["CreateFloorRequest"] }) => {
      const { error, response } = await api.POST("/v1/owner/buildings/{buildingId}/floors", {
        params: { path: { buildingId: v.buildingId }, header: idempotencyHeader(v.key) },
        body: v.body,
      });
      if (error) throw problem("createFloor", error, response);
    },
    onSuccess: refresh,
  });
}

export function useCreateRooms() {
  const refresh = useRefresh();
  return useMutation({
    mutationFn: async (v: { key: string; body: S["CreateRoomsRequest"] }) => {
      const { error, response } = await api.POST("/v1/owner/rooms", {
        params: { header: idempotencyHeader(v.key) },
        body: v.body,
      });
      if (error) throw problem("createRooms", error, response);
    },
    onSuccess: refresh,
  });
}

export function useUpdateRoom() {
  const refresh = useRefresh();
  return useMutation({
    mutationFn: async (v: { id: string; body: S["UpdateRoomRequest"] }) => {
      const { error, response } = await api.PATCH("/v1/owner/rooms/{roomId}", {
        params: { path: { roomId: v.id } },
        body: v.body,
      });
      if (error) throw problem("updateRoom", error, response);
    },
    onSuccess: refresh,
  });
}
