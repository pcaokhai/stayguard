import { useQuery } from "@tanstack/react-query";
import { api } from "../../lib/api";

export function useMe() {
  return useQuery({
    queryKey: ["me"],
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/me");
      if (error || !data) throw new Error("getMe failed");
      return data;
    },
  });
}
