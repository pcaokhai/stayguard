import type { Page } from "@playwright/test";
import { expect, test, uiLogin, type Who } from "./helpers";

// Visual baselines (VS-01 to VS-12, proposed rows): 12 key screens in Vietnamese at 390 and 1280. They run FIRST, on the fresh guesthouse,
// so the data behind them is the same every run (200 vacant rooms, nothing paid). Times, dates and the guesthouse code are masked.
// The images live in docs/rehearsal/baseline/ and are NOT approved until Khai approves them: a missing image is written and the case
// fails ("writing actual"), a present one is compared. Never run with --update-snapshots without Khai's say-so.
const SCREENS: { id: string; user: string; route: string; name: string }[] = [
  { id: "VS-01", user: "", route: "/vi/sign-in", name: "dang-nhap" },
  { id: "VS-02", user: "linh", route: "/vi/rooms", name: "le-tan-so-do-phong" },
  { id: "VS-03", user: "linh", route: "/vi/shift", name: "le-tan-ca" },
  { id: "VS-04", user: "linh", route: "/vi/stays", name: "le-tan-lich-su" },
  { id: "VS-05", user: "linh", route: "/vi/account", name: "tai-khoan" },
  { id: "VS-06", user: "hk", route: "/vi/housekeeping", name: "buong-phong" },
  { id: "VS-07", user: "owner", route: "/vi/owner", name: "chu-tong-quan" },
  { id: "VS-08", user: "owner", route: "/vi/owner/rooms", name: "chu-so-do-phong" },
  { id: "VS-09", user: "owner", route: "/vi/owner/alerts", name: "chu-canh-bao" },
  { id: "VS-10", user: "owner", route: "/vi/owner/transactions", name: "chu-giao-dich" },
  { id: "VS-11", user: "owner", route: "/vi/owner/items", name: "chu-dich-vu-kho" },
  { id: "VS-12", user: "owner", route: "/vi/owner/reports", name: "chu-bao-cao" },
];
const SIZES = [
  { w: 390, h: 844 },
  { w: 1280, h: 900 },
];
const dynamic = (page: Page) => [
  page.getByText(/rh[0-9a-f]{6}/),
  page.getByText(/\d{1,2}:\d{2}/),
  page.getByText(/\b\d{1,2}\/\d{1,2}(\/\d{2,4})?\b/),
  page.getByText(/(Thg|Th|Hôm nay|Today)[^\n]{0,12}\d/),
];

for (const s of SCREENS) {
  test(`${s.id} ${s.name} looks the same as its baseline at 390 and 1280`, async ({
    api,
    browser,
  }) => {
    const who: Who | null = s.user ? await api.as(s.user) : null;
    for (const { w, h } of SIZES) {
      const ctx = await browser.newContext({
        viewport: { width: w, height: h },
        reducedMotion: "reduce",
      });
      const page = await ctx.newPage();
      if (who) await uiLogin(page, who);
      await page.goto(s.route);
      await page.waitForLoadState("load");
      await page.waitForTimeout(1_500);
      await expect
        .soft(page)
        .toHaveScreenshot(`${s.id}-${s.name}-${w}.png`, { mask: dynamic(page), fullPage: false });
      await ctx.close();
    }
  });
}
