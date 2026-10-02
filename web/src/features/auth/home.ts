import type { components } from "@/api/generated/schema";

type Role = components["schemas"]["Role"];

// Where each role lands after sign-in.
export const homeFor = (role: Role): string =>
  role === "OWNER"
    ? "/owner"
    : role === "MANAGER"
      ? "/owner/rooms"
      : role === "HOUSEKEEPING"
        ? "/housekeeping"
        : "/rooms";
