// Inside the separate screen's copy: records this page with rrweb and streams
// it on the separate screen's own socket, always with control on. The same
// recorder as the mirror's (liveView.ts) with the live_virtual:* events.
// It talks to the till's window directly for the frames: LiveVirtualSend is
// not a call the copy may make through the bridge.
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { createLiveView } from "./liveView";

export function startVirtualRecorder(parentGo: any = (window.parent as any).go) {
  const live = createLiveView({
    loadRrweb: async () => {
      const rrweb = await import("rrweb");
      return { record: rrweb.record } as any;
    },
    send: (batch) => parentGo.main.App.LiveVirtualSend(batch),
  });
  EventsOn("live_virtual:snapshot", () => live.snapshot());
  EventsOn("live_virtual:control", (raw: string) => live.control(raw));
  void live.start(true);
  return live;
}
