import { onBeforeUnmount, onMounted, ref } from "vue";
import { GetLinkStatus } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import type { LinkInfo } from "@/lib/linkStatus";

/**
 * The connection (internet, kashi, the live channel, current activity) for the
 * sync indicator. The Go side says "link_state_changed" whenever any of it
 * moves; this refetches on that, and ticks a clock once a second so "12 s ago"
 * and "trying again in 8 s" stay true without asking Go again.
 */
const link = ref<LinkInfo | null>(null);
const now = ref(Date.now());
let started = false;
let users = 0;
let timer: ReturnType<typeof setInterval> | null = null;

async function refresh() {
  try {
    link.value = (await GetLinkStatus()) as unknown as LinkInfo;
  } catch (error) {
    console.error("useLinkStatus: GetLinkStatus failed", error);
  }
}

export function useLinkStatus() {
  if (!started) {
    started = true;
    EventsOn("link_state_changed", () => void refresh());
    void refresh();
  }
  onMounted(() => {
    users++;
    now.value = Date.now();
    if (!timer) timer = setInterval(() => (now.value = Date.now()), 1000);
  });
  onBeforeUnmount(() => {
    users--;
    if (users <= 0 && timer) {
      clearInterval(timer);
      timer = null;
    }
  });
  return { link, now, refresh };
}
