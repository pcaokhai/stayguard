import { expect, test } from "@playwright/test";

// Only the receipt prints on an 80 mm roll; every other page prints on the browser's normal paper size.
// Chromium drops "size: 80mm auto" as invalid (both lengths are required), hence 80mm 200mm. Reads the page size from the PDF Chromium makes with the page's own CSS (preferCSSPageSize).
async function pageSizeMm(page: import("@playwright/test").Page): Promise<[number, number]> {
  const pdf = await page.pdf({ preferCSSPageSize: true });
  const m = pdf.toString("latin1").match(/\/MediaBox\s*\[\s*0\s+0\s+([\d.]+)\s+([\d.]+)\s*\]/);
  if (!m) throw new Error("no MediaBox in the PDF");
  const mm = (pt: string) => (Number(pt) * 25.4) / 72;
  return [mm(m[1]), mm(m[2])];
}

test("the receipt prints at 80 mm wide, other pages do not", async ({ page }) => {
  await page.goto("/en/receipt?invoice=inv-1");
  await expect(page.getByRole("article")).toBeVisible();
  const [receiptW] = await pageSizeMm(page);
  expect(receiptW).toBeGreaterThan(78);
  expect(receiptW).toBeLessThan(82);

  for (const route of ["/en/rooms", "/en/stays", "/en/shift"]) {
    await page.goto(route);
    await page.waitForLoadState("networkidle");
    const [w] = await pageSizeMm(page);
    expect(w, `${route} stays on normal paper`).toBeGreaterThan(190); // A4 is 210 mm, Letter 216 mm
  }
});
