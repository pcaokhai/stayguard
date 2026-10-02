import { useMutation, useQueryClient } from "@tanstack/react-query";
import type { components } from "@/api/generated/schema";
import { api, idempotencyHeader } from "@/lib/api";

type S = components["schemas"];
export type Room = S["Room"] & { floorId?: string };
export type RoomFeature = S["RoomFeature"];

const fail = (op: string, status: number) => Object.assign(new Error(`${op} failed`), { status });
export const statusOf = (e: unknown) => (e as { status?: number }).status;

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
      if (error || !data) throw fail("createBuilding", response.status);
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
      if (error) throw fail("updateBuilding", response.status);
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
      if (error) throw fail("createFloor", response.status);
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
      if (error) throw fail("createRooms", response.status);
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
      if (error) throw fail("updateRoom", response.status);
    },
    onSuccess: refresh,
  });
}
