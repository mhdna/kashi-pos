// @vitest-environment happy-dom
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

// The unknown-item pictures session (the QR on the sale screen) must survive
// leaving the sale screen and coming back: its ref rides in the cart draft.
// home.vue is too big to mount; its wiring is read here.
describe("sale screen unknown-item session is kept in the draft", () => {
  const src = readFileSync(resolve(process.cwd(), "src/pages/home.vue"), "utf8");
  it("writes the session ref into the saved draft", () => {
    expect(src).toMatch(/unknownClientRef: unknownSession\.clientRef \|\| undefined/);
  });
  it("saves the draft again when the session ref changes", () => {
    expect(src).toMatch(/\(\) => unknownSession\.clientRef,\s*(unknownPicturedFor,\s*)?\],/);
  });
  it("restores the session ref from the draft (empty when there is none)", () => {
    expect(src).toMatch(/unknownSession\.clientRef = draft\.unknownClientRef \?\? ""/);
  });
  it("declares the session before the draft watcher reads it", () => {
    expect(src.indexOf("const unknownSession = reactive")).toBeLessThan(src.indexOf("() => unknownSession.clientRef,"));
  });
  it("a finished or cleared sale still forgets the session", () => {
    expect(src).toMatch(/unknownSession\.clientRef = "";\s*clearDraft\(\);/);
  });
  it("writes, watches and restores that the cart was pictured, with its session only", () => {
    expect(src).toMatch(/unknownPicturedFor: \(unknownSession\.clientRef && unknownPicturedFor\.value\) \|\| undefined/);
    expect(src).toMatch(/\(\) => unknownSession\.clientRef,\s*unknownPicturedFor,\s*\],/);
    expect(src).toMatch(/unknownPicturedFor\.value = \(draft\.unknownClientRef && draft\.unknownPicturedFor\) \|\| ""/);
    expect(src.indexOf("const unknownPicturedFor = ref")).toBeLessThan(src.indexOf("unknownPicturedFor,\n  ],"));
  });
  it("a new sale forgets it was pictured", () => {
    expect(src).toMatch(/unknownPicturedFor\.value = "";\s*unknownSession\.clientRef = "";\s*clearDraft\(\);/);
  });
  it("tells the QR strip, and checkout skips the pictures, when the cart is the pictured one", () => {
    expect(src).toMatch(/:pictured-for="unknownPicturedFor"/);
    expect(src).toMatch(/@ready="unknownPicturedFor = \$event"/);
    expect(src).toMatch(/if \(signature === unknownPicturedFor\.value\) return true;/);
  });
});
