// What the till says about its license. The lock message is deliberately
// vague and the same for every reason (turned off, offline too long, clock
// set back): change it here, in one place.
export const LOCK_MESSAGE = "Error starting up: can't access central.";

export type LicenseState = "ok" | "activate" | "locked";

export interface LicenseStatusLike {
  state?: string;
  code?: string;
  warnHours?: number;
}

// Which screen App.vue shows. Anything unknown counts as running, so an
// older backend without licensing never shows a lock.
export function licenseScreen(status: LicenseStatusLike | undefined | null): LicenseState {
  if (status?.state === "activate" || status?.state === "locked") return status.state;
  return "ok";
}

// ActivateLicense rejects with "license:<code>". Only a mistyped key, a key
// already used up and a network problem get their own words; everything else
// is the lock message.
export function activationErrorText(error: unknown): string {
  const raw = error instanceof Error ? error.message : String(error ?? "");
  const code = raw.startsWith("license:") ? raw.slice("license:".length) : "";
  switch (code) {
    case "invalid_key":
      return "That product key isn't right. Check it and try again.";
    case "no_seats":
      return "This product key is already used on all the computers it allows.";
    case "offline":
      return "Can't reach the internet. Connect this computer and try again.";
    case "too_many_attempts":
      return "Too many tries. Wait a minute and try again.";
    default:
      return LOCK_MESSAGE;
  }
}

const KEY_ALPHABET = /[^0-9A-Z]/g;

// formatProductKey shows what was typed as KASHI-XXXXX-XXXXX-XXXXX-XXXXX while
// typing: upper case, dashes added, other characters dropped. The server is
// lenient too (O for 0 and so on); this only makes the key easy to read back.
export function formatProductKey(input: string): string {
  let body = input.toUpperCase().replace(KEY_ALPHABET, "");
  if (body.startsWith("KASHI")) body = body.slice(5);
  body = body.slice(0, 20);
  const groups = body.match(/.{1,5}/g) ?? [];
  return groups.length ? ["KASHI", ...groups].join("-") : "";
}

export function productKeyComplete(input: string): boolean {
  return formatProductKey(input).length === 29;
}

// The warning shown while the till hasn't reached central for a while.
export function offlineWarningText(warnHours: number | undefined): string {
  const h = Math.max(0, Math.floor(warnHours ?? 0));
  if (h <= 0) return "";
  const within = h < 24 ? "today" : h < 48 ? "within a day" : `within ${Math.floor(h / 24)} days`;
  return `Connect this till to the internet ${within}, or it won't start.`;
}
