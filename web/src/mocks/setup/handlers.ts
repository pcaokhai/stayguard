import { getStayGuardAPIMock } from "../generated/api.msw";
import { demoHandlers } from "./demo-handlers";
import { ownerHandlers } from "./owner-handlers";

// Hand-written demo data first, then every operation of the contract from generated data.
export const handlers = [...demoHandlers, ...ownerHandlers, ...getStayGuardAPIMock()];
