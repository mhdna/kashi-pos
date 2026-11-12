// @vitest-environment happy-dom
import { afterEach, describe, expect, it } from "vitest";
import { nextTick } from "vue";
import { mount, type VueWrapper } from "@vue/test-utils";
import ui from "@/ui";

let wrapper: VueWrapper<any> | null = null;
afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
});

const TABS = ["general", "money", "payments", "coupons", "behaviour", "display", "quick", "cash", "sync"];

function show(widths: { client: number; scroll: number }) {
  wrapper = mount(
    {
      template: `<k-tabs ref="tabs" v-model="tab"><k-tab v-for="t in tabs" :key="t" :value="t">{{ t }}</k-tab></k-tabs>`,
      data: () => ({ tab: "general", tabs: TABS }),
    },
    { global: { plugins: [ui] }, attachTo: document.body },
  );
  const track = wrapper.find(".k-tabs__track").element as HTMLElement;
  Object.defineProperty(track, "clientWidth", { configurable: true, value: widths.client });
  Object.defineProperty(track, "scrollWidth", { configurable: true, value: widths.scroll });
  (track as any).scrollTo = ({ left }: { left: number }) => { track.scrollLeft = left; };
  return { track, update: async () => { (wrapper!.vm.$refs.tabs as any).update(); await nextTick(); } };
}

describe("KTabs", () => {
  it("the reported case: Settings tabs wider than the screen can be scrolled to", async () => {
    const { track, update } = show({ client: 1000, scroll: 2300 });
    await update();
    expect(wrapper!.find('[data-test="tabs-prev"]').exists()).toBe(false);
    expect(wrapper!.find('[data-test="tabs-next"]').exists()).toBe(true);

    await wrapper!.find('[data-test="tabs-next"]').trigger("click");
    expect(track.scrollLeft).toBe(800);
    await nextTick();
    expect(wrapper!.find('[data-test="tabs-prev"]').exists()).toBe(true);
  });

  it("a mouse wheel scrolls the strip sideways", async () => {
    const { track } = show({ client: 1000, scroll: 2300 });
    const e = new WheelEvent("wheel", { deltaY: 120, cancelable: true });
    track.dispatchEvent(e);
    expect(track.scrollLeft).toBe(120);
    expect(e.defaultPrevented).toBe(true);
  });

  it("tabs that fit: no arrows, and the wheel scrolls the page as usual", async () => {
    const { track, update } = show({ client: 1000, scroll: 1000 });
    await update();
    expect(wrapper!.find('[data-test="tabs-next"]').exists()).toBe(false);
    expect(wrapper!.find('[data-test="tabs-prev"]').exists()).toBe(false);
    const e = new WheelEvent("wheel", { deltaY: 120, cancelable: true });
    track.dispatchEvent(e);
    expect(e.defaultPrevented).toBe(false);
  });

  it("tapping a tab still selects it", async () => {
    show({ client: 1000, scroll: 2300 });
    await wrapper!.findAll(".k-tab")[3]!.trigger("click");
    expect((wrapper!.vm as any).tab).toBe("coupons");
    expect(wrapper!.findAll(".k-tab")[3]!.classes()).toContain("k-tab--selected");
  });
});
