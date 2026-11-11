import { describe, expect, it } from "vitest";
import { paymentIcon } from "./paymentColor";
import { iconPathFor } from "./icons";

describe("paymentIcon", () => {
  it("the four methods of the shift income card each look like what they are", () => {
    expect(paymentIcon("Cash $$")).toBe("mdi-cash-multiple");
    expect(paymentIcon("Cash LL")).toBe("mdi-cash");
    expect(paymentIcon("Visa")).toBe("mdi-credit-card-outline");
    expect(paymentIcon("Whish")).toBe("mdi-cellphone");
  });
  it("is not the same icon for all four", () => {
    expect(new Set(["Cash $$", "Cash LL", "Visa", "Whish"].map(paymentIcon)).size).toBe(4);
  });
  it("reads the name any way it is cased or spelled", () => {
    for (const n of ["VISA", "visa debit", "Master Card", "Credit card", "MASTERCARD"]) expect(paymentIcon(n), n).toBe("mdi-credit-card-outline");
    for (const n of ["WHISH", "Whish Money", "whish"]) expect(paymentIcon(n), n).toBe("mdi-cellphone");
    for (const n of ["Cash LBP", "LL", "cash ll", "Lebanese Lira", "Lira"]) expect(paymentIcon(n), n).toBe("mdi-cash");
    for (const n of ["Bank", "Bank transfer", "Wire", "Cheque"]) expect(paymentIcon(n), n).toBe("mdi-bank");
  });
  it("anything else, empty or missing is plain cash", () => {
    for (const n of ["Cash", "Cash USD", "Cash $$", "Other", "", null, undefined, 42]) expect(paymentIcon(n), String(n)).toBe("mdi-cash-multiple");
  });
  it("a word that merely contains ll or card does not match", () => {
    expect(paymentIcon("Bella")).toBe("mdi-cash-multiple");
    expect(paymentIcon("Cardigan shop")).toBe("mdi-cash-multiple");
  });
  it("every icon it can return is registered (an unregistered one renders as text)", () => {
    for (const n of ["Visa", "Whish", "Bank", "Cash LL", "Cash $$"]) expect(iconPathFor(paymentIcon(n)), n).toBeTruthy();
  });
});
