import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import type { components } from "../../api/generated/schema";
import { api, idempotencyHeader } from "../../lib/api";
import { t, tf } from "../../lib/t";

type Schemas = components["schemas"];
type Task = Schemas["HousekeepingTask"];
const KEY = ["hk-tasks"];

export function useTasks() {
  return useQuery({
    queryKey: KEY,
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/housekeeping/tasks");
      if (error || !data) throw new Error("listHousekeepingTasks failed");
      return data.items;
    },
  });
}

// Optimistic: the card leaves the list at once; a failure puts it back and says so (docs/14 L-W5).
export function useCompleteTask() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ taskId, key }: { taskId: string; key: string; roomCode?: string }) => {
      const { error } = await api.POST("/v1/housekeeping/tasks/{taskId}/complete", {
        params: { path: { taskId }, header: idempotencyHeader(key) },
      });
      if (error) throw new Error("completeHousekeepingTask failed");
    },
    onMutate: async ({ taskId }) => {
      await qc.cancelQueries({ queryKey: KEY });
      const before = qc.getQueryData<Task[]>(KEY);
      qc.setQueryData<Task[]>(KEY, (list) =>
        list?.map((x) =>
          x.id === taskId ? { ...x, status: "DONE", completedAt: new Date().toISOString() } : x,
        ),
      );
      return { before };
    },
    onSuccess: (_d, { roomCode }) => {
      if (roomCode) toast.success(tf("clean.cleaned", { room: roomCode }));
      void qc.invalidateQueries({ queryKey: ["rooms"] });
      void qc.invalidateQueries({ queryKey: ["buildings"] });
    },
    onError: (_e, { roomCode }, ctx) => {
      qc.setQueryData(KEY, ctx?.before);
      toast.error(tf("clean.failed", { room: roomCode ?? "" }));
    },
    onSettled: () => qc.invalidateQueries({ queryKey: KEY }),
  });
}

// 409 ROOM_OCCUPIED: a guest is in the room, so it cannot be locked.
export class RoomOccupiedError extends Error {}

export function useReportDamage(roomId: string) {
  return useMutation({
    mutationFn: async ({ body, key }: { body: Schemas["DamageReportRequest"]; key: string }) => {
      const { data, error } = await api.POST("/v1/rooms/{roomId}/damage-reports", {
        params: { path: { roomId }, header: idempotencyHeader(key) },
        body,
      });
      if ((error as { code?: string } | undefined)?.code === "ROOM_OCCUPIED")
        throw new RoomOccupiedError();
      if (error || !data) throw new Error("reportDamage failed");
      return data;
    },
  });
}

export function useReportUsage(roomId: string) {
  return useMutation({
    mutationFn: async ({ note, key }: { note: string; key: string }) => {
      const { data, error } = await api.POST("/v1/rooms/{roomId}/usage-reports", {
        params: { path: { roomId }, header: idempotencyHeader(key) },
        body: { note },
      });
      if (error || !data) throw new Error("reportRoomUsage failed");
      return data;
    },
  });
}
