import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { components } from "@/api/generated/schema";
import { problem } from "../../problem/problem";
import { api, idempotencyHeader } from "@/lib/api";

export type Property = components["schemas"]["Property"];
export type BankAccount = components["schemas"]["BankAccount"];
export type UpdateProperty = components["schemas"]["UpdatePropertyRequest"];

export function useProperty() {
  return useQuery({
    queryKey: ["property"],
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/owner/property");
      if (error || !data) throw new Error("getProperty failed");
      return data;
    },
  });
}

export function useUpdateProperty() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (body: UpdateProperty) => {
      const { data, error, response } = await api.PATCH("/v1/owner/property", { body });
      if (error || !data) throw problem("updateProperty", error, response);
      return data;
    },
    onSuccess: (data) => qc.setQueryData(["property"], data),
  });
}

export function useBankAccounts(enabled = true) {
  return useQuery({
    queryKey: ["bank-accounts"],
    enabled,
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/owner/bank-accounts");
      if (error || !data) throw new Error("listBankAccounts failed");
      return data.items;
    },
  });
}

export function useSepayStatus() {
  return useQuery({
    queryKey: ["sepay-status"],
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/owner/sepay-status");
      if (error || !data) throw new Error("getSepayStatus failed");
      return data;
    },
  });
}

function useBankRefresh() {
  const qc = useQueryClient();
  return () => void qc.invalidateQueries({ queryKey: ["bank-accounts"] });
}

// The account number and PIN go out once in the request body; they are never stored client-side.
export function useCreateBank() {
  const refresh = useBankRefresh();
  return useMutation({
    mutationFn: async (v: {
      key: string;
      body: components["schemas"]["CreateBankAccountRequest"];
    }) => {
      const { data, error, response } = await api.POST("/v1/owner/bank-accounts", {
        params: { header: idempotencyHeader(v.key) },
        body: v.body,
      });
      if (error || !data) throw problem("createBankAccount", error, response);
      return data;
    },
    onSuccess: refresh,
  });
}

export function useMakeDefaultBank() {
  const refresh = useBankRefresh();
  return useMutation({
    mutationFn: async (v: { accountId: string; ownerPin: string }) => {
      const { error, response } = await api.POST(
        "/v1/owner/bank-accounts/{accountId}/make-default",
        {
          params: { path: { accountId: v.accountId } },
          body: { ownerPin: v.ownerPin },
        },
      );
      if (error) throw problem("makeDefaultBankAccount", error, response);
    },
    onSuccess: refresh,
  });
}

export function useRemoveBank() {
  const refresh = useBankRefresh();
  return useMutation({
    mutationFn: async (v: { accountId: string; ownerPin: string }) => {
      const { error, response } = await api.POST("/v1/owner/bank-accounts/{accountId}/remove", {
        params: { path: { accountId: v.accountId } },
        body: { ownerPin: v.ownerPin },
      });
      if (error) throw problem("removeBankAccount", error, response);
    },
    onSuccess: refresh,
  });
}
