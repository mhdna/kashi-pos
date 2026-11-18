// Referral codes, as the till checks them (referrals.go has the same rules;
// central makes the codes). Characters that read like another are left out
// of the alphabet, and the last character is a check character, so a typo
// is caught before anything is looked up.

export const referralAlphabet = "ACDEFGHJKMNPQRTWXY34679";
export const referralCodeLength = 6;

/** A code as typed, in capitals, without spaces and dashes. */
export function normalizeReferralCode(code: string): string {
  return code.toUpperCase().replace(/[\s-]/g, "");
}

/** The check character of a code's first characters, or "" if one isn't in the alphabet. */
export function referralCheckChar(body: string): string {
  let sum = 0;
  for (let i = 0; i < body.length; i++) {
    const v = referralAlphabet.indexOf(body.charAt(i));
    if (v < 0) return "";
    sum += (i + 1) * v;
  }
  return referralAlphabet.charAt(sum % referralAlphabet.length);
}

/** Whether a normalized code is a real one: length, alphabet and check character. */
export function validReferralCode(code: string): boolean {
  if (code.length !== referralCodeLength) return false;
  const want = referralCheckChar(code.slice(0, -1));
  return want !== "" && code[code.length - 1] === want;
}

/** What the code field keeps of what was typed: capitals and digits, never
 *  more than a code's length. */
export function referralCodeInput(typed: string): string {
  return typed.toUpperCase().replace(/[^A-Z0-9]/g, "").slice(0, referralCodeLength);
}

/** What the dialog says under a whole code that can't be right, or "". A code
 *  still being typed says nothing. */
export function referralCodeHint(typed: string): string {
  const code = normalizeReferralCode(typed);
  if (code.length < referralCodeLength) return "";
  if (!validReferralCode(code)) return "This isn't a referral code. Check it for a typo";
  return "";
}
