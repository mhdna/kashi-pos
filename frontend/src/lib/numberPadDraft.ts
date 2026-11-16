// The number pad's typed text ("draft") and how keys change it. A pad that
// allows negatives keeps a leading "-" on the draft; the digits after it
// follow the same rules as a positive number.

/** The draft without its sign. */
export function unsigned(draft: string): string {
  return draft.startsWith("-") ? draft.slice(1) : draft;
}

/**
 * The ± key: flips the sign of what is shown, like a calculator, so typing
 * 25000 then ± gives -25000. "0" or nothing becomes "-", ready for digits.
 */
export function toggleSign(draft: string): string {
  if (draft.startsWith("-")) return draft.slice(1);
  if (draft === "" || draft === "0") return "-";
  return `-${draft}`;
}

/**
 * The draft after typing a digit, or null when the key is refused (too long,
 * too many decimals, or over max in size). `replace` is true right after the
 * pad opens, when the first key starts a new (positive) number; after the
 * ± key the digits add on and the sign stays.
 */
export function appendDigitToDraft(
  draft: string,
  digit: string,
  opts: { replace: boolean; maxLength: number; max: number; allowDecimal: boolean; decimalPlaces: number; preserveString?: boolean },
): string | null {
  const negative = draft.startsWith("-");
  const body = unsigned(draft);
  if (!opts.replace && body.length >= opts.maxLength) return null;
  if (opts.allowDecimal && body.includes(".")) {
    const fraction = body.split(".")[1] ?? "";
    if (!opts.replace && fraction.length >= opts.decimalPlaces) return null;
  }
  const nextBody = opts.replace || body === "0" || body === "" ? digit : `${body}${digit}`;
  if (!opts.preserveString && Math.abs(Number(nextBody)) > opts.max) return null;
  return negative && !opts.replace ? `-${nextBody}` : nextBody;
}

/** The draft after the "." key, or null when there already is one. */
export function appendDecimalToDraft(draft: string, replace: boolean): string | null {
  const negative = draft.startsWith("-");
  const body = unsigned(draft);
  if (!replace && body.includes(".")) return null;
  const nextBody = replace || !body ? "0." : `${body}.`;
  return negative && !replace ? `-${nextBody}` : nextBody;
}

/** The number a draft stands for, clamped to [min, max]. "-" alone is 0. */
export function draftValue(
  draft: string,
  opts: { min: number; max: number; allowDecimal: boolean; decimalPlaces: number },
): number {
  const raw = Number(draft) || 0;
  const factor = 10 ** opts.decimalPlaces;
  const numeric = opts.allowDecimal ? Math.round(raw * factor) / factor : Math.trunc(raw);
  const value = Math.min(opts.max, Math.max(opts.min, numeric));
  return value === 0 ? 0 : value; // never -0
}
