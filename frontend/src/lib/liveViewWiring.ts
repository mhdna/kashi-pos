// Connects the live view to Go: the events Go emits and the binding that
// carries frames out. Called once from App.vue; does nothing until Go says
// "live_view:start" (kashi has the feature on and someone is watching).
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { LiveViewSend, LiveViewStatus } from "../../wailsjs/go/main/App";
import { createLiveView } from "./liveView";

export function startLiveViewWiring() {
  const live = createLiveView({
    loadRrweb: async () => {
      const rrweb = await import("rrweb");
      return { record: rrweb.record } as any;
    },
    send: (batch) => LiveViewSend(batch),
  });
  EventsOn("live_view:start", (info?: { control?: boolean }) => void live.start(info?.control === true));
  EventsOn("live_view:stop", () => live.stop());
  EventsOn("live_view:snapshot", () => live.snapshot());
  EventsOn("live_view:control", (raw: string) => live.control(raw));
  EventsOn("live_view:mode", (info?: { control?: boolean }) => live.mode(info?.control === true));
  // A reloaded page picks up a session that is already running.
  void LiveViewStatus()
    .then((s) => {
      if (s?.active) void live.start(s.control);
    })
    .catch(() => undefined);
  return live;
}
