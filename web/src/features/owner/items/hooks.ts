import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { components } from "@/api/generated/schema";
import { api, idempotencyHeader } from "@/lib/api";

export type Service = components["schemas"]["Service"];
export type CreateService = components["schemas"]["CreateServiceRequest"];
export type UpdateService = components["schemas"]["UpdateServiceRequest"];

export function useItems() {
  return useQuery({
    queryKey: ["items"],
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/services");
      if (error || !data) throw new Error("listServices failed");
      return data.items;
    },
  });
}

const fail = (op: string, status: number) => Object.assign(new Error(`${op} failed`), { status });
export const statusOf = (e: unknown) => (e as { status?: number }).status;

export function useCreateItem() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: { key: string; body: CreateService }) => {
      const { data, error, response } = await api.POST("/v1/owner/services", {
        params: { header: idempotencyHeader(v.key) },
        body: v.body,
      });
      if (error || !data) throw fail("createService", response.status);
      return data;
    },
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["items"] }),
  });
}

export function useUpdateItem() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: { code: string; body: UpdateService }) => {
      const { data, error, response } = await api.PATCH("/v1/owner/services/{serviceCode}", {
        params: { path: { serviceCode: v.code } },
        body: v.body,
      });
      if (error || !data) throw fail("updateService", response.status);
      return data;
    },
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["items"] }),
  });
}
