import { describe, expect, it } from "vitest";
import {
  normalizeReferralCode,
  referralAlphabet,
  referralCheckChar,
  referralCodeHint,
  referralCodeLength,
  validReferralCode,
} from "./referralCode";

// The same vectors as referrals_test.go and central's referral test.
const vectors: Record<string, string> = { AAAAA: "A", K7M4Q: "H", XY346: "F", "34679": "P", CDEFG: "M" };

function randomCode(rand: () => number): string {
  let body = "";
  for (let i = 0; i < referralCodeLength - 1; i++) body += referralAlphabet[Math.floor(rand() * referralAlphabet.length)];
  return body + referralCheckChar(body);
}

function seeded(seed: number) {
  return () => {
    seed = (seed * 1103515245 + 12345) % 2147483648;
    return seed / 2147483648;
  };
}

describe("referral codes", () => {
  it("agrees with the till and central on the check character", () => {
    for (const [body, want] of Object.entries(vectors)) {
      expect(referralCheckChar(body)).toBe(want);
      expect(validReferralCode(body + want)).toBe(true);
    }
  });

  it("catches every wrong character and every swap of neighbours", () => {
    const rand = seeded(11);
    for (let n = 0; n < 1500; n++) {
      const code = randomCode(rand);
      expect(validReferralCode(code)).toBe(true);
      for (let i = 0; i < code.length; i++) {
        for (const c of referralAlphabet) {
          if (c === code[i]) continue;
          expect(validReferralCode(code.slice(0, i) + c + code.slice(i + 1))).toBe(false);
        }
        if (i + 1 < code.length && code[i] !== code[i + 1]) {
          expect(validReferralCode(code.slice(0, i) + code[i + 1] + code[i] + code.slice(i + 2))).toBe(false);
        }
      }
    }
  });

  it("reads a code however it was typed", () => {
    expect(normalizeReferralCode(" k7m-4q h ")).toBe("K7M4QH");
    expect(validReferralCode(normalizeReferralCode("k7m 4qh"))).toBe(true);
    expect(validReferralCode("K7M4QO")).toBe(false);
    expect(validReferralCode("K7M4Q")).toBe(false);
    expect(validReferralCode("K7M4QHA")).toBe(false);
  });

  it("hints while typing and on a typo", () => {
    expect(referralCodeHint("")).toBe("");
    expect(referralCodeHint("k7m")).toBe("3 more characters");
    expect(referralCodeHint("K7M4QA")).toMatch(/typo/);
    expect(referralCodeHint("K7M4QH")).toBe("");
  });
});
