import { useMe } from "../session/useMe";

// Owner-only controls (docs/15 §2) are hidden for a MANAGER; the API still refuses them with 403.
export const useIsOwner = () => useMe().data?.user.role === "OWNER";
