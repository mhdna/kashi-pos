// @vitest-environment happy-dom
// In card actions an uncolored button stays text (Cancel), but a colored one
// (Save, Confirm, Delete) gets a solid fill instead of plain colored text.
import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import KBtn from "./KBtn.vue";
import KCardActions from "./KCardActions.vue";

const mountIn = (template: string) => mount({ components: { KBtn, KCardActions }, template });

describe("buttons in card actions", () => {
  it("a colored button is flat", () => {
    const w = mountIn('<k-card-actions><k-btn color="primary">Save</k-btn></k-card-actions>');
    expect(w.find(".k-btn").classes()).toContain("k-btn--variant-flat");
  });
  it("an uncolored button stays text", () => {
    const w = mountIn("<k-card-actions><k-btn>Cancel</k-btn></k-card-actions>");
    expect(w.find(".k-btn").classes()).toContain("k-btn--variant-text");
  });
  it("an explicit variant still wins", () => {
    const w = mountIn('<k-card-actions><k-btn color="error" variant="text">Reject</k-btn></k-card-actions>');
    expect(w.find(".k-btn").classes()).toContain("k-btn--variant-text");
  });
});
