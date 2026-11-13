// @vitest-environment happy-dom
import { describe, expect, it } from "vitest";
import { mount } from "@vue/test-utils";
import ui from "@/ui";
import SalespersonGrid from "./SalespersonGrid.vue";

const PEOPLE = [
  { id: 3, fullName: "Ali A" },
  { id: 5, fullName: "Fatima G" },
  { id: 7, fullName: "Mahdi A" },
];

const mountGrid = (props: Record<string, unknown> = {}) =>
  mount(SalespersonGrid, { props: { salespersons: PEOPLE, modelValue: null, ...props }, global: { plugins: [ui] } });

describe("SalespersonGrid", () => {
  it("shows one tile per salesperson: initials over the name", () => {
    const w = mountGrid();
    const tiles = w.findAll("[role=option]");
    expect(tiles.map((t) => t.find(".sp-name").text())).toEqual(["Ali A", "Fatima G", "Mahdi A"]);
    expect(tiles[1]!.text()).toContain("FG");
    expect(w.find("[data-test=sp-default]").exists()).toBe(false);
  });

  it("tells which one is picked", () => {
    const w = mountGrid({ modelValue: 5 });
    const picked = w.findAll("[aria-selected=true]");
    expect(picked).toHaveLength(1);
    expect(picked[0]!.attributes("data-test")).toBe("sp-5");
  });

  it("emits the id of the tile tapped", async () => {
    const w = mountGrid();
    await w.find("[data-test=sp-7]").trigger("click");
    expect(w.emitted("update:modelValue")).toEqual([[7]]);
  });

  it("offers a default tile (id 0) only when given a label, and picks it", async () => {
    const w = mountGrid({ defaultLabel: "Invoice salesperson", modelValue: 0 });
    const def = w.find("[data-test=sp-default]");
    expect(def.text()).toBe("Invoice salesperson");
    expect(def.attributes("aria-selected")).toBe("true");
    expect(w.findAll("[aria-selected=true]")).toHaveLength(1);
    await w.find("[data-test=sp-3]").trigger("click");
    await def.trigger("click");
    expect(w.emitted("update:modelValue")).toEqual([[3], [0]]);
  });

  it("with no default tile, nothing is picked for null and for 0", () => {
    expect(mountGrid({ modelValue: null }).findAll("[aria-selected=true]")).toHaveLength(0);
    expect(mountGrid({ modelValue: 0 }).findAll("[aria-selected=true]")).toHaveLength(0);
  });
});
