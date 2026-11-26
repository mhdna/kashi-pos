// @vitest-environment happy-dom
import { readFileSync } from "node:fs";
import { describe, expect, it, vi } from "vitest";

vi.mock("@/virtualBridge", () => ({ isVirtual: true }));
vi.mock("../../wailsjs/go/main/App", () => ({ GetSoundsEnabled: async () => true }));
vi.mock("../../wailsjs/runtime/runtime", () => ({ EventsOn: () => {} }));

describe("the separate screen is silent", () => {
  it("plays no sound on the till", async () => {
    const audio = vi.fn();
    vi.stubGlobal("Audio", audio);
    const { playSuccessSound, playClickSound } = await import("./sound");
    await new Promise((r) => setTimeout(r, 0));
    playSuccessSound();
    playClickSound();
    expect(audio).not.toHaveBeenCalled();
    vi.unstubAllGlobals();
  });
});

// The pieces below only the cashier's window may run. They are wired in Vue
// files that need the whole app to mount, so this pins the guards in the source:
// removing one fails here.
describe("cashier-only pieces are gated on isVirtual", () => {
  const read = (p: string) => readFileSync(new URL(p, import.meta.url), "utf8");

  it("App.vue shows no toast, banner or clock warning in the copy, and starts the right recorder", () => {
    const app = read("../App.vue");
    expect(app).toMatch(/<template v-if="!isVirtual">[\s\S]*<AttendanceToast \/>[\s\S]*<WhatsAppToast \/>[\s\S]*<CashMachineToast \/>[\s\S]*<AppUpdateBanner \/>[\s\S]*<ClockWarning \/>[\s\S]*<\/template>/);
    expect(app).toMatch(/if \(isVirtual\) \{\s*startVirtualRecorder\(\);\s*\} else \{\s*startLiveViewWiring\(\);\s*startVirtualSessionWiring\(\);/);
  });

  it("home.vue neither reports the live cart nor applies central's overrides in the copy", () => {
    const home = read("../pages/home.vue");
    expect(home).toMatch(/function sendLiveCart\(\) \{[^}]*isVirtual\) return;/);
    expect(home).toMatch(/async function syncLiveOverrides\(announce: boolean\) \{\s*if \(isVirtual\) return;/);
  });
});
