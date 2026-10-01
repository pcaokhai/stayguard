import { useMutation } from "@tanstack/react-query";
import { api } from "../../lib/api";
import { clearSession, loadSession, markDemo, saveSession } from "../../lib/session";
import type { components } from "../../api/generated/schema";

type Role = components["schemas"]["Role"];

const start = (role: Role, tenantId?: string) =>
  api.POST("/v1/demo/sessions", { body: { role, locale: "vi", tenantId } });

export function useCreateDemoSession() {
  return useMutation({
    mutationFn: async (role: Role) => {
      // Switching role keeps the same trial tenant so the demo story carries over.
      let res = await start(role, loadSession()?.tenantId);
      // The saved trial is gone (expired or reset): start a new one instead of getting stuck.
      if (res.response.status === 404) {
        clearSession();
        res = await start(role);
      }
      if (res.error || !res.data) throw new Error("createDemoSession failed");
      saveSession(res.data);
      markDemo();
      return res.data;
    },
  });
}
