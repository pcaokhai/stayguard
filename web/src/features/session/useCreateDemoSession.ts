import { useMutation } from "@tanstack/react-query";
import { api } from "../../lib/api";
import { loadSession, markDemo, saveSession } from "../../lib/session";
import type { components } from "../../api/generated/schema";

type Role = components["schemas"]["Role"];

export function useCreateDemoSession() {
  return useMutation({
    mutationFn: async (role: Role) => {
      const { data, error } = await api.POST("/v1/demo/sessions", {
        // Switching role keeps the same trial tenant so the demo story carries over.
        body: { role, locale: "vi", tenantId: loadSession()?.tenantId },
      });
      if (error || !data) throw new Error("createDemoSession failed");
      saveSession(data);
      markDemo();
      return data;
    },
  });
}
