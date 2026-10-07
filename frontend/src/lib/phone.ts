// Lebanese 8-digit mobile ranges (Alfa/Touch). libphonenumber's metadata lags
// behind -- it rejects most of 78/79 -- so Lebanese numbers are checked here
// first. Mirrors normalizeLebanesePhone in clients.go; keep the two in step.
const LEBANESE_MOBILE_PREFIXES = ["70", "71", "76", "78", "79", "81"];

/**
 * Returns a Lebanese number in E.164 (+961...) when it is a mobile (03 or the
 * ranges above) or a landline (01, 04-09), typed with or without the leading
 * 0, +961 or 00961. Returns "" for anything else.
 */
export function normalizeLebanesePhone(value: string): string {
  const raw = value.trim();
  if (raw.startsWith("+") && !raw.startsWith("+961")) return "";
  if (/[^\d+\s\-()]/.test(raw)) return "";
  let digits = raw.replace(/\D/g, "");
  if (digits.startsWith("00961")) digits = digits.slice(5);
  else if (digits.startsWith("961") && digits.length >= 10) digits = digits.slice(3);
  if (digits.startsWith("0")) digits = digits.slice(1);
  if (digits.length === 8 && LEBANESE_MOBILE_PREFIXES.some((p) => digits.startsWith(p))) return `+961${digits}`;
  // 3xxxxxx is the old 03 mobile range; 1 and 4-9 are regional landlines.
  if (digits.length === 7 && "13456789".includes(digits.charAt(0))) return `+961${digits}`;
  return "";
}

export function formatLebanesePhone(value: unknown): string {
  const raw = String(value ?? "").trim();
  if (!raw) return "";

  let digits = raw.replace(/\D/g, "");
  if (digits.startsWith("961")) {
    const nationalNumber = digits.slice(3);
    digits = nationalNumber.length === 7 ? `0${nationalNumber}` : nationalNumber;
  }

  if (digits.length <= 2) return `(${digits})`;

  const prefix = digits.slice(0, 2);
  const middle = digits.slice(2, 5);
  const suffix = digits.slice(5);
  return `(${prefix}) ${middle}${suffix ? `-${suffix}` : ""}`;
}
