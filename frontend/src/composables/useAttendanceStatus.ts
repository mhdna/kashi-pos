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
  status.value = { reachable: Boolean(result?.reachable), message: result?.message };
}

async function refresh() {
  try {
    apply(await GetAttendanceStatus());
  } catch {
    status.value = { reachable: false, message: "Attendance status unavailable" };
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
