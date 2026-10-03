import { expect, type Page, test } from "@playwright/test";

// The room-map panel for a To-clean room offers the same four things as /clean (checklist, mark cleaned, set maintenance,
// report damage), for receptionist, owner and manager, on a phone (tap opens the page) and on a desktop (panel). A user
// without Edit on the building sees the information and none of the actions. Run through `make smoke`.
const configured = Boolean(process.env.SMOKE_HOOK_PATH);
const env = (n: string) => process.env[n] ?? "";
const NEW_PIN = "482916";
const USERS = {
  linh: { pin: "SMOKE_RECEPTIONIST_PIN", home: /\/en\/rooms/, map: "/en/rooms" },
  owner: { pin: "SMOKE_OWNER_PIN", home: /\/en\/owner/, map: "/en/owner/rooms" },
  mina: { pin: "SMOKE_MANAGER_PIN", home: /\/en\/owner/, map: "/en/owner/rooms" },
  viv: { pin: "SMOKE_VIEWER_PIN", home: /\/en\/rooms/, map: "/en/rooms" },
} as const;
type User = keyof typeof USERS;

async function typePin(page: Page, nth: number, pin: string) {
  await page.locator("input").nth(nth).click();
  await page.keyboard.type(pin);
}
// Works whichever spec signed the user in first. The chosen PIN goes first: wrong guesses count towards the
// five-try lockout, so the one-time PIN is only tried by the very first sign-in of a user.
async function signIn(page: Page, user: User) {
  const u = USERS[user];
  const attempt = async (pin: string) => {
    await page.goto("/en/sign-in");
    await page.locator('input[name="guesthouseCode"]').fill(env("SMOKE_GUESTHOUSE"));
    await page.locator('input[name="username"]').fill(user);
    await typePin(page, 2, pin);
    await page.getByRole("button", { name: "Sign in" }).click();
  };
  await attempt(NEW_PIN);
  const next = await Promise.race([
    page.waitForURL(u.home).then(() => "home"),
    page
      .getByText("is not right")
      .waitFor()
      .then(() => "first"),
  ]);
  if (next === "first") {
    await attempt(env(u.pin));
    await page.waitForURL(/set-pin/);
    await typePin(page, 0, NEW_PIN);
    await typePin(page, 1, NEW_PIN);
    await page.getByRole("button", { name: "Save PIN and continue" }).click();
    await page.waitForURL(u.home);
  }
}

const ACTIONS = ["Change bed linen", "Cleaned · room vacant", "Set maintenance", "Report damage"];
const hasAll = async (scope: import("@playwright/test").Locator) => {
  for (const text of ACTIONS)
    await expect(scope.getByText(text, { exact: false }).first()).toBeVisible();
};
const hasNone = async (scope: import("@playwright/test").Locator) => {
  for (const text of ACTIONS) await expect(scope.getByText(text, { exact: false })).toHaveCount(0);
};

test.describe.configure({ mode: "serial" });
let cleanUrl = "";

test("a To-clean room: all four actions on the page and in the panel for receptionist, owner and manager, phone and desktop; none for a viewer", async ({
  page,
  browser,
}) => {
  test.skip(!configured, "needs the stack and test guesthouse that `make smoke` creates");
  test.setTimeout(240_000);
  // Make A103 To clean: check in, check out, take the rest in cash.
  await page.setViewportSize({ width: 390, height: 844 });
  await signIn(page, "linh");
  await page.getByRole("link", { name: /A103/ }).first().click();
  await page.locator('input[name="guestName"]').fill("Clean Guest");
  await page.locator('input[name="guestPhone"]').fill("0912345679");
  await page.locator('input[name="deposit"]').fill("10000");
  await page.getByRole("button", { name: "Confirm check-in" }).click();
  await page.getByRole("link", { name: "Check out" }).click();
  await page.getByRole("button", { name: /^Cash/ }).click();
  await expect(page).toHaveURL(/\/en\/paid\?payment=/);
  await page.goto("/en/rooms");
  cleanUrl =
    (await page
      .locator("li", { hasText: /A103/ })
      .first()
      .getByRole("link")
      .getAttribute("href")) ?? "";
  expect(cleanUrl).toContain("/clean?room=");

  // One sign-in per user (the API rate-limits sign-ins); the viewport changes between phone and desktop.
  for (const user of ["linh", "owner", "mina"] as const) {
    const p = await (await browser.newContext({ viewport: { width: 390, height: 900 } })).newPage();
    await signIn(p, user);
    for (const width of [390, 1280]) {
      await p.setViewportSize({ width, height: 900 });
      await p.goto(USERS[user].map);
      const tile = p.locator("li", { hasText: /A103/ }).first();
      await expect(tile).toContainText("To clean");
      if (width >= 1024) {
        await tile.getByRole("button").click();
        await hasAll(p.locator("aside"));
      } else {
        await tile.getByRole("link").click();
        await expect(p).toHaveURL(/\/en\/clean\?room=/);
        await hasAll(p.locator("body"));
      }
    }
    await p.context().close();
  }

  // Without Edit on the building: the information (when the guest left), none of the actions.
  const viv = await (await browser.newContext({ viewport: { width: 390, height: 900 } })).newPage();
  await signIn(viv, "viv");
  await viv.goto(cleanUrl);
  await expect(viv.locator("body")).toContainText("Checked out");
  await hasNone(viv.locator("body"));
  await viv.setViewportSize({ width: 1280, height: 900 });
  await viv.goto("/en/rooms");
  await viv.locator("li", { hasText: /A103/ }).first().getByRole("button").click();
  const panel = viv.locator("aside");
  await expect(panel).toContainText("To clean");
  await expect(panel).toContainText("Checked out");
  await hasNone(panel);
  await viv.context().close();

  // Marking it cleaned from the panel keeps the optimistic update and the toast.
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto("/en/rooms");
  await page.locator("li", { hasText: /A103/ }).first().getByRole("button").click();
  await page
    .locator("aside")
    .getByRole("button", { name: /Cleaned · room vacant/ })
    .click();
  await expect(page.getByText("A103 cleaned")).toBeVisible();
  await expect(page.locator("li", { hasText: /A103/ }).first()).toContainText("Vacant");
});
