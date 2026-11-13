// @vitest-environment happy-dom
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

// A hold keeps the whole sale (lib/holdSnapshot.ts heldCart). home.vue is too
// big to mount; its wiring is read here.
describe("sale screen hold and restore", () => {
  const src = readFileSync(resolve(process.cwd(), "src/pages/home.vue"), "utf8");
  const onHold = src.slice(src.indexOf("async function onHold()"), src.indexOf("async function onVoid()"));
  const onRestore = src.slice(src.indexOf("function onRestoreHold("), src.indexOf("function consumePendingHold()"));

  it("holds the sale's ref and its unknown-item picture session", () => {
    expect(onHold).toMatch(/saleRef: saleRef\.value/);
    expect(onHold).toMatch(/unknownClientRef: unknownSession\.clientRef \|\| undefined/);
  });
  it("puts the lines back as held, never re-adding them", () => {
    expect(onRestore).toMatch(/invoiceItems\.value\.items = held\.restore\.items/);
    expect(onRestore).not.toMatch(/\.addItem\(/);
  });
  it("holds and restores that the cart was already pictured", () => {
    expect(onHold).toMatch(/unknownPicturedFor: unknownPicturedFor\.value \|\| undefined/);
    expect(onRestore).toMatch(/unknownPicturedFor\.value = held\.unknownPicturedFor/);
  });
  it("restores the sale's ref and picture session", () => {
    expect(onRestore).toMatch(/saleRef\.value = held\.saleRef/);
    expect(onRestore).toMatch(/unknownSession\.clientRef = held\.unknownClientRef/);
  });
  it("resumes the hold (central's overrides come back) and then shows them", () => {
    expect(onRestore).toMatch(/ResumeHoldInvoice\(hold\.id\)\s*\.then\(\(\) => syncLiveOverrides\(false\)\)/);
    expect(onRestore).not.toMatch(/DeleteHoldInvoice/);
  });
  it("sets the ref before resuming, so the next report is the held sale's", () => {
    expect(onRestore.indexOf("saleRef.value = held.saleRef")).toBeLessThan(onRestore.indexOf("ResumeHoldInvoice("));
  });
});
