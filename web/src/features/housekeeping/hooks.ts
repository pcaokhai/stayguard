import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, idempotencyHeader } from "../../lib/api";

export function useTasks() {
  return useQuery({
    queryKey: ["hk-tasks"],
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/housekeeping/tasks");
      if (error || !data) throw new Error("listHousekeepingTasks failed");
      return data.items;
    },
  });
}

export function useCompleteTask() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ taskId, key }: { taskId: string; key: string }) => {
      const { error } = await api.POST("/v1/housekeeping/tasks/{taskId}/complete", {
        params: { path: { taskId }, header: idempotencyHeader(key) },
      });
      if (error) throw new Error("completeHousekeepingTask failed");
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["hk-tasks"] }),
  });
}
