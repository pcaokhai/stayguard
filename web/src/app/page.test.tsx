import { renderToStaticMarkup } from "react-dom/server";
import { expect, test } from "vitest";
import en from "../../messages/en.json";
import vi from "../../messages/vi.json";
import Page from "./page";

test("placeholder_renders_english_message_SG001_AC3", () => {
  const html = renderToStaticMarkup(<Page />);
  expect(html).toContain(en.app.title);
  expect(html).toContain(en.app.tagline);
});

// CLAUDE.md §6 rule 12: both languages carry the same keys.
test("messages_en_vi_same_keys_SG001_AC3", () => {
  expect(Object.keys(vi.app).sort()).toEqual(Object.keys(en.app).sort());
});
