import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, idempotencyHeader } from "../../lib/api";
import type { components } from "../../api/generated/schema";

type Schemas = components["schemas"];
const must = <T>(data: T | undefined, error: unknown, op: string): T => {
  if (error || data === undefined) throw new Error(`${op} failed`);
  return data;
};

export function useRoom(roomId: string | null) {
  return useQuery({
    queryKey: ["room", roomId],
    enabled: !!roomId,
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/rooms/{roomId}", {
        params: { path: { roomId: roomId! } },
      });
      return must(data, error, "getRoom");
    },
  });
}

// Running total comes from the API; refetch once a minute (docs/14 W3).
export function useStay(stayId: string | null) {
  return useQuery({
    queryKey: ["stay", stayId],
    enabled: !!stayId,
    refetchInterval: 60_000,
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/stays/{stayId}", {
        params: { path: { stayId: stayId! } },
      });
      return must(data, error, "getStay");
    },
  });
}

export function useServices() {
  return useQuery({
    queryKey: ["services"],
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/services");
      return must(data, error, "listServices").items;
    },
  });
}

export function useCreateStay(roomId: string) {
  return useMutation({
    mutationFn: async ({ body, key }: { body: Schemas["CreateStayRequest"]; key: string }) => {
      const { data, error } = await api.POST("/v1/rooms/{roomId}/stays", {
        params: { path: { roomId }, header: idempotencyHeader(key) },
        body,
      });
      return must(data, error, "createStay");
    },
  });
}

export function useAddExtras(stayId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ body, key }: { body: Schemas["AddExtrasRequest"]; key: string }) => {
      const { data, error } = await api.POST("/v1/stays/{stayId}/extras", {
        params: { path: { stayId }, header: idempotencyHeader(key) },
        body,
      });
      return must(data, error, "addStayExtras");
    },
    onSuccess: (stay) => {
      qc.setQueryData(["stay", stayId], stay);
      qc.invalidateQueries({ queryKey: ["services"] });
    },
  });
}

export function useCheckout(stayId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (key: string) => {
      const { data, error } = await api.POST("/v1/stays/{stayId}/checkout", {
        params: { path: { stayId }, header: idempotencyHeader(key) },
      });
      return must(data, error, "checkoutStay");
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["stay", stayId] }),
  });
}

export function useCreatePayment(invoiceId: string) {
  return useMutation({
    mutationFn: async ({ method, key }: { method: Schemas["PaymentMethod"]; key: string }) => {
      const { data, error } = await api.POST("/v1/invoices/{invoiceId}/payments", {
        params: { path: { invoiceId }, header: idempotencyHeader(key) },
        body: { method },
      });
      return must(data, error, "createPayment");
    },
  });
}

// Both actions return the updated Stay (new quote); the stay and room lists are refetched.
function useRefreshAfter(stayId: string) {
  const qc = useQueryClient();
  return (stay: Schemas["Stay"]) => {
    qc.setQueryData(["stay", stayId], stay);
    void qc.invalidateQueries({ queryKey: ["rooms"] });
    void qc.invalidateQueries({ queryKey: ["buildings"] });
  };
}

export function useEditCheckIn(stayId: string) {
  const refresh = useRefreshAfter(stayId);
  return useMutation({
    mutationFn: async ({ body, key }: { body: Schemas["EditCheckInRequest"]; key: string }) => {
      const { data, error } = await api.POST("/v1/stays/{stayId}/check-in-time", {
        params: { path: { stayId }, header: idempotencyHeader(key) },
        body,
      });
      return must(data, error, "editCheckInTime");
    },
    onSuccess: refresh,
  });
}

export function useMoveStay(stayId: string) {
  const refresh = useRefreshAfter(stayId);
  return useMutation({
    mutationFn: async ({ body, key }: { body: Schemas["MoveStayRequest"]; key: string }) => {
      const { data, error } = await api.POST("/v1/stays/{stayId}/move", {
        params: { path: { stayId }, header: idempotencyHeader(key) },
        body,
      });
      return must(data, error, "moveStay");
    },
    onSuccess: refresh,
  });
}

export function useReceipt(invoiceId: string | null) {
  return useQuery({
    queryKey: ["receipt", invoiceId],
    enabled: !!invoiceId,
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/invoices/{invoiceId}/receipt", {
        params: { path: { invoiceId: invoiceId! } },
      });
      return must(data, error, "getReceipt");
    },
  });
}
