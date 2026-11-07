import { describe, expect, it } from "vitest";
import { colourStops, swatchBackground } from "./colorSwatch";

describe("colourStops", () => {
  it("knows the colours on the till's shoes", () => {
    expect(colourStops("Grey")).toEqual(["#8a8a8a"]);
    expect(colourStops("Beige")).toEqual(["#d9c7a7"]);
    expect(colourStops("White")).toEqual(["#ffffff"]);
    expect(colourStops("Black")).toEqual(["#111111"]);
  });

  it("ignores case, spaces and dashes", () => {
    expect(colourStops("  navy ")).toEqual(colourStops("Navy"));
    expect(colourStops("OFF-WHITE")).toEqual(colourStops("off white"));
    expect(colourStops("Gray")).toEqual(colourStops("grey"));
  });

  it("paints each part of a multi-colour name, in order", () => {
    expect(colourStops("Black/White/Grey")).toEqual(["#111111", "#ffffff", "#8a8a8a"]);
    expect(colourStops("Brown/Black")).toEqual(["#6f4e37", "#111111"]);
    expect(colourStops("Green & Blue")).toEqual(["#2e9e4f", "#2463eb"]);
  });

  it("reads the catalogue's short forms", () => {
    expect(colourStops("Pur/Tra")).toEqual(["#7b3fa0", "transparent"]);
    expect(colourStops("BLU/SHW")).toEqual(["#2463eb"]); // SHW unknown: skipped
    expect(colourStops("Yel/Cuv")).toEqual(["#f5d000"]);
    expect(colourStops("Ora/Cuv")).toEqual(["#f28c28"]);
    expect(colourStops("Red/Tra")).toEqual(["#d62828", "transparent"]);
  });

  it("lightens and darkens", () => {
    const [light] = colourStops("Light Gray");
    const [dark] = colourStops("Dark Green");
    expect(light).not.toBe("#8a8a8a");
    expect(parseInt(light!.slice(1, 3), 16)).toBeGreaterThan(0x8a);
    expect(dark).toBe("#1f5130"); // a named shade wins over the modifier
    expect(parseInt(colourStops("dark red")[0]!.slice(1, 3), 16)).toBeLessThan(0xd6);
  });

  it("takes a hex value as it is", () => {
    expect(colourStops("#1976D2")).toEqual(["#1976d2"]);
    expect(colourStops("#fff")).toEqual(["#fff"]);
  });

  it("paints a rainbow for Colored / Multi", () => {
    expect(colourStops("Colored").length).toBeGreaterThan(3);
    expect(colourStops("multi-color")).toEqual(colourStops("Colored"));
  });

  it("gives nothing for a blank, #N/A or an unknown name", () => {
    for (const name of ["", "   ", "#N/A", "N/A", "na", "Cuval", "Tra/Cuval".slice(4), null, undefined]) {
      expect(colourStops(name as any), String(name)).toEqual([]);
    }
  });

  it("doesn't repeat a colour twice in a row", () => {
    expect(colourStops("Black/Black")).toEqual(["#111111"]);
    expect(colourStops("Pur/Pur")).toEqual(["#7b3fa0"]);
  });

  it("every stop is a hex colour or transparent, whatever the name", () => {
    const names = ["Black", "Red/Tra", "white/cuv", "Purple/Cuv", "Olive", "Bordeaux", "Camel", "Greige", "Clear", "Light Gray", "Colored", "x/y/z"];
    for (const n of names) for (const s of colourStops(n)) expect(s).toMatch(/^(#[0-9a-f]{3,6}|transparent)$/);
  });
});

describe("swatchBackground", () => {
  it("fills with one colour", () => {
    expect(swatchBackground(["#111111"])).toBe("#111111");
  });
  it("slices several colours evenly", () => {
    expect(swatchBackground(["#111111", "#ffffff"])).toBe("conic-gradient(#111111 0.00% 50.00%, #ffffff 50.00% 100.00%)");
    const three = swatchBackground(["#a", "#b", "#c"].map((x) => x.padEnd(7, "0")));
    expect(three).toContain("33.33%");
    expect(three).toContain("66.67%");
  });
  it("shows transparent as a checkerboard", () => {
    expect(swatchBackground(["transparent"])).toContain("repeating-conic-gradient");
    const mixed = swatchBackground(["#d62828", "transparent"]);
    expect(mixed).toContain("#d62828 0.00% 50.00%");
    expect(mixed).toContain("transparent 50.00% 100.00%");
    expect(mixed).toContain("repeating-conic-gradient");
  });
  it("is transparent with nothing known", () => {
    expect(swatchBackground([])).toBe("transparent");
  });
});
