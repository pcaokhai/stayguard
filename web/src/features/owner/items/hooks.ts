import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { components } from "@/api/generated/schema";
import { problem } from "../../problem/problem";
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

export function useCreateItem() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: { key: string; body: CreateService }) => {
      const { data, error, response } = await api.POST("/v1/owner/services", {
        params: { header: idempotencyHeader(v.key) },
        body: v.body,
      });
      if (error || !data) throw problem("createService", error, response);
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
      if (error || !data) throw problem("updateService", error, response);
      return data;
    },
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["items"] }),
  });
}

export type Movement = components["schemas"]["StockMovement"];

export function useMovements(code: string | null, kind?: Movement["kind"]) {
  return useQuery({
    queryKey: ["movements", code, kind],
    enabled: !!code,
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/owner/services/{serviceCode}/movements", {
        params: { path: { serviceCode: code! }, query: kind ? { kind } : {} },
      });
      if (error || !data) throw new Error("listStockMovements failed");
      return data.items;
    },
  });
}

// Stock only changes through restock, stocktake or a sale, so each leaves a history line.
export function useRestock() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: { code: string; key: string; quantity: number; unitCost: number }) => {
      const { data, error, response } = await api.POST("/v1/owner/services/{serviceCode}/restock", {
        params: { path: { serviceCode: v.code }, header: idempotencyHeader(v.key) },
        body: { quantity: v.quantity, unitCost: v.unitCost },
      });
      if (error || !data) throw problem("restockService", error, response);
      return data;
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["items"] });
      void qc.invalidateQueries({ queryKey: ["movements"] });
    },
  });
}

export function useRemoveItem() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: { code: string; key: string }) => {
      const { data, error, response } = await api.POST("/v1/owner/services/{serviceCode}/remove", {
        params: { path: { serviceCode: v.code }, header: idempotencyHeader(v.key) },
      });
      if (error || !data) throw problem("removeService", error, response);
      return data.result;
    },
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["items"] }),
  });
}

export function useStocktake() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: {
      key: string;
      lines: { serviceCode: string; counted: number }[];
      note: string | null;
    }) => {
      const { data, error, response } = await api.POST("/v1/stocktakes", {
        params: { header: idempotencyHeader(v.key) },
        body: { lines: v.lines, note: v.note },
      });
      if (error || !data) throw problem("createStocktake", error, response);
      return data;
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["items"] });
      void qc.invalidateQueries({ queryKey: ["movements"] });
    },
  });
}
