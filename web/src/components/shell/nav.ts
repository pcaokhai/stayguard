import {
  Banknote,
  BarChart3,
  Bell,
  BedDouble,
  Building2,
  Clock,
  Home,
  Landmark,
  List,
  Package,
  Receipt,
  Shield,
  Tag,
  TriangleAlert,
  User,
  UserCog,
  Users,
  type LucideIcon,
} from "lucide-react";
import type { components } from "@/api/generated/schema";
import type { MessageKey } from "@/lib/t";

type Role = components["schemas"]["Role"];

export type NavItem = { label: MessageKey; href: string; icon: LucideIcon };

// Navigation shows only pages that exist (docs/14 W0): add a path here when its task is merged.
const READY = new Set<string>(["/owner", "/rooms", "/housekeeping"]);
export const visible = (items: NavItem[]) => items.filter((i) => READY.has(i.href));

const OWNER_TABS: NavItem[] = [
  { label: "nav.overview", href: "/owner", icon: Home },
  { label: "nav.rooms", href: "/rooms", icon: Building2 },
  { label: "nav.payments", href: "/owner/payments", icon: List },
  { label: "nav.alerts", href: "/owner/alerts", icon: Bell },
];
const DESK_TABS: NavItem[] = [
  { label: "nav.roomMap", href: "/rooms", icon: Building2 },
  { label: "nav.history", href: "/history", icon: Clock },
  { label: "nav.shift", href: "/shift", icon: Banknote },
  { label: "nav.schedule", href: "/schedule", icon: Users },
  { label: "nav.account", href: "/account", icon: User },
];
const HK_TABS: NavItem[] = [
  { label: "nav.clean", href: "/housekeeping", icon: BedDouble },
  { label: "nav.schedule", href: "/schedule", icon: Users },
  { label: "nav.account", href: "/account", icon: User },
];

// Owner menu groups: the More sheet on phones, the sidebar on desktop (boards MenuChu, TongQuanPC).
export const OWNER_GROUPS: { label: MessageKey; items: NavItem[] }[] = [
  {
    label: "nav.monitor",
    items: [
      { label: "nav.roomMap", href: "/rooms", icon: Building2 },
      { label: "nav.alerts", href: "/owner/alerts", icon: Bell },
      { label: "nav.payments", href: "/owner/payments", icon: List },
      { label: "nav.stays", href: "/owner/stays", icon: BedDouble },
      { label: "nav.shiftReview", href: "/owner/shifts", icon: Banknote },
      { label: "nav.activity", href: "/owner/activity", icon: Clock },
    ],
  },
  {
    label: "nav.finance",
    items: [
      { label: "nav.reports", href: "/owner/reports", icon: BarChart3 },
      { label: "nav.expenses", href: "/owner/expenses", icon: Receipt },
    ],
  },
  {
    label: "nav.operations",
    items: [
      { label: "nav.maintenance", href: "/owner/maintenance", icon: TriangleAlert },
      { label: "nav.extras", href: "/owner/extras", icon: Package },
    ],
  },
  {
    label: "nav.people",
    items: [
      { label: "nav.staff", href: "/owner/staff", icon: UserCog },
      { label: "nav.roster", href: "/owner/roster", icon: Users },
      { label: "nav.access", href: "/owner/access", icon: Shield },
    ],
  },
  {
    label: "nav.settings",
    items: [
      { label: "nav.property", href: "/owner/property", icon: Landmark },
      { label: "nav.buildings", href: "/owner/buildings", icon: Building2 },
      { label: "nav.rates", href: "/owner/rates", icon: Tag },
    ],
  },
];

export const isOwnerRole = (r?: Role) => r === "OWNER" || r === "MANAGER";

// Tabs without "More"; the owner's More tab is added by the bar itself.
export function tabsFor(role: Role | undefined): NavItem[] {
  if (isOwnerRole(role)) return visible(OWNER_TABS);
  if (role === "RECEPTIONIST") return visible(DESK_TABS);
  if (role === "HOUSEKEEPING") return visible(HK_TABS);
  return [];
}

// Rail and sidebar list every tab plus the groups that exist.
export function railFor(role: Role | undefined): NavItem[] {
  if (!isOwnerRole(role)) return tabsFor(role);
  const seen = new Set<string>();
  return [...visible(OWNER_TABS), ...OWNER_GROUPS.flatMap((g) => visible(g.items))].filter(
    (i) => !seen.has(i.href) && !!seen.add(i.href),
  );
}
