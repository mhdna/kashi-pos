import { ref } from "vue";
import {
  AdjustVisitorCountToday,
  GetVisitorCounterEnabled,
  GetVisitorCountToday,
} from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";

/**
 * Shared, module-cached state for the nav-drawer people-visits counter.
 * `enabled` mirrors the `visitor_counter` feature flag (toggled from
 * Settings); `count` is today's running tally. Disabling the flag only
 * hides the buttons and the monthly-income chip - recorded counts stay in
 * the DB.
 *
 * `settings_changed` fires whenever a feature flag changes - including a
 * remote `update_settings` push from kashi - so a running till re-reads the
 * flag live instead of only on restart.
 */
const enabled = ref(false);
const count = ref(0);
let loaded = false;

async function refresh() {
  try {
    enabled.value = Boolean(await GetVisitorCounterEnabled());
  } catch (error) {
    console.error("useVisitorCounter: GetVisitorCounterEnabled failed", error);
  }
  try {
    count.value = Number(await GetVisitorCountToday()) || 0;
  } catch (error) {
    console.error("useVisitorCounter: GetVisitorCountToday failed", error);
  }
}

async function adjust(delta: number) {
  // Optimistic bump so the UI responds immediately; the server value
  // (clamped at zero) overwrites it once the call returns.
  const optimistic = Math.max(0, count.value + delta);
  count.value = optimistic;
  try {
    const next = Number(await AdjustVisitorCountToday(delta));
    count.value = Number.isFinite(next) ? next : optimistic;
  } catch (error) {
    console.error("useVisitorCounter: AdjustVisitorCountToday failed", error);
  }
}

EventsOn("settings_changed", () => {
  void refresh();
});

export function useVisitorCounter() {
  if (!loaded) {
    loaded = true;
    void refresh();
  }
  return { enabled, count, refresh, adjust };
}
