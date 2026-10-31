import { describe, expect, it } from "vitest";
import { isSelected, normaliseItems, toggleSelection } from "./items";

describe("normaliseItems", () => {
  it("uses primitives as title and value", () => {
    expect(normaliseItems(["a", 2])).toEqual([
      { title: "a", value: "a", raw: "a" },
      { title: "2", value: 2, raw: 2 },
    ]);
  });
  it("reads item-title / item-value keys", () => {
    const [item] = normaliseItems([{ id: 7, name: "Bob" }], "name", "id");
    expect(item).toMatchObject({ title: "Bob", value: 7 });
  });
  it("defaults to title/value and falls back to the object", () => {
    const [a] = normaliseItems([{ title: "T", value: "v" }]);
    expect(a).toMatchObject({ title: "T", value: "v" });
    const obj = { x: 1 };
    const [b] = normaliseItems([obj]);
    expect(b!.value).toBe(obj);
  });
  it("accepts a title function", () => {
    expect(normaliseItems([{ id: 1, n: "A" }], (i) => `#${i.id} ${i.n}`, "id")[0]!.title).toBe("#1 A");
  });
});

describe("selection", () => {
  it("replaces when single", () => {
    expect(toggleSelection(1, 2, false)).toBe(2);
  });
  it("toggles when multiple", () => {
    expect(toggleSelection(["a"], "b", true)).toEqual(["a", "b"]);
    expect(toggleSelection(["a", "b"], "a", true)).toEqual(["b"]);
    expect(toggleSelection(undefined, "a", true)).toEqual(["a"]);
  });
  it("reports selection", () => {
    expect(isSelected(["a"], "a", true)).toBe(true);
    expect(isSelected("a", "b", false)).toBe(false);
  });
});
