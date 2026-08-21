// The backend stores every created_at/closed_at/reviewed_at timestamp as a
// UTC "YYYY-MM-DD HH:MM:SS" string with no timezone suffix, so every place
// that displays one of these values must go through here to convert it to
// the viewer's local time. Do not display a server timestamp string
// directly - use one of these instead.

function parseServerDate(value?: string | null): Date | null {
  if (!value) return null;
  const normalized = value.includes("T") ? value : value.replace(" ", "T");
  const hasTimezone = /(?:Z|[+-]\d{2}:\d{2})$/i.test(normalized);
  const date = new Date(hasTimezone ? normalized : `${normalized}Z`);
  return Number.isNaN(date.getTime()) ? null : date;
}

function pad(part: number): string {
  return String(part).padStart(2, "0");
}

export function formatServerDateTime(value?: string | null): string {
  const date = parseServerDate(value);
  if (!date) return value ? value.slice(0, 19) : "";
  return [
    `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`,
    `${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`,
  ].join(" ");
}

export function formatServerDateOnly(value?: string | null): string {
  const date = parseServerDate(value);
  if (!date) return value ? value.slice(0, 10) : "";
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

// A key that groups by local calendar day - use for keying/bucketing shifts
// or other rows by "the day this happened" in the viewer's timezone.
export function serverDateLocalDayKey(value?: string | null): string {
  const date = parseServerDate(value);
  if (!date) return "—";
  return `${pad(date.getDate())}/${pad(date.getMonth() + 1)}/${date.getFullYear()}`;
}
