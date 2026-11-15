// The phone number screen stays neutral: the country Search button and the
// keypad's C are grey like the other keys, and the country page arrows have no
// background.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const dialog = readFileSync(resolve(__dirname, "ClientDialog.vue"), "utf8");
const pad = readFileSync(resolve(__dirname, "PhonePad.vue"), "utf8");
const rule = (src: string, sel: string) => src.slice(src.indexOf(`${sel} {`), src.indexOf("}", src.indexOf(`${sel} {`)));

describe("phone number screen colours", () => {
  it("the country Search button is neutral grey, not teal", () => {
    const css = rule(dialog, ".client-phone-country-trigger-search");
    expect(css).not.toMatch(/#00796b/i);
    expect(css).toMatch(/background: rgba\(var\(--k-theme-on-surface\), 0\.12\)/);
  });

  it("the page arrows are text buttons with no background", () => {
    const pager = dialog.slice(dialog.indexOf('<div class="client-phone-country-pager">'), dialog.indexOf("</div>", dialog.indexOf('<div class="client-phone-country-pager">')));
    expect(pager).not.toMatch(/variant="tonal"/);
    expect(pager.match(/variant="text"/g)).toHaveLength(2);
  });

  it("the country cards are square, with no rounded corners", () => {
    expect(rule(dialog, ".client-phone-country-card")).toMatch(/border-radius: 0;/);
  });

  it("the C key has no red of its own", () => {
    expect(pad).not.toMatch(/\.phone-pad-key--clear \{[^}]*error/);
  });
});
