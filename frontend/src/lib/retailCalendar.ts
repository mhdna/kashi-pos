// The built-in calendar (Lebanese holidays, clothing seasons...). The events
// themselves are DATA -- date rules -- that kashi sends in the
// "calendar_settings" sync entity and the till keeps in its local database, so
// they work offline and a change in kashi reaches the till on its next pull,
// with no till release. builtinEvents.json is a copy of kashi's
// api/builtin_events.json, used only until the first pull; the rule kinds are
// explained at expandRule below.

import bundledCatalog from "./builtinEvents.json";

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

type Rule =
  | { type: "fixed"; month: number; day: number; days?: number }
  | { type: "span"; from: [number, number]; to: [number, number]; toNextYear?: boolean }
  | {
      type: "nthWeekday";
      month: number;
      weekday: number;
      n: number;
      offset?: number;
      days?: number;
    }
  | { type: "easter"; kind: "western" | "orthodox"; offset?: number; days?: number }
  | { type: "hijri"; anchor: string; offset?: number; days?: number };

export interface CatalogEvent {
  list: string;
  name: string;
  color: string;
  icon: string;
  rule: Rule;
}

export interface Catalog {
  lists: Record<string, { title: string; icon: string }>;
  businessTypes: Record<string, { title: string; icon: string; lists: string[] }>;
  events: CatalogEvent[];
}

export const DEFAULT_BUSINESS_TYPE = "clothing";

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

// Mean length of a tabular Hijri year, used only to approximate where a Hijri
// holiday falls in the Gregorian calendar.
const HIJRI_YEAR_DAYS = 354.367_07;

// A Hijri date comes round every ~354 days, so it can land twice in one
// Gregorian year (early January and late December): return every hit.
function approximateHijriDates(anchor: Date, year: number): Date[] {
  const roughSteps = Math.round(((year - anchor.getFullYear()) * 365.2425) / HIJRI_YEAR_DAYS);
  const found: Date[] = [];
  for (let delta = -3; delta <= 3; delta++) {
    const candidate = addDays(anchor, Math.round((roughSteps + delta) * HIJRI_YEAR_DAYS));
    if (candidate.getFullYear() === year) {
      found.push(candidate);
    }
  }
  return found;
}

// Western (Gregorian) Easter Sunday -- Meeus/Jones/Butcher algorithm.
function westernEaster(year: number): Date {
  const a = year % 19;
  const b = Math.floor(year / 100);
  const c = year % 100;
  const d = Math.floor(b / 4);
  const e = b % 4;
  const f = Math.floor((b + 8) / 25);
  const g = Math.floor((b - f + 1) / 3);
  const h = (19 * a + b - d - g + 15) % 30;
  const i = Math.floor(c / 4);
  const k = c % 4;
  const l = (32 + 2 * e + 2 * i - h - k) % 7;
  const m = Math.floor((a + 11 * h + 22 * l) / 451);
  const month = Math.floor((h + l - 7 * m + 114) / 31);
  const day = ((h + l - 7 * m + 114) % 31) + 1;
  return new Date(year, month - 1, day);
}

// Orthodox Easter Sunday: computed on the Julian calendar, then moved to the
// Gregorian one (+13 days, correct for 1900-2099).
function orthodoxEaster(year: number): Date {
  const a = year % 4;
  const b = year % 7;
  const c = year % 19;
  const d = (19 * c + 15) % 30;
  const e = (2 * a + 4 * b - d + 34) % 7;
  const month = Math.floor((d + e + 114) / 31);
  const day = ((d + e + 114) % 31) + 1;
  return addDays(new Date(year, month - 1, day), 13);
}

