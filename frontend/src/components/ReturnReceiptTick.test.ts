// @vitest-environment happy-dom
import { describe, expect, it } from "vitest";
import { mount } from "@vue/test-utils";
import ui from "@/ui";
import ReturnReceiptTick from "./ReturnReceiptTick.vue";

function mountTick(modelValue: boolean) {
  return mount(ReturnReceiptTick, { props: { modelValue }, global: { plugins: [ui] } });
}

describe("ReturnReceiptTick", () => {
  it("shows Print receipt, ticked as given", () => {
    const ticked = mountTick(true);
    expect(ticked.text()).toContain("Print receipt");
    expect((ticked.find("input[type=checkbox]").element as HTMLInputElement).checked).toBe(true);
    const unticked = mountTick(false);
    expect((unticked.find("input[type=checkbox]").element as HTMLInputElement).checked).toBe(false);
  });

  it("unticking and ticking again reach the page", async () => {
    const wrapper = mountTick(true);
    const box = wrapper.find("input[type=checkbox]");
    await box.setValue(false);
    await wrapper.setProps({ modelValue: false });
    await box.setValue(true);
    expect(wrapper.emitted("update:modelValue")).toEqual([[false], [true]]);
  });
});
