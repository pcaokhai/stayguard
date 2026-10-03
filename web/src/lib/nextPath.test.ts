import { expect, test } from "vitest";
import { safeNext } from "./nextPath";

test("return_path_is_same_app_only_FU", () => {
  expect(safeNext("/vi/clean?room=r1", "/vi")).toBe("/vi/clean?room=r1");
  expect(safeNext("/vi/owner/staff", "/vi")).toBe("/vi/owner/staff");
  for (const bad of [
    "https://evil.example/vi/x",
    "//evil.example/vi/",
    "/vi/\\evil",
    "/en/rooms",
    "javascript:alert(1)",
    "/vi/a://b",
    "/vi/%0d",
    "",
    null,
  ])
    expect(safeNext(bad as string | null, "/vi"), String(bad)).toBeNull();
});