// The dates of one rule in one year. Months are 1-12; `days` defaults to 1 and
// `offset` moves the start by that many days. Hijri dates are approximated from
// an anchor (a known Gregorian date of that Hijri day), so they can be a day
// or two off.
export function expandRule(rule: Rule, year: number): Range[] {
  const run = (start: Date, days = 1): Range => ({ start, end: addDays(start, days - 1) });
  switch (rule.type) {
    case "fixed":
      return [run(new Date(year, rule.month - 1, rule.day), rule.days)];
    case "span":
      return [
        {
          start: new Date(year, rule.from[0] - 1, rule.from[1]),
          end: new Date(year + (rule.toNextYear ? 1 : 0), rule.to[0] - 1, rule.to[1]),
        },
      ];
    case "nthWeekday":
      return [
        run(
          addDays(nthWeekdayOfMonth(year, rule.month - 1, rule.weekday, rule.n), rule.offset ?? 0),
          rule.days,
        ),
      ];
    case "easter": {
      const easter = rule.kind === "orthodox" ? orthodoxEaster(year) : westernEaster(year);
      return [run(addDays(easter, rule.offset ?? 0), rule.days)];
    }
    case "hijri": {
      const [y, m, d] = rule.anchor.split("-").map(Number);
      return approximateHijriDates(new Date(y!, m! - 1, d!), year).map((date) =>
        run(addDays(date, rule.offset ?? 0), rule.days),
      );
    }
    default:
      return [];
  }
}

const bundled = bundledCatalog as Catalog;

// The catalog kashi last sent (JSON text from the local database), or the
// bundled one when none has arrived yet or it does not look right.
let parsedFor = "";
let parsed: Catalog = bundled;
function catalogFrom(text: string | undefined): Catalog {
  if (!text) return bundled;
  if (text !== parsedFor) {
    parsedFor = text;
    try {
      const c = JSON.parse(text) as Catalog;
      parsed = Array.isArray(c.events) && c.businessTypes && c.lists ? c : bundled;
    } catch {
      parsed = bundled;
    }
  }
  return parsed;
}

/** The built-in events a business type shows (before any are hidden). */
export function eventsForBusiness(catalog: Catalog, businessType: string): CatalogEvent[] {
  const type = catalog.businessTypes[businessType] ?? catalog.businessTypes[DEFAULT_BUSINESS_TYPE];
  const lists = new Set(type?.lists ?? []);
  return catalog.events.filter((event) => lists.has(event.list));
}

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

export interface CalendarSettings {
  businessType: string;
  hiddenEvents: string[];
  /** kashi's built-in events as JSON text; empty until the first pull. */
  catalog?: string;
}

export const DEFAULT_CALENDAR_SETTINGS: CalendarSettings = {
  businessType: DEFAULT_BUSINESS_TYPE,
  hiddenEvents: [],
};

/** Built-in (for this business type, minus the hidden ones) and synced events that overlap [from, to], earliest first. */
export function eventsInRange(
  from: Date,
  to: Date,
  synced: SyncedEvent[],
  settings: CalendarSettings = DEFAULT_CALENDAR_SETTINGS,
): CalendarEvent[] {
  const hidden = new Set(settings.hiddenEvents);
  const events = eventsForBusiness(catalogFrom(settings.catalog), settings.businessType);
  const out: CalendarEvent[] = [];
  for (let year = from.getFullYear() - 1; year <= to.getFullYear() + 1; year++) {
    for (const b of events) {
      if (hidden.has(b.name)) continue;
      for (const r of expandRule(b.rule, year)) {
        const end = endOfDay(r.end);
        if (end >= from && r.start <= to) {
          out.push({
            key: `b:${b.name}:${year}`,
            name: b.name,
            start: r.start,
            end,
            color: b.color,
            icon: b.icon,
            builtin: true,
          });
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
      out.push({
        key: `c:${e.id}`,
        name: e.name,
        start,
        end,
        color: e.color || "primary",
        icon: e.icon || "mdi-calendar-star",
        builtin: false,
      });
    }
  }
  return out.sort((a, b) => a.start.getTime() - b.start.getTime() || a.name.localeCompare(b.name));
}
