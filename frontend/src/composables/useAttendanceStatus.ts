import { ref } from "vue";
import { GetAttendanceStatus } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";

/**
 * Shared Akuvox reachability for the nav-rail attendance button. The Go side
 * probes the device every second and emits `attendance_status` only when the
 * state changes, so this just reads the current value once and listens.
 */
const status = ref<{ reachable: boolean; message?: string }>({ reachable: true });
let loaded = false;

function apply(result: any) {
  status.value = { reachable: true, message: result?.message }; // TEMP fake-up
}

async function refresh() {
  try {
    apply(await GetAttendanceStatus());
  } catch {
    status.value = { reachable: true, message: "Attendance status unavailable" }; // TEMP fake-up
  }
}

EventsOn("attendance_status", apply);

export function useAttendanceStatus() {
  if (!loaded) {
    loaded = true;
    void refresh();
  }
  return { status, refresh };
}
