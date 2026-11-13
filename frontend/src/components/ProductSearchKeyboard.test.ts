// @vitest-environment happy-dom
//
// The product search's bottom bar: the rows/columns toggle is for admins only.
import { afterEach, describe, expect, it, vi } from "vitest";
import { mount, type VueWrapper } from "@vue/test-utils";
import { ref } from "vue";
import ui from "@/ui";

const isAdmin = ref(true);
vi.mock("@/stores/auth", () => ({ useAuthStore: () => ({ get isAdmin() { return isAdmin.value; } }) }));

import ProductSearchKeyboard from "./ProductSearchKeyboard.vue";

let wrapper: VueWrapper<any> | null = null;
afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
  isAdmin.value = true;
});

const mountBar = (props: Record<string, unknown> = {}) =>
  (wrapper = mount(ProductSearchKeyboard, {
    props: { modelValue: "", gridMode: false, ...props },
    global: { plugins: [ui] },
  }));

describe("product search layout toggle", () => {
  it("is there for an admin, and flips the layout", async () => {
    const w = mountBar();
    const toggle = w.find(".product-layout-toggle");
    expect(toggle.exists()).toBe(true);
    await toggle.trigger("click");
    expect(w.emitted("update:gridMode")).toEqual([[true]]);
  });

  it("is gone for everyone else, whatever the layout is", () => {
    isAdmin.value = false;
    expect(mountBar().find(".product-layout-toggle").exists()).toBe(false);
    wrapper!.unmount();
    expect(mountBar({ gridMode: true }).find(".product-layout-toggle").exists()).toBe(false);
  });

  it("still has the search field, the keyboard and Close for a cashier", () => {
    isAdmin.value = false;
    const w = mountBar();
    expect(w.find(".product-search-control").exists()).toBe(true);
    expect(w.find(".product-inline-keyboard").exists()).toBe(true);
    expect(w.find(".product-search-close").exists()).toBe(true);
  });

  it("the search field spans the row when the toggle is not there", () => {
    isAdmin.value = false;
    const w = mountBar();
    expect(w.find(".product-search-row").element.children.length).toBe(1);
  });
});
