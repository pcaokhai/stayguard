import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, describe, expect, test } from "vitest";
import { createApiClient } from "@/lib/api";
import { setLocale } from "@/lib/locale";
import { dismissErrorMessage, noteProblem, NOTE_MAX } from "./dismiss";
import { dismissTransfer } from "./hooks";

const server = setupServer();
beforeAll(() => server.listen({ onUnhandledFrame: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());
const client = () => createApiClient("http://localhost");

describe("the note", () => {
  test("is required once trimmed", () => {
    expect(noteProblem("")).toBe("required");
    expect(noteProblem("   \n")).toBe("required");
  });
  test("is at most 500 characters", () => {
    expect(noteProblem("a".repeat(NOTE_MAX))).toBeNull();
    expect(noteProblem("a".repeat(NOTE_MAX + 1))).toBe("tooLong");
  });
  test("a normal note passes", () => expect(noteProblem("Khách chuyển nhầm")).toBeNull());
});

describe("dismissTransfer", () => {
  test("posts the trimmed note with the idempotency key to the event", async () => {
    let seen: { path: string; key: string | null; body: unknown } | null = null;
    server.use(
      http.post("*/v1/owner/payment-events/:id/dismiss", async ({ request, params }) => {
        seen = {
          path: String(params.id),
          key: request.headers.get("Idempotency-Key"),
          body: await request.json(),
        };
        return HttpResponse.json({ eventId: "e1", result: "DISMISSED", note: "x" });
      }),
    );
    await dismissTransfer(
      { eventId: "e1", note: "  Chuyển nhầm  ", key: "6f1c3c0e-3b5e-4c8a-9d52-1b1d1a2b3c4d" },
      client(),
    );
    expect(seen).toEqual({
      path: "e1",
      key: "6f1c3c0e-3b5e-4c8a-9d52-1b1d1a2b3c4d",
      body: { note: "Chuyển nhầm" },
    });
  });

  test.each([
    ["EVENT_NOT_DISMISSABLE", 409, "Không thể bỏ qua", "can no longer be dismissed"],
    ["ROLE_FORBIDDEN", 403, "không có quyền", "not allowed"],
    ["NOT_FOUND", 404, "Không tìm thấy", "could not find"],
  ])("%s is mapped by code", async (code, status, vi, en) => {
    server.use(
      http.post("*/v1/owner/payment-events/:id/dismiss", () =>
        HttpResponse.json({ type: "about:blank", title: "x", status, code }, { status }),
      ),
    );
    const err = await dismissTransfer({ eventId: "e1", note: "n", key: "k" }, client()).then(
      () => null,
      (e: unknown) => e,
    );
    expect(err).not.toBeNull();
    setLocale("vi");
    expect(dismissErrorMessage(err)).toContain(vi);
    setLocale("en");
    expect(dismissErrorMessage(err)).toContain(en);
  });

  test("an unknown 409 code does not claim the transfer is not dismissable", async () => {
    server.use(
      http.post("*/v1/owner/payment-events/:id/dismiss", () =>
        HttpResponse.json(
          { type: "about:blank", title: "x", status: 409, code: "NEW_ONE", traceId: "tr-9" },
          { status: 409 },
        ),
      ),
    );
    const err = await dismissTransfer({ eventId: "e1", note: "n", key: "k" }, client()).then(
      () => null,
      (e: unknown) => e,
    );
    setLocale("vi");
    const msg = dismissErrorMessage(err);
    expect(msg).toContain("tr-9");
    expect(msg).not.toContain("Không thể bỏ qua");
  });
});
