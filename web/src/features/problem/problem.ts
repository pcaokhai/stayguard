import { t, tf, type MessageKey } from "@/lib/t";

// The API answers every failure with a Problem body whose `code` is stable (contract: Problem.code).
// Handlers map by that code, never by the HTTP status: several different failures share 409.
export class ApiProblem extends Error {
  constructor(
    readonly op: string,
    readonly status: number,
    readonly code: string,
    readonly traceId?: string,
  ) {
    super(`${op} failed: ${code}`);
  }
}

// Builds the error to throw from an openapi-fetch result (`error` is the parsed Problem body).
export function problem(op: string, error: unknown, response: Response): ApiProblem {
  const body = (error ?? {}) as { code?: unknown; traceId?: unknown };
  return new ApiProblem(
    op,
    response.status,
    typeof body.code === "string" ? body.code : "UNKNOWN",
    typeof body.traceId === "string" ? body.traceId : undefined,
  );
}

export const codeOf = (e: unknown): string | undefined =>
  e instanceof ApiProblem ? e.code : undefined;

// Codes that read the same wherever they come from.
const COMMON: Record<string, MessageKey> = {
  ROLE_FORBIDDEN: "problem.forbidden",
  BUILDING_FORBIDDEN: "problem.forbidden",
  NOT_FOUND: "problem.notFound",
  INTERNAL: "problem.server",
  RATE_LIMITED: "problem.rate",
  IDEMPOTENCY_KEY_REUSED: "problem.again",
};

// Text for a failed action: the handler's own map first, then the common codes. An unknown code shows a
// generic line with the trace id so support can find the request; a network failure says so.
export function messageFor(e: unknown, specific: Partial<Record<string, MessageKey>> = {}): string {
  if (!(e instanceof ApiProblem)) return t("problem.network");
  const key = specific[e.code] ?? COMMON[e.code];
  return key ? t(key) : tf("problem.unknown", { id: e.traceId ?? "—" });
}
