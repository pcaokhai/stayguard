import { getStayGuardAPIMock } from "../generated/api.msw";

// Every operation of the contract, answered from the spec's examples and generated data.
export const handlers = getStayGuardAPIMock();
