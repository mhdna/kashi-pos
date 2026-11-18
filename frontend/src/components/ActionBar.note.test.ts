// @vitest-environment happy-dom
//
// The invoice note sits in the action bar, just before Hold.
import { afterEach, describe, expect, it } from "vitest";
import { mount, type VueWrapper } from "@vue/test-utils";
import ui from "@/ui";
import ActionBar from "./ActionBar.vue";

let wrapper: VueWrapper | null = null;
afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
});

function bar(hasNote = false, hasItems = true) {
  wrapper = mount(ActionBar, {
    props: { total: 10, totalSummary: { first: "$10.00", second: "" }, canCheckout: true, hasItems, mode: "sale", hasNote },
    global: { plugins: [ui] },
    attachTo: document.body,
  });
  return wrapper;
}

describe("ActionBar Note", () => {
  it("the reported case: Note comes right before Hold, before Check Out", () => {
    const labels = bar().findAll("button").map((b) => b.text().trim());
    expect(labels).toEqual(["Cancel", "Note", "Hold", "Check Out"]);
  });
  it("opens the note, even with an empty cart", async () => {
    const w = bar(false, false);
    await w.find('[data-test="note-open"]').trigger("click");
    expect(w.emitted("note")).toHaveLength(1);
  });
  it("shows when the sale has a note: its look changes, its label stays", () => {
    const withNote = bar(true).find('[data-test="note-open"]');
    const withNoteHtml = withNote.html();
    expect(withNote.text().trim()).toBe("Note");
    wrapper!.unmount();
    const without = bar(false).find('[data-test="note-open"]');
    expect(without.text().trim()).toBe("Note");
    expect(without.html()).not.toBe(withNoteHtml);
  });
  it("only the icon turns green; the word stays grey", () => {
    const w = bar(true);
    const button = w.find('[data-test="note-open"]');
    const icon = w.find('[data-test="note-icon"]');
    const plain = bar(false);
    expect(button.attributes("style") ?? "").toBe(plain.find('[data-test="note-open"]').attributes("style") ?? "");
    expect(button.classes()).toEqual(plain.find('[data-test="note-open"]').classes());
    expect(icon.html()).not.toBe(plain.find('[data-test="note-icon"]').html());
  });
});
