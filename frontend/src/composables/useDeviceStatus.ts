import { ref } from "vue";
import { GetDeviceStatus } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import type { main } from "../../wailsjs/go/models";

/**
 * Shared, module-cached status of the scanner, receipt printer and keyboard
 * for the nav rail. Each is "ok", "off" (gone) or "unknown" (no printer
 * chosen, or the scanner/keyboard hasn't been used on this till yet).
 *
 * `device_status_changed` fires from the Go side whenever one changes, so
 * the icons follow a plug or unplug without polling.
 */
const status = ref<main.DeviceStatus | null>(null);
let loaded = false;

EventsOn("device_status_changed", (next: main.DeviceStatus) => {
  status.value = next;
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
  return { status };
}
