// @vitest-environment happy-dom
//
// Record photo thumbnails: fetched once per cache key, and fetched again
// when the till says the record's photos changed in central (a photo turned,
// replaced or added), so the row shows the new one without reopening the
// page.
import { afterEach, describe, expect, it, vi } from "vitest";
import { defineComponent, h, nextTick } from "vue";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import ui from "@/ui";

type Listener = (...args: any[]) => void;
const listeners = new Map<string, Set<Listener>>();
vi.mock("../../wailsjs/runtime/runtime", () => ({
  EventsOn: vi.fn((name: string, cb: Listener) => {
    if (!listeners.has(name)) listeners.set(name, new Set());
    listeners.get(name)!.add(cb);
    return () => listeners.get(name)!.delete(cb);
  }),
}));
function emit(name: string, ...args: any[]) {
  for (const cb of listeners.get(name) ?? []) cb(...args);
}

import RecordPhoto from "./RecordPhoto.vue";
import { onRecordPhotosChanged, recordPhotosChangedEvent, usePhotoVersions } from "@/lib/recordPhotos";

let wrappers: VueWrapper<any>[] = [];
afterEach(() => {
  wrappers.forEach((w) => w.unmount());
  wrappers = [];
  listeners.clear();
  (globalThis as any).__recordPhotoCache?.clear();
});

function mountPhoto(cacheKey: string, load: () => Promise<string>) {
  const w = mount(RecordPhoto, { props: { cacheKey, load }, global: { plugins: [ui] } });
  wrappers.push(w);
  return w;
}

// A page's row list, wired the way expenses, loans and transfers are: the
// thumbnail key carries the record's photo version, and the change event
// bumps it.
const Rows = defineComponent({
  props: { refs: { type: Array as () => string[], required: true }, load: { type: Function, required: true } },
  setup() {
    const { cacheKey, bump } = usePhotoVersions();
    const unsub = onRecordPhotosChanged(bump);
    return { cacheKey, unsub };
  },
  unmounted() {
    this.unsub();
  },
  render() {
    return h(
      "div",
      this.refs.map((ref) =>
        h(RecordPhoto, { key: ref, class: `row-${ref}`, cacheKey: this.cacheKey("expense-thumb", ref), load: () => (this.load as any)(ref) }),
      ),
    );
  },
});

describe("RecordPhoto", () => {
  it("shows the photo the till returns", async () => {
    const w = mountPhoto("expense-thumb-exp-1-0", async () => "data:image/webp;base64,AAA");
    await flushPromises();
    expect(w.find("img").attributes("src")).toBe("data:image/webp;base64,AAA");
  });

  it("shows the no-photo icon for an empty answer and for a failed one", async () => {
    const none = mountPhoto("k-none", async () => "");
    const failed = mountPhoto("k-failed", async () => {
      throw new Error("offline");
    });
    await flushPromises();
    expect(none.find("img").exists()).toBe(false);
    expect(failed.find("img").exists()).toBe(false);
  });

  it("asks the till once per key, however many times the row is drawn", async () => {
    const load = vi.fn(async () => "data:a");
    mountPhoto("same-key", load);
    mountPhoto("same-key", load);
    await flushPromises();
    expect(load).toHaveBeenCalledTimes(1);
  });

  it("asks again when the key changes", async () => {
    const load = vi.fn(async () => "data:a");
    const w = mountPhoto("key-0", load);
    await flushPromises();
    load.mockResolvedValue("data:b");
    await w.setProps({ cacheKey: "key-1" });
    await flushPromises();
    expect(load).toHaveBeenCalledTimes(2);
    expect(w.find("img").attributes("src")).toBe("data:b");
  });
});

describe("rows when central changes a photo", () => {
  it("refetches only the changed record's thumbnail and shows the new one", async () => {
    let version = "v1";
    const load = vi.fn(async (ref: string) => `data:${ref}-${version}`);
    const w = mount(Rows, { props: { refs: ["exp-1", "exp-2"], load }, global: { plugins: [ui] } });
    wrappers.push(w);
    await flushPromises();
    expect(load).toHaveBeenCalledTimes(2);

    version = "v2";
    emit(recordPhotosChangedEvent, "exp-1");
    await nextTick();
    await flushPromises();
    expect(load).toHaveBeenCalledTimes(3);
    expect(load).toHaveBeenLastCalledWith("exp-1");
    expect(w.find(".row-exp-1 img").attributes("src")).toBe("data:exp-1-v2");
    expect(w.find(".row-exp-2 img").attributes("src")).toBe("data:exp-2-v1");
  });

  it("ignores an event for a record not on screen, and empty or odd payloads", async () => {
    const load = vi.fn(async (ref: string) => `data:${ref}`);
    const w = mount(Rows, { props: { refs: ["exp-1"], load }, global: { plugins: [ui] } });
    wrappers.push(w);
    await flushPromises();
    emit(recordPhotosChangedEvent, "exp-9");
    emit(recordPhotosChangedEvent, "");
    emit(recordPhotosChangedEvent, undefined);
    emit(recordPhotosChangedEvent, 42);
    await flushPromises();
    expect(load).toHaveBeenCalledTimes(1);
  });

  it("stops listening once the page is closed", async () => {
    const w = mount(Rows, { props: { refs: ["exp-1"], load: async () => "" }, global: { plugins: [ui] } });
    await flushPromises();
    w.unmount();
    expect(listeners.get(recordPhotosChangedEvent)?.size ?? 0).toBe(0);
  });
});
