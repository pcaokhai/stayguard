import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { components } from "@/api/generated/schema";
import { problem } from "../../problem/problem";
import { api, idempotencyHeader, newIdempotencyKey } from "@/lib/api";

type S = components["schemas"];
export type Roster = S["Roster"];
export type Assignment = S["RosterAssignment"];
export type Leave = S["LeaveRequest"];
export type ShiftCode = S["ShiftCode"];

export function useRoster(from: string, to: string) {
  return useQuery({
    queryKey: ["roster", from, to],
    placeholderData: (prev) => prev,
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/owner/roster", {
        params: { query: { from, to } },
      });
      if (error || !data) throw new Error("getRoster failed");
      return data;
    },
  });
}

// Each toggle is its own action, so each gets its own Idempotency-Key.
export function usePutRoster() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: { set: Assignment[]; remove: Assignment[] }) => {
      const { data, error, response } = await api.PUT("/v1/owner/roster", {
        params: { header: idempotencyHeader(newIdempotencyKey()) },
        body: v,
      });
      if (error || !data) throw problem("putRoster", error, response);
      return data;
    },
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["roster"] }),
  });
}

export function useCopyWeek() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (weekStart: string) => {
      const { data, error, response } = await api.POST("/v1/owner/roster/copy-week", {
        params: { header: idempotencyHeader(newIdempotencyKey()) },
        body: { weekStart },
      });
      if (error || !data) throw problem("copyRosterWeek", error, response);
      return data;
    },
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["roster"] }),
  });
}

export function usePendingLeave() {
  return useQuery({
    queryKey: ["leave-pending"],
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/owner/leave-requests", {
        params: { query: { status: "PENDING" } },
      });
      if (error || !data) throw new Error("listLeaveRequests failed");
      return data.items;
    },
  });
}

export function useDecideLeave() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: { id: string; reason?: string }) => {
      const header = idempotencyHeader(newIdempotencyKey());
      if (v.reason !== undefined) {
        const { error, response } = await api.POST("/v1/owner/leave-requests/{leaveId}/decline", {
          params: { path: { leaveId: v.id }, header },
          body: { reason: v.reason },
        });
        if (error) throw problem("declineLeave", error, response);
        return;
      }
      const { error, response } = await api.POST("/v1/owner/leave-requests/{leaveId}/approve", {
        params: { path: { leaveId: v.id }, header },
      });
      if (error) throw problem("approveLeave", error, response);
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["leave-pending"] });
      void qc.invalidateQueries({ queryKey: ["roster"] });
    },
  });
}
