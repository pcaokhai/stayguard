import { useMutation } from "@tanstack/react-query";
import type { components } from "@/api/generated/schema";
import { api } from "@/lib/api";
import { saveSession } from "@/lib/session";

type SignInBody = components["schemas"]["SignInRequest"];
export type SignInResult = components["schemas"]["SignInResponse"];

// Five wrong PINs lock the account for 15 minutes (docs/15 rule 12): problem code ACCOUNT_LOCKED, the only locked screen.
// Too many sign-in requests (429, RATE_LIMITED) is a different, short wait with its own message.
const LOCK_MINUTES = 15;

export class WrongCredentialsError extends Error {}
export class LockedError extends Error {
  constructor(readonly until: Date) {
    super("account locked");
  }
}

export class RateLimitedError extends Error {
  constructor(readonly retryAfterSeconds?: number) {
    super("rate limited");
  }
}

export function signInFailure(code: string | undefined, retryAfter: string | null): Error {
  const wait = Number(retryAfter);
  if (code === "ACCOUNT_LOCKED")
    return new LockedError(new Date(Date.now() + (wait > 0 ? wait * 1000 : LOCK_MINUTES * 60_000)));
  if (code === "RATE_LIMITED") return new RateLimitedError(wait > 0 ? Math.ceil(wait) : undefined);
  if (code === "PIN_INVALID" || code === "VALIDATION_FAILED") return new WrongCredentialsError();
  return new Error("signIn failed");
}

export function useSignIn() {
  return useMutation({
    mutationFn: async (body: SignInBody): Promise<SignInResult> => {
      const { data, error, response } = await api.POST("/v1/auth/sign-in", { body });
      if (data) {
        saveSession(data);
        return data;
      }
      const code = (error as { code?: string } | undefined)?.code;
      throw signInFailure(code, response.headers.get("Retry-After"));
    },
  });
}

export function useChangePin() {
  return useMutation({
    mutationFn: async (body: components["schemas"]["ChangePinRequest"]) => {
      const { error } = await api.PUT("/v1/me/pin", { body });
      if (error) throw new Error("changeMyPin failed");
    },
  });
}

// The one-time PIN stays in memory only (never stored) so the first change needs no second prompt.
let pendingPin: string | null = null;
export const rememberPin = (pin: string) => {
  pendingPin = pin;
};
export const peekPin = () => pendingPin;
export const forgetPin = () => {
  pendingPin = null;
};
