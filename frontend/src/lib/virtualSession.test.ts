// @vitest-environment happy-dom
import { beforeEach, describe, expect, it, vi } from "vitest";
import { flushPromises } from "@vue/test-utils";

const handlers = new Map<string, (...a: any[]) => void>();
vi.mock("../../wailsjs/runtime/runtime", () => ({
  EventsOn: (name: string, fn: (...a: any[]) => void) => handlers.set(name, fn),
}));
let status = { active: false, control: false };
vi.mock("../../wailsjs/go/main/App", () => ({ LiveVirtualStatus: async () => status }));

const { createVirtualSession, installFocusGuard, startVirtualSessionWiring, VIRTUAL_SRC } = await import("./virtualSession");

beforeEach(() => {
  document.body.innerHTML = "";
  handlers.clear();
  status = { active: false, control: false };
});

describe("the hidden frame", () => {
  it("opens one off-screen copy of the app at the window's size, and closes it", () => {
    const s = createVirtualSession();
    s.open();
    s.open(); // a second open is a no-op: at most one hidden screen
    const frames = document.querySelectorAll("iframe[data-virtual-screen]");
    expect(frames).toHaveLength(1);
    const f = frames[0] as HTMLIFrameElement;
    expect(f.getAttribute("src")).toBe(VIRTUAL_SRC);
    expect(VIRTUAL_SRC).toContain("virtual=1");
    expect(f.getAttribute("tabindex")).toBe("-1");
    expect(f.style.left).toBe("-10000px");
    expect(f.style.width).toBe(`${window.innerWidth}px`);
    expect(f.style.display).not.toBe("none");
    expect(s.isOpen()).toBe(true);
    s.close();
    expect(document.querySelectorAll("iframe")).toHaveLength(0);
    expect(s.isOpen()).toBe(false);
    s.close(); // closing twice is harmless
  });

  it("drops the copy's Go listeners before removing it", () => {
    const s = createVirtualSession();
    s.open();
    const f = document.querySelector("iframe") as HTMLIFrameElement;
    const teardown = vi.fn();
    Object.defineProperty(f, "contentWindow", { value: { __virtualTeardown: teardown, addEventListener() {}, removeEventListener() {} } });
    s.close();
    expect(teardown).toHaveBeenCalledTimes(1);
  });

  it("can be reopened after a close", () => {
    const s = createVirtualSession();
    s.open();
    s.close();
    s.open();
    expect(document.querySelectorAll("iframe")).toHaveLength(1);
  });
});

describe("focus guard", () => {
  it("hands focus back to what the cashier had when it moves into the frame", () => {
    document.body.innerHTML = '<input id="barcode">';
    const input = document.getElementById("barcode") as HTMLInputElement;
    const s = createVirtualSession();
    s.open();
    const frame = document.querySelector("iframe") as HTMLIFrameElement;
    input.focus();
    expect(document.activeElement).toBe(input);

    // Code inside the copy calls .focus(): the parent's focus lands on the frame.
    frame.focus();
    window.dispatchEvent(new Event("blur"));
    expect(document.activeElement).toBe(input);
    s.close();
  });

  it("does nothing while focus is not in the frame, and stops after close", () => {
    document.body.innerHTML = '<input id="a"><input id="b">';
    const a = document.getElementById("a") as HTMLInputElement;
    const b = document.getElementById("b") as HTMLInputElement;
    const frame = document.createElement("iframe");
    document.body.append(frame);
    const remove = installFocusGuard(document, window, frame);
    a.focus();
    b.focus();
    window.dispatchEvent(new Event("blur"));
    expect(document.activeElement).toBe(b);

    remove();
    frame.focus();
    window.dispatchEvent(new Event("blur"));
    expect(document.activeElement).toBe(frame);
  });
});

describe("wiring", () => {
  it("listens for start and stop, and does not open a frame before Go says so", async () => {
    startVirtualSessionWiring();
    await flushPromises();
    expect([...handlers.keys()].sort()).toEqual(["live_virtual:start", "live_virtual:stop"]);
    expect(document.querySelectorAll("iframe")).toHaveLength(0);
    handlers.get("live_virtual:start")!();
    expect(document.querySelectorAll("iframe")).toHaveLength(1);
    handlers.get("live_virtual:stop")!();
    expect(document.querySelectorAll("iframe")).toHaveLength(0);
  });

  it("a reloaded window picks up a session that is already open", async () => {
    status = { active: true, control: true };
    startVirtualSessionWiring();
    await flushPromises();
    expect(document.querySelectorAll("iframe")).toHaveLength(1);
  });
});
