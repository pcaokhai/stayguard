import { fromIso } from "../history/dates";
import { clockLocale } from "../../lib/time";
import { t, tf, type MessageKey } from "../../lib/t";
import type { LeaveRequest, ShiftCode } from "./hooks";

const weekday = (iso: string, style: "short" | "long") =>
  new Intl.DateTimeFormat(clockLocale(), { weekday: style }).format(fromIso(iso));
const dayMonth = (iso: string) => {
  const [, m, d] = iso.split("-");
  return `${d}/${m}`;
};
export { dayMonth };
export const shortWeekday = (iso: string) => weekday(iso, "short");
export const longWeekday = (iso: string) => weekday(iso, "long");
export const shiftName = (s: ShiftCode) => t(`schedule.${s}` as MessageKey);

// "Fri, 03/10 · afternoon shift" / "Thứ 6, 03/10 · cả ngày" / "03/10 – 05/10 · whole day"
export function leaveTitle(r: LeaveRequest): string {
  const when =
    r.fromDate === r.toDate
      ? `${longWeekday(r.fromDate)}, ${dayMonth(r.fromDate)}`
      : `${dayMonth(r.fromDate)} – ${dayMonth(r.toDate)}`;
  const part = r.shift
    ? tf("leave.shiftNamed", { shift: shiftName(r.shift).toLowerCase() })
    : t("leave.wholeDay");
  return `${when} · ${part}`;
}
