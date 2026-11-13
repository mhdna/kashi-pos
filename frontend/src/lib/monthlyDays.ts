// The Balance page's month list: each day is "dd/mm/yyyy". What the list shows
// besides the date and the total: the weekday, whether the day is past, today or
// still to come, and the takings per visitor.

export type DayKind = "past" | "today" | "future";

/** "13/11/2026" as a date, or null when it isn't one. */
export function parseDay(day: string): Date | null {
  const match = /^(\d{1,2})\/(\d{1,2})\/(\d{4})$/.exec(day.trim());
  if (!match) return null;
  const [, d, m, y] = match.map(Number) as [number, number, number, number];
  const date = new Date(y, m - 1, d);
  // 31/02 rolls over into March: that is not a day.
  if (date.getFullYear() !== y || date.getMonth() !== m - 1 || date.getDate() !== d) return null;
  return date;
}

const WEEKDAYS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

/** "Fri" for "13/11/2026"; "" for something that is not a day. */
export function weekdayShort(day: string): string {
  const date = parseDay(day);
  return date ? WEEKDAYS[date.getDay()]! : "";
}

/** Whether `day` is before, on, or after `today` (both "dd/mm/yyyy"). */
export function dayKind(day: string, today: string): DayKind {
  const a = parseDay(day);
  const b = parseDay(today);
  if (!a || !b) return "past";
  const diff = a.getTime() - b.getTime();
  return diff === 0 ? "today" : diff < 0 ? "past" : "future";
}

/** The takings per visitor, in the same units as `total`; null without visitors. */
export function perVisitor(total: number, visitors: number | null | undefined): number | null {
  if (!visitors || visitors <= 0 || !Number.isFinite(total)) return null;
  return Math.round(total / visitors);
}
