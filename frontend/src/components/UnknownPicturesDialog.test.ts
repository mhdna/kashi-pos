// @vitest-environment happy-dom
//
// The payment's unknown-item pictures dialog. The reported case: the till's
// internet drops while the phone sends the pictures; the dialog must not get
// stuck saying pictures are needed, but try again by itself and close once
// central answers that every picture is in.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import ui from "@/ui";

const api = vi.hoisted(() => ({
  SyncUnknownItemSession: vi.fn(),
  UnknownItemSessionStatus: vi.fn(),
  RequestUnknownItemHelp: vi.fn(),
}));
vi.mock("../../wailsjs/go/main/App", () => api);
vi.mock("qrcode", () => ({ default: { toDataURL: vi.fn(async (url: string) => `data:qr,${url}`) } }));

import UnknownPicturesDialog from "./UnknownPicturesDialog.vue";

const units = [{ label: "Unknown item $25.00", unitPriceCents: 2500, discount: 0, lineRef: "a", slotRef: "a:1" }];
const done = { clientRef: "pics-1", uploadUrl: "https://central/u/pics-1", complete: true, slots: [{ slotRef: "a:1", lineRef: "a", label: "x", hasImage: true, waived: false, done: true }] };

let wrapper: VueWrapper<any> | null = null;
beforeEach(() => {
  vi.useFakeTimers();
  api.SyncUnknownItemSession.mockReset();
  api.UnknownItemSessionStatus.mockReset();
  api.UnknownItemSessionStatus.mockImplementation(() => new Promise(() => {}));
});
afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
  document.body.innerHTML = "";
  vi.useRealTimers();
});

function mountDialog(modelValue = true) {
  wrapper = mount(UnknownPicturesDialog, {
    props: { modelValue, units, clientRef: "pics-1" },
    global: { plugins: [ui] },
    attachTo: document.body,
  });
  return wrapper;
}

describe("UnknownPicturesDialog", () => {
  it("the reported case: offline at first, it retries by itself and closes once the pictures are in", async () => {
    api.SyncUnknownItemSession.mockRejectedValueOnce(new Error("couldn't reach central")).mockResolvedValue(done);
    mountDialog();
    await flushPromises();
    expect(document.body.textContent).toContain("couldn't reach central");
    expect(document.body.querySelector('[data-test="unknown-retrying"]')).not.toBeNull();
    expect(wrapper!.emitted("ready")).toBeUndefined();
    await vi.advanceTimersByTimeAsync(3000);
    await flushPromises();
    expect(api.SyncUnknownItemSession).toHaveBeenCalledTimes(2);
    expect(api.SyncUnknownItemSession.mock.calls[1][0]).toBe("pics-1");
    expect(wrapper!.emitted("ready")).toHaveLength(1);
  });

  it("keeps trying while central stays away, and stops once the cashier closes it", async () => {
    api.SyncUnknownItemSession.mockRejectedValue(new Error("couldn't reach central"));
    mountDialog();
    await flushPromises();
    await vi.advanceTimersByTimeAsync(9000);
    await flushPromises();
    expect(api.SyncUnknownItemSession).toHaveBeenCalledTimes(4);
    await wrapper!.setProps({ modelValue: false });
    await vi.advanceTimersByTimeAsync(9000);
    await flushPromises();
    expect(api.SyncUnknownItemSession).toHaveBeenCalledTimes(4);
    expect(wrapper!.emitted("ready")).toBeUndefined();
  });
});
