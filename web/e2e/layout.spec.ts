import { expect, test } from "@playwright/test";

// Every route at phone, tablet and desktop width: no page-level sideways scroll (docs/16 §6).
const ROUTES = [
  "",
  "/rooms",
  "/owner/rooms",
  "/stays",
  "/shift",
  "/shift/payout",
  "/rooms?room=A101",
  "/checkin?room=A102",
  "/stay?id=stay-1",
  "/stay/edit-time?id=stay-1",
  "/stay/move?id=stay-1",
  "/receipt?invoice=inv-1",
  "/pay?payment=pay-EXPIRED&room=A101",
  "/pay?payment=pay-MISMATCH&room=A101",
  "/checkout?stay=stay-1",
  "/pay?payment=pay-TRANSFER",
  "/paid?payment=pay-CASH",
  "/housekeeping",
  "/owner",
  "/owner/alerts",
  "/owner/activity",
  "/owner/transactions",
  "/owner/stays",
  "/owner/stay?id=s2",
  "/owner/shifts",
  "/owner/shift?id=sh1",
  "/sign-in",
  "/set-pin",
  "/account",
  "/states?kind=forbidden",
];
const WIDTHS = [390, 834, 1280];

for (const locale of ["vi", "en"]) {
  for (const width of WIDTHS) {
    for (const route of ROUTES) {
      test(`no sideways scroll: /${locale}${route} at ${width}`, async ({ page }) => {
        await page.setViewportSize({ width, height: 900 });
        await page.goto(`/${locale}${route}`);
        await page.waitForLoadState("networkidle");
        const overflow = await page.evaluate(
          () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
        );
        expect(overflow).toBeLessThanOrEqual(0);
      });
    }
  }
}
