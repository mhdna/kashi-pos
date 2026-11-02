// The built-in retail calendar (Valentine's, Black Friday...). A copy of
// kashi's ui/src/data/retailCalendarEvents.js: those events are not synced,
// only the user-defined ones are (calendar_events.go). Keep the two in step.

export interface CalendarEvent {
  key: string;
  name: string;
  start: Date;
  end: Date; // inclusive, end of day
  color: string;
  icon: string;
  builtin: boolean;
}

interface Range {
  start: Date;
  end: Date;
}

interface Builtin {
  name: string;
  color: string;
  icon: string;
  dates: (year: number) => Range[];
}

function nthWeekdayOfMonth(year: number, month: number, weekday: number, n: number): Date {
  const first = new Date(year, month, 1);
  const day = 1 + ((weekday - first.getDay() + 7) % 7) + (n - 1) * 7;
  return new Date(year, month, day);
}

function addDays(date: Date, days: number): Date {
  const d = new Date(date);
  d.setDate(d.getDate() + days);
  return d;
}

// Hijri holidays move with the moon, so they are approximated from a known
// date and labelled "(approx.)": the real day can differ by a day or two.
const HIJRI_YEAR_DAYS = 354.36707;
const EID_AL_FITR_ANCHOR = new Date(2024, 3, 10);
const EID_AL_ADHA_ANCHOR = new Date(2024, 5, 16);

function approximateHijriHoliday(anchor: Date, year: number): Date | null {
  const roughSteps = Math.round(((year - anchor.getFullYear()) * 365.2425) / HIJRI_YEAR_DAYS);
  for (const delta of [0, 1, -1, 2, -2]) {
    const candidate = addDays(anchor, Math.round((roughSteps + delta) * HIJRI_YEAR_DAYS));
    if (candidate.getFullYear() === year) return candidate;
  }
  return null;
}

const day = (y: number, m: number, d: number) => ({ start: new Date(y, m, d), end: new Date(y, m, d) });
const span = (a: Date, b: Date) => ({ start: a, end: b });

const builtins: Builtin[] = [
  { name: "New Year Sale", color: "indigo", icon: "mdi-firework", dates: (y) => [span(new Date(y, 0, 1), new Date(y, 0, 7))] },
  { name: "Valentine’s Day", color: "pink", icon: "mdi-heart", dates: (y) => [day(y, 1, 14)] },
  {
    name: "Mother’s Day",
    color: "deep-purple",
    icon: "mdi-flower-tulip",
    dates: (y) => {
      const d = nthWeekdayOfMonth(y, 4, 0, 2);
      return [span(d, d)];
    },
  },
  { name: "Summer Sale", color: "orange", icon: "mdi-white-balance-sunny", dates: (y) => [span(new Date(y, 5, 1), new Date(y, 5, 30))] },
  { name: "Back to School", color: "teal", icon: "mdi-school", dates: (y) => [span(new Date(y, 7, 15), new Date(y, 8, 5))] },
  { name: "Halloween", color: "deep-orange", icon: "mdi-halloween", dates: (y) => [day(y, 9, 31)] },
  {
    name: "Black Friday",
    color: "blue-grey",
    icon: "mdi-sale",
    dates: (y) => {
      const d = addDays(nthWeekdayOfMonth(y, 10, 4, 4), 1);
      return [span(d, d)];
    },
  },
  {
    name: "Cyber Monday",
    color: "blue-darken-2",
    icon: "mdi-laptop",
    dates: (y) => {
      const d = addDays(nthWeekdayOfMonth(y, 10, 4, 4), 4);
      return [span(d, d)];
    },
  },
  { name: "Christmas Day", color: "green-darken-2", icon: "mdi-pine-tree", dates: (y) => [day(y, 11, 25)] },
  { name: "Winter Clearance Sale", color: "cyan-darken-1", icon: "mdi-snowflake", dates: (y) => [span(new Date(y, 11, 26), new Date(y + 1, 0, 10))] },
  {
    name: "Eid al-Fitr (approx.)",
    color: "amber-darken-2",
    icon: "mdi-moon-waning-crescent",
    dates: (y) => {
      const d = approximateHijriHoliday(EID_AL_FITR_ANCHOR, y);
      return d ? [span(d, addDays(d, 2))] : [];
    },
  },
  {
    name: "Eid al-Adha (approx.)",
    color: "brown-darken-1",
    icon: "mdi-moon-waning-crescent",
    dates: (y) => {
      const d = approximateHijriHoliday(EID_AL_ADHA_ANCHOR, y);
      return d ? [span(d, addDays(d, 3))] : [];
    },
  },
];

export interface SyncedEvent {
  id: number;
  name: string;
  icon: string;
  color: string;
  startDate: string; // YYYY-MM-DD
  endDate: string;
}

const endOfDay = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate(), 23, 59, 59);

function parseDay(s: string): Date | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(s);
  return m ? new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3])) : null;
}

/** Built-in and synced events that overlap [from, to], earliest first. */
export function eventsInRange(from: Date, to: Date, synced: SyncedEvent[]): CalendarEvent[] {
  const out: CalendarEvent[] = [];
  for (let year = from.getFullYear() - 1; year <= to.getFullYear() + 1; year++) {
    for (const b of builtins) {
      for (const r of b.dates(year)) {
        const end = endOfDay(r.end);
        if (end >= from && r.start <= to) {
          out.push({ key: `b:${b.name}:${year}`, name: b.name, start: r.start, end, color: b.color, icon: b.icon, builtin: true });
        }
      }
    }
  }
  for (const e of synced) {
    const start = parseDay(e.startDate);
    const endDay = parseDay(e.endDate);
    if (!start || !endDay) continue;
    const end = endOfDay(endDay);
    if (end >= from && start <= to) {
      out.push({ key: `c:${e.id}`, name: e.name, start, end, color: e.color || "primary", icon: e.icon || "mdi-calendar-star", builtin: false });
    }
  }
  return out.sort((a, b) => a.start.getTime() - b.start.getTime() || a.name.localeCompare(b.name));
}
