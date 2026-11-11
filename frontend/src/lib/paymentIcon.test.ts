import { describe, expect, it } from "vitest";
import { paymentIcon } from "./paymentIcon";

const icon = (name: string, extra = {}) => paymentIcon({ name, ...extra });

describe("paymentIcon", () => {
  it("the accounts the till ships with", () => {
    expect(icon("Cash $$")).toBe("mdi-cash");
    expect(icon("Cash USD")).toBe("mdi-cash");
    expect(icon("Cash")).toBe("mdi-cash");
    expect(icon("Cash LBP")).toBe("mdi-cash-100");
    expect(icon("Cash LL")).toBe("mdi-cash-100");
    expect(icon("Lebanese pounds")).toBe("mdi-cash-100");
    expect(icon("Visa")).toBe("mdi-credit-card-outline");
    expect(icon("Visa 2")).toBe("mdi-credit-card-outline");
    expect(icon("Mastercard")).toBe("mdi-credit-card-outline");
    expect(icon("Whish")).toBe("mdi-cellphone");
    expect(icon("OMT")).toBe("mdi-cellphone");
    expect(icon("Bank transfer")).toBe("mdi-bank");
  });

  it("any case", () => {
    expect(icon("VISA")).toBe("mdi-credit-card-outline");
    expect(icon("cash lbp")).toBe("mdi-cash-100");
  });

  it("on account and coupons win over the name", () => {
    expect(icon("Cash $$", { isOnAccount: true })).toBe("mdi-account-clock");
    expect(icon("Visa", { isOnCoupon: true })).toBe("mdi-ticket-percent-outline");
    expect(icon("Store coupon")).toBe("mdi-ticket-percent-outline");
  });

  it("anything unknown, empty or odd still gets an icon", () => {
    expect(icon("Something new")).toBe("mdi-wallet");
    expect(icon("")).toBe("mdi-wallet");
    expect(paymentIcon({})).toBe("mdi-wallet");
    expect(paymentIcon({ name: null })).toBe("mdi-wallet");
    expect(paymentIcon({ name: 42 })).toBe("mdi-wallet");
  });

  it("every icon it can return is registered with the app", async () => {
    const { readFileSync } = await import("node:fs");
    const { resolve } = await import("node:path");
    const registry = readFileSync(resolve(process.cwd(), "src/lib/icons.ts"), "utf8");
    const names = ["Cash $$", "Cash LBP", "Visa", "Whish", "Bank", "coupon", "x", ""].map(icon);
    for (const n of [...names, icon("a", { isOnAccount: true })]) {
      expect(registry, n).toContain(`"${n}":`);
    }
  });
});
