// The built-in retail calendar (Lebanese holidays, clothing seasons...). A
// copy of kashi's ui/src/data/retailCalendarEvents.js: those events are not
// synced, only the user-defined ones (calendar_events.go) and the business
// type + hidden list (calendar_settings) are. Keep the two in step.

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
  list: string;
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

// Mean length of a tabular Hijri year, used only to approximate where a Hijri
// holiday falls in the Gregorian calendar. The real date depends on moon
// sighting and can differ by a day or two - events below are labelled
// "(approx.)" for that reason.
const HIJRI_YEAR_DAYS = 354.367_07;
const EID_AL_FITR_ANCHOR = new Date(2024, 3, 10); // 1 Shawwal 1445 AH
const EID_AL_ADHA_ANCHOR = new Date(2024, 5, 16); // 10 Dhu al-Hijjah 1445 AH
const RAMADAN_ANCHOR = new Date(2024, 2, 11); // 1 Ramadan 1445 AH
const ISLAMIC_NEW_YEAR_ANCHOR = new Date(2024, 6, 7); // 1 Muharram 1446 AH
const ASHURA_ANCHOR = new Date(2024, 6, 16); // 10 Muharram 1446 AH
const MAWLID_ANCHOR = new Date(2024, 8, 15); // 12 Rabi' al-awwal 1446 AH

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

