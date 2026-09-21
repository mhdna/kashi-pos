import { ref } from "vue";
import { SyncAttendance } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";

/**
 * Shared, module-cached Akuvox reachability for the nav-rail attendance
 * button. Polls the same `SyncAttendance` call the attendance page uses,
 * scoped to today, purely to read back `reachable`.
 */
const status = ref<{ reachable: boolean; message?: string }>({ reachable: true });
let loaded = false;

function todayIso() {
  return new Date().toISOString().slice(0, 10);
}

async function refresh() {
  try {
    const result = await SyncAttendance(todayIso());
    status.value = { reachable: Boolean(result?.reachable), message: result?.message };
  } catch {
    status.value = { reachable: false, message: "Attendance sync failed" };
  }
}

EventsOn("settings_changed", () => {
  void refresh();
});

export function useAttendanceStatus() {
  if (!loaded) {
    loaded = true;
    void refresh();
    setInterval(refresh, 60_000);
  }
  return { status, refresh };
}
