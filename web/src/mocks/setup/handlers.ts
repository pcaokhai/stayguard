import { getStayGuardAPIMock } from "../generated/api.msw";
import { demoHandlers } from "./demo-handlers";

// Hand-written demo data first, then every operation of the contract from generated data.
export const handlers = [...demoHandlers, ...getStayGuardAPIMock()];
