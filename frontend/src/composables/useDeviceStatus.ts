import { computed, ref } from "vue";
import { GetDeviceStatus } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import type { main } from "../../wailsjs/go/models";

/**
 * Shared, module-cached status of the scanner, receipt printer, keyboard and
 * mouse for the nav rail and the Devices dialog. Each is "ok", "off" (gone)
 * or "unknown" (no printer chosen, or the scanner hasn't scanned on this till
 * yet).
 *
 * `device_status_changed` fires from the Go side whenever one changes, so
 * the icons follow a plug or unplug without polling.
 */
const status = ref<main.DeviceStatus | null>(null);
let loaded = false;

EventsOn("device_status_changed", (next: main.DeviceStatus) => {
  status.value = next;
});

export type DeviceState = "ok" | "off" | "unknown";
export type DeviceKind = "scanner" | "printer" | "keyboard" | "mouse";

const connectedWords: Record<DeviceState, string> = {
  ok: "Connected",
  off: "Not connected",
  unknown: "Not known yet",
};

/** The four devices in rail order, with what to say about each. */
const devices = computed(() => {
  const st = status.value;
  const state = (value?: string) => (value ?? "unknown") as DeviceState;
  const list: { kind: DeviceKind; icon: string; label: string; state: DeviceState; name: string; words: Record<DeviceState, string> }[] = [
    {
      kind: "scanner",
      icon: "mdi-barcode-scanner-gun",
      label: "Scanner",
      state: state(st?.scanner),
      name: st?.scannerName ?? "",
      words: { ...connectedWords, unknown: "Scan a barcode once so the till knows it" },
    },
    {
      kind: "printer",
      icon: "mdi-printer",
      label: "Printer",
      state: state(st?.printer),
      name: st?.printerName ?? "",
      words: { ok: "Ready", off: "Offline", unknown: "None chosen in Settings" },
    },
    { kind: "keyboard", icon: "mdi-keyboard", label: "Keyboard", state: state(st?.keyboard), name: st?.keyboardName ?? "", words: connectedWords },
    { kind: "mouse", icon: "mdi-mouse", label: "Mouse", state: state(st?.mouse), name: st?.mouseName ?? "", words: connectedWords },
  ];
  return list.map((device) => ({
    ...device,
    word: device.words[device.state],
    title: `${device.label}: ${device.words[device.state]}`,
  }));
});

export function useDeviceStatus() {
  if (!loaded) {
    loaded = true;
    GetDeviceStatus()
      .then((next) => {
        status.value = next;
      })
      .catch((error) => console.error("useDeviceStatus: GetDeviceStatus failed", error));
  }
  return { status, devices };
}
