import { expect, test } from "@playwright/test";

// GD-07: the payment screen polls every 3 s. With no connection it says so (with Retry); coming back resumes the
// same payment and sends no write, so no second payment can be created.
test("offline on the payment screen shows the offline state, back online creates no second payment", async ({
  context,
  page,
}) => {
  const writes: string[] = [];
  page.on("request", (r) => {
    if (r.method() !== "GET" && r.url().includes("/v1/")) writes.push(`${r.method()} ${r.url()}`);
  });
  await page.goto("/en/pay?payment=pay-TRANSFER&room=A101");
  await expect(page.getByText("Transfer note")).toBeVisible();

  await context.setOffline(true);
  await expect(page.getByText("No internet connection")).toBeVisible({ timeout: 10_000 });
  await expect(page.getByRole("button", { name: "Retry" })).toBeVisible();

  await context.setOffline(false);
  // the paused poll resumes by itself; Retry (a plain GET) is there for a person who does not wait
  await expect(page.getByText("Transfer note")).toBeVisible({ timeout: 10_000 });
  await expect(page.getByText("No internet connection")).toBeHidden();
  expect(writes, "no write while offline or after coming back").toEqual([]);
});
