import { t, tf } from "../../lib/t";
import { RateLimitedError, WrongCredentialsError } from "./hooks";

export function signInErrorText(e: unknown): string {
  if (e instanceof RateLimitedError)
    return e.retryAfterSeconds
      ? tf("auth.rateLimited", { n: e.retryAfterSeconds })
      : t("auth.rateLimitedGeneric");
  return e instanceof WrongCredentialsError ? t("auth.wrong") : t("auth.failed");
}
