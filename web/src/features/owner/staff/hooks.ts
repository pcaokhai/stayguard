import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { components } from "@/api/generated/schema";
import { api, idempotencyHeader } from "@/lib/api";

export type Staff = components["schemas"]["Staff"];
export type CreateStaff = components["schemas"]["CreateStaffRequest"];
export type UpdateStaff = components["schemas"]["UpdateStaffRequest"];
export type OneTimePin = components["schemas"]["OneTimePin"];
export type Level = components["schemas"]["PermissionLevel"];

export function useStaff(enabled = true) {
  return useQuery({
    queryKey: ["staff"],
    enabled,
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/owner/staff", { params: { query: {} } });
      if (error || !data) throw new Error("listStaff failed");
      return data.items;
    },
  });
}

// Errors carry the HTTP status so the dialogs can word 409 and 422 differently.
const fail = (op: string, status: number) => Object.assign(new Error(`${op} failed`), { status });
export const statusOf = (e: unknown) => (e as { status?: number }).status;

function useRefresh() {
  const qc = useQueryClient();
  return () => {
    void qc.invalidateQueries({ queryKey: ["staff"] });
    void qc.invalidateQueries({ queryKey: ["staff-permissions"] });
  };
}

export function useCreateStaff() {
  const refresh = useRefresh();
  return useMutation({
    mutationFn: async (v: { body: CreateStaff; key: string }) => {
      const { data, error, response } = await api.POST("/v1/owner/staff", {
        params: { header: idempotencyHeader(v.key) },
        body: v.body,
      });
      if (error || !data) throw fail("createStaff", response.status);
      return data;
    },
    onSuccess: refresh,
  });
}

export function useUpdateStaff() {
  const refresh = useRefresh();
  return useMutation({
    mutationFn: async (v: { userId: string; body: UpdateStaff }) => {
      const { data, error, response } = await api.PATCH("/v1/owner/staff/{userId}", {
        params: { path: { userId: v.userId } },
        body: v.body,
      });
      if (error || !data) throw fail("updateStaff", response.status);
      return data;
    },
    onSuccess: refresh,
  });
}

export function useResetPin() {
  const refresh = useRefresh();
  return useMutation({
    mutationFn: async (v: { userId: string; key: string }) => {
      const { data, error, response } = await api.POST("/v1/owner/staff/{userId}/pin-reset", {
        params: { path: { userId: v.userId }, header: idempotencyHeader(v.key) },
      });
      if (error || !data) throw fail("resetStaffPin", response.status);
      return data;
    },
    onSuccess: refresh,
  });
}

export function useLockToggle() {
  const refresh = useRefresh();
  return useMutation({
    mutationFn: async (v: { userId: string; lock: boolean }) => {
      const path = v.lock ? "/v1/owner/staff/{userId}/lock" : "/v1/owner/staff/{userId}/unlock";
      const { error, response } = await api.POST(path, { params: { path: { userId: v.userId } } });
      if (error) throw fail("lockStaff", response.status);
    },
    onSuccess: refresh,
  });
}

export function useRemoveStaff() {
  const refresh = useRefresh();
  return useMutation({
    mutationFn: async (v: { userId: string; ownerPin: string }) => {
      const { error, response } = await api.POST("/v1/owner/staff/{userId}/remove", {
        params: { path: { userId: v.userId } },
        body: { ownerPin: v.ownerPin },
      });
      if (error) throw fail("removeStaff", response.status);
    },
    onSuccess: refresh,
  });
}

export function useStaffPermissions() {
  return useQuery({
    queryKey: ["staff-permissions"],
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/owner/staff-permissions");
      if (error || !data) throw new Error("listStaffPermissions failed");
      return data.items;
    },
  });
}

export function useSetPermission() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: { userId: string; buildingId: string; level: Level }) => {
      const { data, error, response } = await api.PUT(
        "/v1/owner/staff/{userId}/building-permissions/{buildingId}",
        {
          params: { path: { userId: v.userId, buildingId: v.buildingId } },
          body: { level: v.level },
        },
      );
      if (error || !data) throw fail("setBuildingPermission", response.status);
      return data;
    },
    onSettled: () => {
      void qc.invalidateQueries({ queryKey: ["staff-permissions"] });
      void qc.invalidateQueries({ queryKey: ["staff"] });
    },
  });
}
