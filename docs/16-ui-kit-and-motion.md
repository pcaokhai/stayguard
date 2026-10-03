# 16 — UI Kit, Motion and Visual Checks

Version 1.0 · 2026-10-02 · Owner: Khai
Applies to every task in `web/`. Design source: `docs/assets/design/` (index in `INDEX.md`) and the canvas boards "DOC · Thư viện UI và chuyển động" and "DOC · Quy tắc responsive". Responsive rules: docs/15 §1.

**Principle: do not hand-build UI primitives.** Generate components with the shadcn/ui CLI, compose them, and theme them to the design. Hand-written Tailwind is for layout (grid, flex, spacing) and small one-off details only.

## 1. Libraries

| Job | Library | How it enters the repo |
| --- | --- | --- |
| Components | shadcn/ui (Radix primitives) | `npx shadcn@latest init`, then `npx shadcn@latest add …` (list below) into `src/components/ui` |
| Motion | Motion (`motion/react`) | `npm i motion` |
| Bottom sheets | vaul | via `shadcn add drawer` |
| Toasts | sonner | via `shadcn add sonner` |
| Tables | TanStack Table | `npm i @tanstack/react-table`, pattern from shadcn "Data Table" |
| Calendar and ranges | react-day-picker | via `shadcn add calendar` |
| Charts | Recharts | via `shadcn add chart` (load only on report pages) |
| Forms | react-hook-form, zod | `npm i react-hook-form zod @hookform/resolvers`, `shadcn add form` |
| PIN entry | input-otp | via `shadcn add input-otp` |
| Rolling numbers | @number-flow/react | `npm i @number-flow/react` |
| Icons | lucide-react | via shadcn |
| Dates | date-fns with `vi` locale | `npm i date-fns` |
| QR | qrcode (already installed) | — |

Components to add in W0: `button card badge input label textarea select checkbox switch radio-group tabs toggle-group dialog drawer sheet popover calendar tooltip dropdown-menu table skeleton sonner form input-otp separator scroll-area progress`. Add others only when a board needs them. All licences are MIT, ISC or Apache-2.0 (`make licenses` must stay green).

Static export: no server actions or route handlers; every component here works client-side.

## 2. Theme

Keep `src/styles/globals.css` tokens as the source and point shadcn variables at them, so components look like the design without editing their files.

| shadcn variable | Value |
| --- | --- |
| `--background` | `var(--color-bg)` #F6F4EF |
| `--foreground` | `var(--color-ink)` #1C1B19 |
| `--card`, `--popover` | #FFFFFF |
| `--primary` | `var(--color-brand)` #1F4E5F, foreground #FFFFFF |
| `--secondary`, `--muted`, `--accent` | `var(--color-sunken)` #ECE9E2, foreground #1C1B19 / muted text #5C5A55 |
| `--border`, `--input` | #E2DED6 / #CFCBC3 |
| `--ring` | #1F4E5F |
| `--destructive` | #8E2320 |
| `--radius` | 14px for cards; controls use `rounded-[10px]` |
| `--chart-1 … 5` | brand #1F4E5F, cost #D9A37A, green #1F5A30, blue #1C4A82, gray #A8A39A |

Status tones (vacant, occupied, overdue, to clean, maintenance, ok, warn, info) stay as the existing `--color-*` tokens and are exposed as Badge variants (`variant="vacant"` etc.) in `src/components/ui/badge.tsx` via cva. Font: Be Vietnam Pro through `next/font/google` (Vietnamese subset), fallback system-ui.

Sizes from the design: touch targets ≥ 44 px (buttons 44–52 px high), body 14–16 px, page titles 20–28 px, card padding 14–18 px.

## 3. Design element → component

