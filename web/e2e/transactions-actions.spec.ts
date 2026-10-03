import { expect, test } from "@playwright/test";

// QA UI-01 (bugs.md row 11): the owner's two actions on an unmatched transfer must not widen the page at 390 px.
// The layout spec runs without a session, so it never sees the owner-only buttons; this one signs in as the owner.
for (const locale of ["vi", "en"]) {
  for (const width of [390, 834, 1280]) {
    test(`owner actions on an unmatched transfer fit: /${locale} at ${width}`, async ({ page }) => {
      await page.addInitScript(() =>
        sessionStorage.setItem(
          "stayguard.session",
          JSON.stringify({
            tenantId: "t1",
            user: { id: "u1", name: "Chủ nhà", role: "OWNER", locale: "vi" },
          }),
        ),
      );
      await page.setViewportSize({ width, height: 900 });
      await page.goto(`/${locale}/owner/transactions`);
      await page.waitForLoadState("networkidle");
      const buttons = page.getByRole("button", {
        name: /Không phải tiền phòng|Not a room payment/,
      });
      await expect(buttons.filter({ visible: true }).first()).toBeVisible();
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
        ),
      ).toBeLessThanOrEqual(0);
      for (const b of await buttons.filter({ visible: true }).all()) {
        const box = (await b.boundingBox())!;
        expect(box.x).toBeGreaterThanOrEqual(0);
        expect(box.x + box.width).toBeLessThanOrEqual(width);
        expect(box.height).toBeGreaterThanOrEqual(44);
      }
      // A long note without spaces must not widen the dialog or the page either.
      await buttons.filter({ visible: true }).first().click();
      const dialog = page.getByRole("dialog");
      await dialog.getByRole("textbox").fill("a".repeat(501));
      await page.waitForTimeout(600);
      const d = (await dialog.boundingBox())!;
      expect(d.x).toBeGreaterThanOrEqual(0);
      expect(d.x + d.width).toBeLessThanOrEqual(width);
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
        ),
      ).toBeLessThanOrEqual(0);
    });
  }
}