// An event that falls on a Hijri date, lasting `days` days from `offset`
// days before/after it (offset 0 = starting on the day itself).
function hijriSpan(anchor: Date, { days = 1, offset = 0 } = {}) {
  return (year: number): Range[] =>
    approximateHijriDates(anchor, year).map((d) => {
      const start = addDays(d, offset);
      return { start, end: addDays(start, days - 1) };
    });
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

function oneDay(year: number, month: number, date: number): Range[] {
  return [{ start: new Date(year, month, date), end: new Date(year, month, date) }];
}
function span(
  year: number,
  m1: number,
  d1: number,
  m2: number,
  d2: number,
  endYearOffset = 0,
): Range[] {
  return [{ start: new Date(year, m1, d1), end: new Date(year + endYearOffset, m2, d2) }];
}
function single(d: Date): Range[] {
  return [{ start: d, end: d }];
}

// Every built-in event belongs to one list. A business type (below) is a set
// of lists, so a clothing shop never sees Cyber Monday and a tech shop never
// sees Swimwear Season. Lebanon's list is part of every business type and, as
// events are shown unless hidden, is on by default.
export const EVENT_LISTS: Record<string, { title: string; icon: string }> = {
  lebanon: { title: "Lebanon", icon: "mdi-flag" },
  retail: { title: "Shopping seasons", icon: "mdi-sale" },
  clothing: { title: "Clothing", icon: "mdi-hanger" },
  tech: { title: "Tech", icon: "mdi-laptop" },
  party: { title: "Parties & fun", icon: "mdi-party-popper" },
};

// The key is what api/app_settings.go (validBusinessTypes) accepts.
export const BUSINESS_TYPES: Record<string, { title: string; icon: string; lists: string[] }> = {
  clothing: { title: "Clothing", icon: "mdi-hanger", lists: ["lebanon", "retail", "clothing"] },
  tech: { title: "Tech", icon: "mdi-laptop", lists: ["lebanon", "retail", "tech"] },
  general: { title: "General shop", icon: "mdi-store", lists: ["lebanon", "retail", "party"] },
};
export const DEFAULT_BUSINESS_TYPE = "clothing";

export const retailCalendarEvents: Builtin[] = [
  // --- Lebanon: public holidays and the moments Lebanese shoppers dress up for.
  {
    list: "lebanon",
    name: "Armenian Christmas",
    color: "red-darken-2",
    icon: "mdi-church",
    dates: (year) => oneDay(year, 0, 6),
  },
  {
    list: "lebanon",
    name: "St. Maroun’s Day",
    color: "red-darken-3",
    icon: "mdi-church",
    dates: (year) => oneDay(year, 1, 9),
  },
  {
    list: "lebanon",
    name: "Mother’s Day",
    color: "deep-purple",
    icon: "mdi-flower-tulip",
    dates: (year) => oneDay(year, 2, 21), // Lebanon and the Arab world: 21 March
  },
  {
    list: "lebanon",
    name: "Annunciation Day",
    color: "indigo-lighten-1",
    icon: "mdi-church",
    dates: (year) => oneDay(year, 2, 25),
  },
  {
    list: "lebanon",
    name: "Good Friday (Western)",
    color: "blue-grey-darken-1",
    icon: "mdi-cross",
    dates: (year) => single(addDays(westernEaster(year), -2)),
  },
  {
    list: "lebanon",
    name: "Easter (Western)",
    color: "purple-lighten-1",
    icon: "mdi-flower-tulip",
    dates: (year) => single(westernEaster(year)),
  },
  {
    list: "lebanon",
    name: "Good Friday (Orthodox)",
    color: "blue-grey-darken-2",
    icon: "mdi-cross",
    dates: (year) => single(addDays(orthodoxEaster(year), -2)),
  },
  {
    list: "lebanon",
    name: "Easter (Orthodox)",
    color: "purple-darken-1",
    icon: "mdi-flower-tulip",
    dates: (year) => single(orthodoxEaster(year)),
  },
  {
    list: "lebanon",
    name: "Labour Day",
    color: "red",
    icon: "mdi-hammer-wrench",
    dates: (year) => oneDay(year, 4, 1),
  },
  {
    list: "lebanon",
    name: "Resistance & Liberation Day",
    color: "green-darken-3",
    icon: "mdi-flag",
    dates: (year) => oneDay(year, 4, 25),
  },
  {
    list: "lebanon",
    name: "Army Day",
    color: "green-darken-4",
    icon: "mdi-flag",
    dates: (year) => oneDay(year, 7, 1),
  },
  {
    list: "lebanon",
    name: "Assumption Day",
    color: "light-blue-darken-1",
    icon: "mdi-church",
    dates: (year) => oneDay(year, 7, 15),
  },
  {
    list: "lebanon",
    name: "All Saints’ Day",
    color: "grey-darken-1",
    icon: "mdi-church",
    dates: (year) => oneDay(year, 10, 1),
  },
  {
    list: "lebanon",
    name: "Independence Day",
    color: "red-darken-1",
    icon: "mdi-flag",
    dates: (year) => oneDay(year, 10, 22),
  },
  {
    list: "lebanon",
    name: "Christmas Day",
    color: "green-darken-2",
    icon: "mdi-pine-tree",
    dates: (year) => oneDay(year, 11, 25),
  },
  {
    list: "lebanon",
    name: "Ramadan (approx.)",
    color: "teal-darken-2",
    icon: "mdi-star-crescent",
    dates: hijriSpan(RAMADAN_ANCHOR, { days: 30 }),
  },
  {
    list: "lebanon",
    name: "Eid al-Fitr (approx.)",
    color: "amber-darken-2",
    icon: "mdi-moon-waning-crescent",
    dates: hijriSpan(EID_AL_FITR_ANCHOR, { days: 3 }),
  },
  {
    list: "lebanon",
    name: "Eid al-Adha (approx.)",
    color: "brown-darken-1",
    icon: "mdi-moon-waning-crescent",
    dates: hijriSpan(EID_AL_ADHA_ANCHOR, { days: 4 }),
  },
  {
    list: "lebanon",
    name: "Islamic New Year (approx.)",
    color: "teal-darken-3",
    icon: "mdi-mosque",
    dates: hijriSpan(ISLAMIC_NEW_YEAR_ANCHOR),
  },
  {
    list: "lebanon",
    name: "Ashura (approx.)",
    color: "grey-darken-3",
    icon: "mdi-mosque",
    dates: hijriSpan(ASHURA_ANCHOR),
  },
  {
    list: "lebanon",
    name: "Prophet’s Birthday (approx.)",
    color: "green",
    icon: "mdi-mosque",
    dates: hijriSpan(MAWLID_ANCHOR),
  },

  // --- Shopping seasons every shop has.
  {
    list: "retail",
    name: "New Year Sale",
    color: "indigo",
    icon: "mdi-firework",
    dates: (year) => span(year, 0, 1, 0, 7),
  },
  {
    list: "retail",
    name: "Valentine’s Day",
    color: "pink",
    icon: "mdi-heart",
    dates: (year) => oneDay(year, 1, 14),
  },
  {
    list: "retail",
    name: "Summer Sale",
    color: "orange",
    icon: "mdi-white-balance-sunny",
    dates: (year) => span(year, 5, 1, 5, 30),
  },
  {
    list: "retail",
    name: "Back to School",
    color: "teal",
    icon: "mdi-school",
    dates: (year) => span(year, 7, 15, 8, 5),
  },
  {
    list: "retail",
    name: "Black Friday",
    color: "blue-grey",
    icon: "mdi-sale",
    dates: (year) => single(addDays(nthWeekdayOfMonth(year, 10, 4, 4), 1)), // day after US Thanksgiving
  },
  {
    list: "retail",
    name: "Christmas Gift Season",
    color: "green-lighten-1",
    icon: "mdi-gift-outline",
    dates: (year) => span(year, 11, 1, 11, 24),
  },

  // --- Clothing.
  {
    list: "clothing",
    name: "Women’s Day",
    color: "pink-darken-1",
    icon: "mdi-human-female",
    dates: (year) => oneDay(year, 2, 8),
  },
  {
    list: "clothing",
    name: "Spring Collection Arrival",
    color: "light-green",
    icon: "mdi-leaf",
    dates: (year) => span(year, 1, 15, 2, 15),
  },
  {
    list: "clothing",
    name: "Eid al-Fitr Shopping (approx.)",
    color: "amber",
    icon: "mdi-tag-heart",
    // The two weeks before Eid, when families buy new clothes.
    dates: hijriSpan(EID_AL_FITR_ANCHOR, { days: 14, offset: -14 }),
  },
  {
    list: "clothing",
    name: "Easter Shopping",
    color: "purple-lighten-2",
    icon: "mdi-tag-heart",
    dates: (year) => [
      { start: addDays(westernEaster(year), -14), end: addDays(westernEaster(year), -1) },
    ],
  },
  {
    list: "clothing",
    name: "Summer Collection Arrival",
    color: "yellow-darken-2",
    icon: "mdi-tshirt-crew",
    dates: (year) => span(year, 3, 15, 4, 15),
  },
  {
    list: "clothing",
    name: "Swimwear Season",
    color: "cyan",
    icon: "mdi-beach",
    dates: (year) => span(year, 4, 1, 5, 15),
  },
  {
    list: "clothing",
    name: "Graduation Season",
    color: "deep-purple-lighten-1",
    icon: "mdi-school",
    dates: (year) => span(year, 4, 15, 5, 30),
  },
  {
    list: "clothing",
    name: "Father’s Day",
    color: "blue-darken-1",
    icon: "mdi-account-tie",
    dates: (year) => single(nthWeekdayOfMonth(year, 5, 0, 3)), // 3rd Sunday of June
  },
  {
    list: "clothing",
    name: "Wedding Season",
    color: "pink-lighten-2",
    icon: "mdi-ring",
    dates: (year) => span(year, 5, 15, 8, 15),
  },
  {
    list: "clothing",
    name: "Eid al-Adha Shopping (approx.)",
    color: "brown-lighten-1",
    icon: "mdi-tag-heart",
    dates: hijriSpan(EID_AL_ADHA_ANCHOR, { days: 7, offset: -7 }),
  },
  {
    list: "clothing",
    name: "Summer Clearance",
    color: "deep-orange-lighten-1",
    icon: "mdi-sale",
    dates: (year) => span(year, 6, 15, 7, 31),
  },
  {
    list: "clothing",
    name: "Fall Collection Arrival",
    color: "orange-darken-3",
    icon: "mdi-leaf",
    dates: (year) => span(year, 8, 1, 9, 15),
  },
  {
    list: "clothing",
    name: "Coats & Jackets Season",
    color: "blue-grey-darken-2",
    icon: "mdi-snowflake",
    dates: (year) => span(year, 9, 15, 10, 30),
  },
  {
    list: "clothing",
    name: "Holiday Party Season",
    color: "red-darken-2",
    icon: "mdi-party-popper",
    dates: (year) => span(year, 11, 10, 11, 31),
  },
  {
    list: "clothing",
    name: "Winter Clearance Sale",
    color: "cyan-darken-1",
    icon: "mdi-snowflake",
    dates: (year) => span(year, 11, 26, 0, 10, 1),
  },

  // --- Tech.
  {
    list: "tech",
    name: "Prime Day (approx.)",
    color: "blue-darken-3",
    icon: "mdi-sale",
    dates: (year) => span(year, 6, 15, 6, 16),
  },
  {
    list: "tech",
    name: "New iPhone Season",
    color: "grey-darken-2",
    icon: "mdi-cellphone",
    dates: (year) => span(year, 8, 10, 8, 30),
  },
  {
    list: "tech",
    name: "Singles’ Day (11.11)",
    color: "red-darken-1",
    icon: "mdi-sale",
    dates: (year) => oneDay(year, 10, 11),
  },
  {
    list: "tech",
    name: "Cyber Monday",
    color: "blue-darken-2",
    icon: "mdi-laptop",
    dates: (year) => single(addDays(nthWeekdayOfMonth(year, 10, 4, 4), 4)),
  },

  // --- Parties & fun (general shops).
  {
    list: "party",
    name: "Mother’s Day (May)",
    color: "deep-purple-lighten-2",
    icon: "mdi-flower-tulip",
    dates: (year) => single(nthWeekdayOfMonth(year, 4, 0, 2)), // 2nd Sunday of May (US/Europe)
  },
  {
    list: "party",
    name: "Halloween",
    color: "deep-orange",
    icon: "mdi-halloween",
    dates: (year) => oneDay(year, 9, 31),
  },
  {
    list: "party",
    name: "New Year’s Eve",
    color: "indigo-darken-2",
    icon: "mdi-glass-cocktail",
    dates: (year) => oneDay(year, 11, 31),
  },
];

// The built-in events a business type shows (before the user hides any).
export function eventsForBusiness(businessType: string): Builtin[] {
  const lists = new Set(
    (BUSINESS_TYPES[businessType] ?? BUSINESS_TYPES[DEFAULT_BUSINESS_TYPE])!.lists,
  );
  return retailCalendarEvents.filter((event) => lists.has(event.list));
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

/** Built-in (for this business type, minus the hidden ones) and synced events that overlap [from, to], earliest first. */
export interface CalendarSettings {
  businessType: string;
  hiddenEvents: string[];
}

export const DEFAULT_CALENDAR_SETTINGS: CalendarSettings = {
  businessType: DEFAULT_BUSINESS_TYPE,
  hiddenEvents: [],
};

export function eventsInRange(
  from: Date,
  to: Date,
  synced: SyncedEvent[],
  settings: CalendarSettings = DEFAULT_CALENDAR_SETTINGS,
): CalendarEvent[] {
  const hidden = new Set(settings.hiddenEvents);
  const out: CalendarEvent[] = [];
  for (let year = from.getFullYear() - 1; year <= to.getFullYear() + 1; year++) {
    for (const b of eventsForBusiness(settings.businessType)) {
      if (hidden.has(b.name)) continue;
      for (const r of b.dates(year)) {
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