| Design element | Component |
| --- | --- |
| Buttons (primary, secondary, ghost, danger, dashed) | `Button` with variants; dashed = `variant="outline"` + `border-dashed` |
| Status pills | `Badge` with status variants |
| Cards, panels | `Card` |
| Building chips, filter chips, segmented controls | `ToggleGroup` (single) inside `ScrollArea` horizontal; sliding background with Motion `layoutId` |
| Tabs (Cần dọn / Đã dọn) | `Tabs` |
| Bottom sheets on phone (extras, confirmations, date picker, gán phiếu) | `Drawer` (vaul) |
| Right drawers on tablet and desktop (add staff, add expense, ticket) | `Sheet side="right"` |
| Centred confirmations and viewers | `Dialog` |
| Choose phone vs desktop container | `useMediaQuery('(min-width: 640px)')` → Drawer or Dialog/Sheet (shadcn "responsive dialog" pattern) |
| Tables | TanStack Table in `Table`; sticky first column with `sticky left-0 bg-card`; on phone render the same rows as `Card` list |
| Date bar and calendar | `Popover` + `Calendar` (single and range modes) |
| PIN boxes | `InputOTP` with 6 slots, numeric pattern |
| Toggles, checkboxes | `Switch`, `Checkbox` |
| Selects | `Select` |
| Forms with errors | `Form` (react-hook-form + zod); error text under the field |
| Toasts | `toast()` from sonner, bottom-center on phone, bottom-right on desktop |
| Skeletons | `Skeleton` shaped like the real content |
| Charts | `ChartContainer` + Recharts `BarChart` |
| Icons | lucide-react at 16–22 px, stroke 2 |
| Phone navigation | `src/components/shell/BottomTabBar` (plain links + Motion `layoutId` pill, 56 px + `env(safe-area-inset-bottom)`); owner More = `Drawer` with grouped tiles (boards DieuHuongMobile, MenuChu) |

## 4. Motion tokens and primitives

Put tokens in `src/lib/motion.ts` and primitives in `src/components/motion/`:

| Token | Value |
| --- | --- |
| `duration.fast` | 0.12 s (press, hover) |
| `duration.base` | 0.2 s (fades, colour changes) |
| `duration.slow` | 0.32 s (sheets, page enter) |
| `ease.standard` | `[0.2, 0, 0, 1]` |
| `spring.snappy` | `{ type: 'spring', stiffness: 500, damping: 32 }` (pills, tabs) |
| `stagger` | 0.03 s, at most the first 10 items |

Primitives: `FadeIn` (opacity + 8 px rise), `StaggerList`, `SlidingPill` (shared `layoutId`), `PressScale` (`whileTap={{ scale: 0.97 }}`), `RollingNumber` (wraps NumberFlow), `SuccessTick` (SVG path length 0 → 1 in 0.4 s), `Pulse` (one-time scale 1 → 1.04 → 1 with ring glow).

Rules:
1. Animate only `transform` and `opacity` (colour cross-fades through CSS transitions on background are fine). No layout-thrashing properties.
2. Wrap the app in `<MotionConfig reducedMotion="user">`; with reduced motion only fades remain (check in the browser with emulated `prefers-reduced-motion`).
3. Never delay input: interactions are usable immediately; animations run alongside.
4. Money amounts on the QR, bill, receipt and checkout screens never animate. Rolling numbers are for counts and dashboard totals only.
5. One celebratory moment only: the Paid screen. Nothing else bounces more than once.

## 5. Micro-interactions by screen

