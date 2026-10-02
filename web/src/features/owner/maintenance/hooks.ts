import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { components } from "@/api/generated/schema";
import { api } from "@/lib/api";

export type Ticket = components["schemas"]["MaintenanceTicket"];
export type TicketStatus = components["schemas"]["TicketStatus"];
export type UpdateTicket = components["schemas"]["UpdateTicketRequest"];

export function useTickets() {
  return useQuery({
    queryKey: ["tickets"],
    refetchInterval: 60_000,
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/owner/maintenance-tickets", {
        params: { query: {} },
      });
      if (error || !data) throw new Error("listTickets failed");
      return data.items;
    },
  });
}

export function useUpdateTicket() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: { id: string; body: UpdateTicket }) => {
      const { data, error, response } = await api.PATCH(
        "/v1/owner/maintenance-tickets/{ticketId}",
        {
          params: { path: { ticketId: v.id } },
          body: v.body,
        },
      );
      if (error || !data)
        throw Object.assign(new Error("updateTicket failed"), { status: response.status });
      return data;
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["tickets"] });
      void qc.invalidateQueries({ queryKey: ["buildings"] });
      void qc.invalidateQueries({ queryKey: ["rooms"] });
      void qc.invalidateQueries({ queryKey: ["owner-overview"] });
    },
  });
}
