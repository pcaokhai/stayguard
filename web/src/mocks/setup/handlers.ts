import { getStayGuardAPIMock } from "../generated/api.msw";
import { demoHandlers } from "./demo-handlers";
import { ownerHandlers } from "./owner-handlers";

// Owner fixtures first (they replace thinner demo data of the same route), then the rest of the hand-written demo data, then every operation of the contract from generated data.
export const handlers = [...ownerHandlers, ...demoHandlers, ...getStayGuardAPIMock()];
