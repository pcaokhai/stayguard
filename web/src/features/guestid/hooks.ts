import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { components } from "../../api/generated/schema";
import { api, idempotencyHeader, newIdempotencyKey } from "../../lib/api";

export type Side = "FRONT" | "BACK";
type Indicators = components["schemas"]["GuestIdIndicators"];

// Guest ID data never goes into logs, query keys or storage: the number lives in component state
// only, photos are fetched as blobs and shown through object URLs the caller revokes (rules 21-25).
export function useGuestIdRecord(stayId: string) {
  return useQuery({
    queryKey: ["guest-id", stayId],
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/owner/stays/{stayId}/guest-id", {
        params: { path: { stayId } },
      });
      if (error || !data) throw new Error("getGuestIdRecord failed");
      return data;
    },
  });
}

export function useRevealIdNumber(stayId: string) {
  return useMutation({
    mutationFn: async () => {
      const { data, error } = await api.POST("/v1/owner/stays/{stayId}/guest-id/reveal", {
        params: { path: { stayId } },
      });
      if (error || !data) throw new Error("revealGuestIdNumber failed");
      return data.idNumber;
    },
  });
}

export async function fetchPhoto(stayId: string, side: Side, download = false): Promise<Blob> {
  const { data, error } = await api.GET("/v1/owner/stays/{stayId}/guest-id/photos/{side}", {
    params: { path: { stayId, side }, query: { download } },
    parseAs: "blob",
    cache: "no-store",
  });
  if (error || !data) throw new Error("getGuestIdPhoto failed");
  return data as Blob;
}

export function useDeleteGuestId(stayId: string) {
  const qc = useQueryClient();
  const done = () => qc.invalidateQueries({ queryKey: ["guest-id", stayId] });
  return {
    photo: useMutation({
      mutationFn: async (side: Side) => {
        const { error } = await api.DELETE("/v1/owner/stays/{stayId}/guest-id/photos/{side}", {
          params: { path: { stayId, side } },
        });
        if (error) throw new Error("deleteGuestIdPhoto failed");
      },
      onSuccess: done,
    }),
    number: useMutation({
      mutationFn: async () => {
        const { error } = await api.DELETE("/v1/owner/stays/{stayId}/guest-id/number", {
          params: { path: { stayId } },
        });
        if (error) throw new Error("deleteGuestIdNumber failed");
      },
      onSuccess: done,
    }),
  };
}

// Front desk side: write-only. Multipart upload; the server strips metadata and encrypts.
export async function uploadIdPhoto(stayId: string, side: Side, file: File): Promise<Indicators> {
  const { data, error, response } = await api.PUT("/v1/stays/{stayId}/guest-id/photos/{side}", {
    params: { path: { stayId, side }, header: idempotencyHeader(newIdempotencyKey()) },
    body: { file: file as unknown as string },
    bodySerializer: (b) => {
      const fd = new FormData();
      fd.set("file", b.file as unknown as Blob);
      return fd;
    },
  });
  if (error || !data) {
    const code = (error as { code?: string } | undefined)?.code;
    throw new Error(code ?? `uploadGuestIdPhoto ${response.status}`);
  }
  return data;
}
