import { afterEach, expect, test } from "vitest";
import { setLocale } from "../../lib/locale";
import { LockedError, RateLimitedError, WrongCredentialsError, signInFailure } from "./hooks";
import { signInErrorText } from "./signInErrors";

afterEach(() => setLocale("vi"));

test("signin_429_is_rate_limited_with_retry_after_FU", () => {
  const e = signInFailure("RATE_LIMITED", "30");
  expect(e).toBeInstanceOf(RateLimitedError);
  expect(e).not.toBeInstanceOf(LockedError);
  expect(signInErrorText(e)).toBe("Thử quá nhiều lần. Đợi 30 giây rồi thử lại.");
  setLocale("en");
  expect(signInErrorText(e)).toBe("Too many attempts. Wait 30 seconds and try again.");
});

test("signin_429_without_retry_after_is_generic_FU", () => {
  for (const header of [null, "", "abc", "0", "-5"]) {
    const e = signInFailure("RATE_LIMITED", header);
    expect(e).toBeInstanceOf(RateLimitedError);
    expect(signInErrorText(e)).toBe("Thử quá nhiều lần. Đợi một lúc rồi thử lại.");
  }
});

test("locked_screen_only_for_account_locked_FU", () => {
  const locked = signInFailure("ACCOUNT_LOCKED", "900");
  expect(locked).toBeInstanceOf(LockedError);
  expect((locked as LockedError).until.getTime()).toBeGreaterThan(Date.now() + 800_000);
  expect(signInFailure("PIN_INVALID", null)).toBeInstanceOf(WrongCredentialsError);
  expect(signInFailure("SOMETHING_ELSE", null)).not.toBeInstanceOf(LockedError);
});
