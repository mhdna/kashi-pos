// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { ref } from "vue";
import ui from "@/ui";

globalThis.visualViewport ??= { width: 1024, height: 768, offsetTop: 0, offsetLeft: 0, scale: 1, addEventListener() {}, removeEventListener() {} } as any;

const push = vi.fn();
const logout = vi.fn();
vi.mock("vue-router", () => ({ useRoute: () => ({ path: "/home" }), useRouter: () => ({ push }) }));
vi.mock("@/stores/auth", () => ({ useAuthStore: () => ({ isAdmin: true, logout }) }));
vi.mock("@/composables/usePageUnlocks", () => ({ usePageUnlocks: () => ({ pageUnlocks: ref({}) }) }));
vi.mock("@/composables/useVisitorCounter", () => ({
  useVisitorCounter: () => ({ enabled: ref(false), count: ref(0), adjust: vi.fn() }),
}));
vi.mock("@/composables/useDeviceStatus", () => ({ useDeviceStatus: () => ({ devices: ref([]) }) }));
vi.mock("@/composables/useAttendanceStatus", () => ({ useAttendanceStatus: () => ({ status: ref({ reachable: true }) }) }));
vi.mock("@/components/SyncIndicator.vue", () => ({ default: { template: "<div />" } }));
vi.mock("@/components/DevicesDialog.vue", () => ({ default: { template: "<div />" } }));
const Quit = vi.fn();
vi.mock("../../wailsjs/runtime/runtime", () => ({ Quit: (...a: unknown[]) => Quit(...a) }));
const RestartApp = vi.fn();
const RestartComputer = vi.fn();
const ShutDownComputer = vi.fn();
vi.mock("../../wailsjs/go/main/App", () => ({
  RestartApp: (...a: unknown[]) => RestartApp(...a),
  RestartComputer: (...a: unknown[]) => RestartComputer(...a),
  ShutDownComputer: (...a: unknown[]) => ShutDownComputer(...a),
}));

import NavDrawer from "./NavDrawer.vue";

let wrapper: VueWrapper<any> | null = null;
afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
  document.body.innerHTML = "";
  vi.clearAllMocks();
});

async function openQuitMenu() {
  wrapper = mount(NavDrawer, { global: { plugins: [ui] }, attachTo: document.body });
  await flushPromises();
  (document.body.querySelector(".options-item") as HTMLElement).click();
  await flushPromises();
}
const q = (sel: string) => document.body.querySelector(sel) as HTMLElement | null;
const click = async (sel: string) => {
  q(sel)!.click();
  await flushPromises();
};

describe("Quit menu", () => {
  it("lists the options in three colour-coded groups, with no captions", async () => {
    await openQuitMenu();
    expect(document.body.querySelector(".options-group-title")).toBeNull();
    const tones = [...document.body.querySelectorAll(".options-group")].map((e) => e.className.match(/options-group--(\w+)/)?.[1]);
    expect(tones).toEqual(["shift", "app", "computer"]);
    expect(document.body.textContent).not.toContain("This app");
    expect(document.body.textContent).not.toContain("This computer");
    const labels = [...document.body.querySelectorAll(".option-card")].map((e) => e.textContent?.trim());
    expect(labels).toEqual(["Close Shift", "Logout", "Reload App", "Close App", "Restart PC", "Shut Down PC"]);
  });

  it("Reload App asks first, then reloads the app, and not the computer", async () => {
    await openQuitMenu();
    await click('[data-test="quit-reload-app"]');
    expect(q('[data-test="reload-app-dialog"]')!.textContent).toContain("Reload the app?");
    expect(RestartApp).not.toHaveBeenCalled();
    await click('[data-test="reload-app-confirm"]');
    expect(RestartApp).toHaveBeenCalledTimes(1);
    expect(RestartComputer).not.toHaveBeenCalled();
    expect(ShutDownComputer).not.toHaveBeenCalled();
  });

  it("cancelling the reload does nothing", async () => {
    await openQuitMenu();
    await click('[data-test="quit-reload-app"]');
    const cancel = [...q('[data-test="reload-app-dialog"]')!.querySelectorAll("button")].find((b) => b.textContent?.trim() === "Cancel")!;
    cancel.click();
    await flushPromises();
    expect(RestartApp).not.toHaveBeenCalled();
  });

  it("shows why when the app can't reload (a development build, say)", async () => {
    RestartApp.mockRejectedValueOnce("not from a development build");
    await openQuitMenu();
    await click('[data-test="quit-reload-app"]');
    await click('[data-test="reload-app-confirm"]');
    expect(q('[data-test="reload-app-error"]')!.textContent).toContain("not from a development build");
  });

  it("Restart PC is the computer restart, behind its own confirmation", async () => {
    await openQuitMenu();
    await click('[data-test="quit-restart-pc"]');
    expect(document.body.textContent).toContain("Restart this computer now?");
    expect(RestartComputer).not.toHaveBeenCalled();
  });

  it("Close App still asks before quitting", async () => {
    await openQuitMenu();
    await click('[data-test="quit-close-app"]');
    expect(document.body.textContent).toContain("Close the app and go to Windows?");
    expect(Quit).not.toHaveBeenCalled();
  });

  it("Close Shift and Logout still do what they did", async () => {
    await openQuitMenu();
    await click('[data-test="quit-close-shift"]');
    expect(push).toHaveBeenCalledWith("/close-shift");
    await openQuitMenu();
    await click('[data-test="quit-logout"]');
    expect(logout).toHaveBeenCalled();
    expect(push).toHaveBeenCalledWith("/login");
  });
});
