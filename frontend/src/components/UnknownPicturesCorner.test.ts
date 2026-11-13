// @vitest-environment happy-dom
//
// The sale screen's unknown-item QR strip. A cart that already has every
// picture (it came back from a hold, or after logout) must not ask central
// for its QR code again; a cart that changed must.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import ui from "@/ui";

const api = vi.hoisted(() => ({
  SyncUnknownItemSession: vi.fn(),
  UnknownItemSessionStatus: vi.fn(),
}));
vi.mock("../../wailsjs/go/main/App", () => api);
vi.mock("qrcode", () => ({ default: { toDataURL: vi.fn(async (url: string) => `data:qr,${url}`) } }));

import UnknownPicturesCorner from "./UnknownPicturesCorner.vue";

const unit = (line: string, n = 1) => ({
  label: "Unknown item $25.00",
  unitPriceCents: 2500,
  discount: 0,
  lineRef: line,
  slotRef: `${line}:${n}`,
});
const TWO = [unit("a"), unit("b")];
const sig = (units: any[]) => JSON.stringify(units);

let wrapper: VueWrapper<any> | null = null;
beforeEach(() => {
  vi.useFakeTimers();
  api.SyncUnknownItemSession.mockReset();
  api.UnknownItemSessionStatus.mockReset();
  api.SyncUnknownItemSession.mockImplementation(async (clientRef: string) => ({
    clientRef: clientRef || "pics-1",
    uploadUrl: "https://central/u/pics-1",
    slots: [],
    complete: false,
  }));
  // A long poll that never answers, unless a test says otherwise.
  api.UnknownItemSessionStatus.mockImplementation(() => new Promise(() => {}));
});
afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
  vi.useRealTimers();
});

async function mountCorner(props: Record<string, unknown>) {
  wrapper = mount(UnknownPicturesCorner, {
    props: { units: TWO, clientRef: "pics-1", ...props },
    global: { plugins: [ui] },
  });
  await vi.advanceTimersByTimeAsync(400);
  await flushPromises();
  return wrapper;
}

describe("UnknownPicturesCorner", () => {
  it("a cart already pictured shows done and never asks central", async () => {
    const w = await mountCorner({ picturedFor: sig(TWO) });
    await vi.advanceTimersByTimeAsync(10_000);
    expect(api.SyncUnknownItemSession).not.toHaveBeenCalled();
    expect(api.UnknownItemSessionStatus).not.toHaveBeenCalled();
    expect(w.find("[data-test=unknown-corner-pictured]").exists()).toBe(true);
    expect(w.find("img").exists()).toBe(false);
    expect(w.text()).toContain("2 of 2 pictured");
    expect(w.find(".unknown-corner--done").exists()).toBe(true);
  });

  it("a line with two units counts as one pictured line", async () => {
    const units = [unit("a", 1), unit("a", 2), unit("b")];
    const w = await mountCorner({ units, picturedFor: sig(units) });
    expect(w.text()).toContain("2 of 2 pictured");
  });

  it("syncs when nothing was pictured yet", async () => {
    const w = await mountCorner({ picturedFor: "" });
    expect(api.SyncUnknownItemSession).toHaveBeenCalledTimes(1);
    expect(w.find("img").attributes("src")).toBe("data:qr,https://central/u/pics-1");
  });

  it("syncs when the pictured cart was a different one", async () => {
    await mountCorner({ picturedFor: sig([unit("a")]) });
    expect(api.SyncUnknownItemSession).toHaveBeenCalledTimes(1);
  });

  it("syncs without a session, whatever it was told", async () => {
    await mountCorner({ clientRef: "", picturedFor: sig(TWO) });
    expect(api.SyncUnknownItemSession).toHaveBeenCalledTimes(1);
  });

  it("an item added after the pictures were taken asks central again", async () => {
    const w = await mountCorner({ picturedFor: sig(TWO) });
    expect(api.SyncUnknownItemSession).not.toHaveBeenCalled();
    await w.setProps({ units: [...TWO, unit("c")] });
    await vi.advanceTimersByTimeAsync(400);
    await flushPromises();
    expect(api.SyncUnknownItemSession).toHaveBeenCalledTimes(1);
    expect(w.find("[data-test=unknown-corner-pictured]").exists()).toBe(false);
  });

  it("stops asking central once the page records the cart as pictured", async () => {
    let polls = 0;
    api.UnknownItemSessionStatus.mockImplementation(async () => {
      polls++;
      await new Promise((resolve) => setTimeout(resolve, 100));
      return { slots: TWO.map((u) => ({ ...u, hasImage: true, waived: false, done: true })), complete: true };
    });
    const w = await mountCorner({ picturedFor: "" });
    await vi.advanceTimersByTimeAsync(250);
    expect(w.emitted("ready")?.at(-1)).toEqual([sig(TWO)]);
    await w.setProps({ picturedFor: sig(TWO) });
    await vi.advanceTimersByTimeAsync(200);
    const after = polls;
    await vi.advanceTimersByTimeAsync(2_000);
    expect(polls).toBe(after);
    // The QR it already had stays up.
    expect(w.find("img").exists()).toBe(true);
    expect(w.text()).toContain("2 of 2 pictured");
  });
});