| Where | Interaction |
| --- | --- |
| Every button | `PressScale`; loading shows a spinner inside the button, label stays |
| Room map | Tiles enter with `StaggerList`; status change cross-fades colour and runs `Pulse` once; building chip selection slides; counters roll |
| Check-in / check-out forms | Field error shakes twice (x: 0 → −6 → 6 → 0, 0.2 s); submit success toast; navigate with `FadeIn` |
| QR payment | Polling indicator breathes (opacity 0.6 ↔ 1, 1.6 s); on PAID the QR scales to 0.9 and fades, `SuccessTick` draws, `navigator.vibrate?.(15)` |
| Extras sheet | Quantity steppers press-scale; line total updates without animation (money) |
| Cleaning list | Cleaned card slides out (x: 40, opacity 0) and the list closes the gap with `layout`; counter rolls; toast with Undo for 5 s where the API allows |
| Owner overview | KPI totals roll; building bars grow from 0 to their width on first load (0.32 s, staggered); new alert badge bounces once |
| Tables | Rows `FadeIn` on page change; hover row background transition 0.12 s |
| Drawers, sheets, dialogs | Library defaults (vaul drag to close, Radix fade + zoom 0.96 → 1) |
| Skeletons | Shimmer via shadcn Skeleton; replaced by content with `FadeIn` |
| Date bar | Previous and next day: content slides 12 px in the direction of travel |
| Tabs, segmented controls | Sliding pill with `spring.snappy` |
| PIN entry | Each filled slot pops (scale 0.9 → 1); wrong PIN shakes the group and clears |

## 6. Visual check with agent-browser (required for every UI task)

Install once: `npm i -g agent-browser && agent-browser install`. Run the app (`npm run dev` on mocks, or `make up`).

For each board the task names:

```bash
scripts/ui-shots.sh /vi/rooms Main            # phone 390 px against Main.png
scripts/ui-shots.sh /vi/rooms SoDoPhongTab 834x1194
scripts/ui-shots.sh /vi/rooms SoDoMayTinh 1280x900
scripts/ui-shots.sh /en/rooms MainEN          # English route against MainEN.png
```

The script sets the viewport, opens the route, waits for the network to settle and saves a full-page screenshot to `.shots/` (git-ignored), then prints the design PNG path. Then:

1. Open both images (the screenshot and `docs/assets/design/screens/<Board>.png`) and compare region by region: layout and order, every text string, colours and status tones, which buttons exist, empty and error states.
2. `agent-browser snapshot -i` to confirm every interactive element has an accessible name and is reachable.
3. Interact once through the main action (`agent-browser click @eN`), screenshot again, and confirm the next state and its motion settle correctly.
4. Repeat with reduced motion (`agent-browser set media prefers-reduced-motion reduce` if your version supports it; otherwise emulate in Playwright) and confirm nothing important depends on animation.
5. In the task's final report, list each board as **match** or **differences: …**. Fix differences in data, text, missing states or broken layout; accept small spacing or font-rendering differences (the PNGs were rendered without the web font).

`npx playwright test e2e/layout.spec.ts` must stay green (no sideways page scroll at 390, 834, 1280 for every route).

## 7. Performance and accessibility budget

- First load JS per route ≤ 200 kB gzip; Recharts and the calendar load with `next/dynamic` only on pages that use them.
- Lists over 100 rows paginate on the server (cursor); no client-side virtualisation needed yet.
- Focus visible on every control (shadcn ring); dialogs trap focus; sheets close with Esc and swipe.
- Colour is never the only signal: every status has text.
- Contrast ≥ 4.5:1 for text; status badges already meet it.

## 8. Sessions, tabs and permissions

- The session token lives in `sessionStorage`, which is **per browser tab**: a new tab (or a copied link opened in a new tab) starts signed out. That is intended (a shared front-desk computer forgets the person when the tab closes). It is not a bug to fix with `localStorage`.
- A tab without a valid session gets a 401 from the API; the app then goes to `/sign-in?reason=expired&next=<page>`, shows "Your session has ended. Sign in again." and, after signing in, returns to `next` (same app and language only). Only a 401 does this.
- A signed-in person without the role for a page (for example a receptionist on any `/owner/...` URL, guarded by `OwnerGuard`, or a 403 from the API) sees the "no permission" screen in the same tab with a back button; the session is kept.
- Covered by `e2e/smoke.then-awaiting.spec.ts` (run through `make smoke`).

