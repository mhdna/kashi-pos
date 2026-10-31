import { describe, expect, it } from "vitest";
import { onColor, resolveColor } from "./colors";

describe("resolveColor", () => {
  it("follows the theme for theme names", () => {
    expect(resolveColor("primary")).toBe("rgb(var(--k-theme-primary))");
  });
  it("maps Material names to hex", () => {
    expect(resolveColor("blue-grey-darken-1")).toBe("#546e7a");
    expect(resolveColor("deep-purple")).toBe("#673ab7");
  });
  it("passes other values through", () => {
    expect(resolveColor("#123456")).toBe("#123456");
    expect(resolveColor(undefined)).toBeUndefined();
  });
});

describe("onColor", () => {
  it("is white on dark fills and black on light ones", () => {
    expect(onColor("#0d47a1")).toBe("#fff");
    expect(onColor("amber")).toBe("#000");
    expect(onColor("primary")).toBe("rgb(var(--k-theme-on-primary))");
  });
});
