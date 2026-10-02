import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { components } from "../../api/generated/schema";
import { api, idempotencyHeader } from "../../lib/api";

type Schemas = components["schemas"];
export type LeaveRequest = Schemas["LeaveRequest"];
export type ShiftCode = Schemas["ShiftCode"];

export function useMyRoster(from: string, to: string) {
  return useQuery({
    queryKey: ["my-roster", from, to],
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/me/roster", { params: { query: { from, to } } });
      if (error || !data) throw new Error("getMyRoster failed");
      return data;
    },
  });
}

export function useMyLeave() {
  return useQuery({
    queryKey: ["my-leave"],
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/me/leave-requests");
      if (error || !data) throw new Error("listMyLeaveRequests failed");
      return data;
    },
  });
}

export function useCreateLeave() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ body, key }: { body: Schemas["CreateLeaveRequest"]; key: string }) => {
      const { data, error } = await api.POST("/v1/me/leave-requests", {
        params: { header: idempotencyHeader(key) },
        body,
      });
      if (error || !data) throw new Error("createLeaveRequest failed");
      return data;
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["my-leave"] });
      void qc.invalidateQueries({ queryKey: ["my-roster"] });
    },
  });
}

// PENDING becomes CANCELLED at once; APPROVED becomes CANCEL_REQUESTED until the owner agrees.
export function useCancelLeave() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ leaveId, key }: { leaveId: string; key: string }) => {
      const { data, error } = await api.POST("/v1/me/leave-requests/{leaveId}/cancel", {
        params: { path: { leaveId }, header: idempotencyHeader(key) },
      });
      if (error || !data) throw new Error("cancelMyLeave failed");
      return data;
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["my-leave"] });
      void qc.invalidateQueries({ queryKey: ["my-roster"] });
    },
  });
}

// ponytail: fixed windows as on the design boards; per-property shift times can replace them later.
export const SHIFT_HOURS: Record<ShiftCode, string> = {
  MORNING: "06:00 – 14:00",
  AFTERNOON: "14:00 – 22:00",
  NIGHT: "22:00 – 06:00",
};
