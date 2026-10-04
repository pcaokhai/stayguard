import { expect, test } from "vitest";
import { backdateRefusal } from "./backdateGuard";

const ok = (project: string, code = "smoke1a2b3c", talked = project) =>
  backdateRefusal({ SMOKE_COMPOSE_PROJECT: project, SMOKE_GUESTHOUSE: code }, talked);

test("backdate_allowed_in_the_smoke_project_and_in_the_demo_check_project_it_started_FU", () => {
  expect(ok("stayguard-smoke")).toBeNull();
  expect(ok("stayguard-web1exp")).toBeNull();
  expect(ok("stayguard-demo-check")).toBeNull(); // demo-check runs make smoke there and exports exactly this
});

test("backdate_refused_on_rehearse_and_on_other_demo_stacks_FU", () => {
  expect(ok("stayguard-rehearse")).toMatch(/rehearsal/);
  expect(ok("stayguard-smoke", "smoke1a2b3c", "stayguard-rehearse")).toMatch(/rehearsal/);
  expect(ok("stayguard-smoke", "smoke1a2b3c", "stayguard-demo")).toMatch(/not the one/);
  expect(ok("stayguard-smoke", "smoke1a2b3c", "stayguard-demo-check")).toMatch(/not the one/);
});

test("backdate_refused_without_the_variable_or_a_smoke_guesthouse_FU", () => {
  expect(ok("")).toMatch(/SMOKE_COMPOSE_PROJECT/);
  expect(backdateRefusal({}, "stayguard-smoke")).toMatch(/SMOKE_COMPOSE_PROJECT/);
  expect(ok("stayguard-smoke", "demo")).toMatch(/guesthouse/);
  expect(ok("stayguard-smoke", "rehearse")).toMatch(/guesthouse/);
});
