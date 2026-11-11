// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { ref } from "vue";
import ui from "@/ui";
import { ACTIVITY_HOLD_MS } from "@/lib/linkStatus";

const nowIso = () => new Date().toISOString();
const baseLink = () => ({
  internet: "up", kashi: "up", kashiError: "", kashiLastOk: nowIso(), kashiDownForSeconds: 0,
  live: true, liveSince: nowIso(), liveLastBeat: nowIso(), liveError: "", liveRetryAt: "", liveEverUp: true,
  phase: "idle", label: "", detail: "", done: 0, total: 0, lastPullAt: nowIso(), lastPushAt: nowIso(), lastPullError: "",
});
const linkRef = ref<any>(baseLink());
const queueRef = ref<any>({ configured: true, pending: 0, failed: 0, confirmedElsewhere: 0 });
const isAdmin = ref(true);
const push = vi.fn();

vi.mock("@/composables/useLinkStatus", () => ({ useLinkStatus: () => ({ link: linkRef, now: ref(Date.now()), refresh: vi.fn() }) }));
vi.mock("@/composables/useSyncStatus", () => ({ useSyncStatus: () => ({ status: queueRef, loading: ref(false), refresh: vi.fn() }) }));
vi.mock("@/stores/auth", () => ({ useAuthStore: () => ({ get isAdmin() { return isAdmin.value; } }) }));
vi.mock("@/stores/resync", () => ({ useResyncStore: () => ({ running: false, progress: "" }) }));
vi.mock("vue-router", () => ({ useRouter: () => ({ push }) }));

import SyncIndicator from "./SyncIndicator.vue";

let wrapper: VueWrapper<any> | null = null;
afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
  document.body.innerHTML = "";
  linkRef.value = baseLink();
  queueRef.value = { configured: true, pending: 0, failed: 0, confirmedElsewhere: 0 };
  isAdmin.value = true;
  push.mockClear();
});

function show() {
  setActivePinia(createPinia());
  wrapper = mount(SyncIndicator, { global: { plugins: [ui] }, attachTo: document.body });
  return wrapper;
}
const text = (sel: string) => wrapper!.find(`[data-test="${sel}"]`).text();

describe("SyncIndicator", () => {
  it("healthy: Synced, both link icons green, no spinner or extra line", () => {
    show();
    expect(text("sync-word")).toBe("Synced");
    expect(wrapper!.find('[data-test="sync-sub"]').exists()).toBe(false);
    expect(wrapper!.find('[data-test="sync-spinner"]').exists()).toBe(false);
    expect(wrapper!.find('[data-test="sync-internet"]').classes()).toContain("link-dot--ok");
    expect(wrapper!.find('[data-test="sync-live"]').classes()).toContain("link-dot--ok");
  });

  it("the reported case: synced but not live shows amber and says Not live", () => {
    linkRef.value = { ...baseLink(), live: false, liveError: "read: closed" };
    show();
    expect(text("sync-word")).toBe("Not live");
    expect(wrapper!.find('[data-test="sync-live"]').classes()).toContain("link-dot--attention");
    expect(wrapper!.find('[data-test="sync-internet"]').classes()).toContain("link-dot--ok");
  });

  it("a pull shows a spinner and the entity it is on", () => {
    linkRef.value = { ...baseLink(), phase: "pulling", label: "Products", done: 2, total: 21 };
    show();
    expect(wrapper!.find('[data-test="sync-spinner"]').exists()).toBe(true);
    expect(text("sync-word")).toBe("Updating");
    expect(text("sync-sub")).toBe("Products 3/21");
  });

  it("no internet shows the wifi-off state in red", () => {
    linkRef.value = { ...baseLink(), internet: "down", kashi: "down", kashiError: "can't find the server", live: false };
    queueRef.value = { configured: true, pending: 2, failed: 0, confirmedElsewhere: 0 };
    show();
    expect(text("sync-word")).toBe("No internet");
    expect(text("sync-sub")).toBe("2 waiting");
    expect(wrapper!.find('[data-test="sync-internet"]').classes()).toContain("link-dot--bad");
  });

  it("tapping opens the detail with every row, for any user", async () => {
    isAdmin.value = false;
    linkRef.value = { ...baseLink(), live: false, liveError: "read: closed", liveRetryAt: new Date(Date.now() + 8000).toISOString() };
    show();
    await wrapper!.find('[data-test="sync-indicator"]').trigger("click");
    await flushPromises();
    const dialog = document.body.querySelector('[data-test="sync-dialog"]');
    expect(dialog).toBeTruthy();
    for (const key of ["internet", "kashi", "live", "now", "received", "sent", "queue"]) {
      expect(dialog!.querySelector(`[data-test="sync-row-${key}"]`), key).toBeTruthy();
    }
    expect(dialog!.querySelector('[data-test="sync-row-live"]')!.textContent).toContain("Not connected");
    expect(dialog!.querySelector('[data-test="sync-row-live"]')!.textContent).toContain("Trying again in");
    expect(dialog!.querySelector('[data-test="sync-open-settings"]')).toBeNull();
  });

  it("an admin can jump from the detail to the sync settings", async () => {
    show();
    await wrapper!.find('[data-test="sync-indicator"]').trigger("click");
    await flushPromises();
    (document.body.querySelector('[data-test="sync-open-settings"]') as HTMLElement).click();
    expect(push).toHaveBeenCalledWith({ path: "/settings", query: { panel: "general-sync" } });
  });

  it("one checkout reads as Sending, then Synced a moment later, not a flicker", async () => {
    vi.useFakeTimers();
    try {
      show();
      expect(text("sync-word")).toBe("Synced");

      // A pass sends records...
      linkRef.value = { ...baseLink(), phase: "pushing", label: "Sending to kashi", done: 0, total: 3 };
      await flushPromises();
      expect(text("sync-word")).toBe("Sending");

      // ...is idle for a beat between passes: the light must not flash Synced.
      linkRef.value = baseLink();
      await flushPromises();
      expect(text("sync-word")).toBe("Sending");

      // A second pass starts inside the hold: still busy.
      vi.advanceTimersByTime(1000);
      linkRef.value = { ...baseLink(), phase: "pulling", label: "Stock", done: 0, total: 1 };
      await flushPromises();
      expect(text("sync-word")).toBe("Updating");

      // Quiet for the whole hold: now it settles.
      linkRef.value = baseLink();
      await flushPromises();
      expect(text("sync-word")).toBe("Updating");
      vi.advanceTimersByTime(ACTIVITY_HOLD_MS + 100);
      await flushPromises();
      expect(text("sync-word")).toBe("Synced");
    } finally {
      vi.useRealTimers();
    }
  });

  it("a problem shows at once, even right after work", async () => {
    vi.useFakeTimers();
    try {
      show();
      linkRef.value = { ...baseLink(), phase: "pushing", label: "Sending to kashi", done: 0, total: 3 };
      await flushPromises();
      linkRef.value = { ...baseLink(), internet: "down", kashi: "down", kashiError: "can't connect to the server", live: false };
      await flushPromises();
      expect(text("sync-word")).toBe("No internet");
    } finally {
      vi.useRealTimers();
    }
  });
});
