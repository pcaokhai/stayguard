import { renderToStaticMarkup } from "react-dom/server";
import { expect, test, vi } from "vitest";
import en from "../../messages/en.json";
import viMsg from "../../messages/vi.json";
import { RolePicker } from "../features/session/RolePicker";

// The router needs the app context; stub it for the static render.
vi.mock("next/navigation", () => ({ useRouter: () => ({ push: () => {} }) }));
vi.mock("../features/session/useCreateDemoSession", () => ({
  useCreateDemoSession: () => ({ isPending: false, isError: false, mutate: () => {} }),
}));

test("role_picker_shows_three_roles_W1", () => {
  const html = renderToStaticMarkup(<RolePicker />);
  for (const k of ["desk", "owner", "housekeeping"] as const) expect(html).toContain(viMsg.login[k]);
});

// CLAUDE.md §6 rule 12: both languages carry the same keys.
test("messages_en_vi_same_keys_SG001_AC3", () => {
  expect(Object.keys(viMsg.app).sort()).toEqual(Object.keys(en.app).sort());
  expect(Object.keys(viMsg.login).sort()).toEqual(Object.keys(en.login).sort());
});
