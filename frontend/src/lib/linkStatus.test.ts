import { describe, expect, it } from "vitest";
import { agoText, describeLink, durationText, secondsUntil, type LinkInfo, type QueueInfo } from "./linkStatus";

const NOW = Date.parse("2026-11-11T12:00:00Z");
const ago = (s: number) => new Date(NOW - s * 1000).toISOString();
const ahead = (s: number) => new Date(NOW + s * 1000).toISOString();

const link = (over: Partial<LinkInfo> = {}): LinkInfo => ({
  internet: "up", kashi: "up", kashiError: "", kashiLastOk: ago(3), kashiDownForSeconds: 0,
  live: true, liveSince: ago(600), liveLastBeat: ago(10), liveError: "", liveRetryAt: "", liveEverUp: true,
  phase: "idle", label: "", detail: "", done: 0, total: 0, lastPullAt: ago(20), lastPushAt: ago(40), lastPullError: "",
  ...over,
});
const queue = (over: Partial<QueueInfo> = {}): QueueInfo => ({ configured: true, pending: 0, failed: 0, confirmedElsewhere: 0, ...over });
const idle = { running: false, progress: "" };
const d = (l: Partial<LinkInfo> = {}, q: Partial<QueueInfo> = {}, r = idle) => describeLink(link(l), queue(q), r, NOW);

describe("agoText", () => {
  it("reads like a person", () => {
    expect(agoText("", NOW)).toBe("never");
    expect(agoText("garbage", NOW)).toBe("never");
    expect(agoText(ago(1), NOW)).toBe("just now");
    expect(agoText(ago(12), NOW)).toBe("12 s ago");
    expect(agoText(ago(59), NOW)).toBe("59 s ago");
    expect(agoText(ago(60), NOW)).toBe("1 min ago");
    expect(agoText(ago(3599), NOW)).toBe("59 min ago");
    expect(agoText(ago(3600), NOW)).toBe("1 h ago");
    expect(agoText(ago(86400 * 2), NOW)).toBe("2 d ago");
  });
  it("never goes negative when the clocks differ a little", () => {
    expect(agoText(ahead(30), NOW)).toBe("just now");
  });
});

describe("durationText and secondsUntil", () => {
  it("formats", () => {
    expect(durationText(0)).toBe("0 s");
    expect(durationText(45)).toBe("45 s");
    expect(durationText(180)).toBe("3 min");
    expect(durationText(7300)).toBe("2 h");
    expect(durationText(-5)).toBe("0 s");
  });
  it("counts down to a time and stops at zero", () => {
    expect(secondsUntil(ahead(8), NOW)).toBe(8);
    expect(secondsUntil(ago(8), NOW)).toBe(0);
    expect(secondsUntil("", NOW)).toBeNull();
    expect(secondsUntil("nope", NOW)).toBeNull();
  });
});

describe("the headline", () => {
  it("everything fine: Synced, live, no spinner", () => {
    const s = d();
    expect(s).toMatchObject({ tone: "ok", word: "Synced", sub: "Live", icon: "mdi-cloud-check-variant" });
    expect(s.spinner.on).toBe(false);
    expect(s.internetTone).toBe("ok");
    expect(s.liveTone).toBe("ok");
    expect(s.liveIcon).toBe("mdi-lan-connect");
  });

  it("not set up", () => {
    expect(describeLink(link(), queue({ configured: false }), idle, NOW)).toMatchObject({ tone: "off", word: "Off" });
    expect(describeLink(link(), null, idle, NOW)).toMatchObject({ tone: "off", word: "Off" });
  });

  it("no internet and kashi silent: says no internet, and what is waiting", () => {
    const s = d({ internet: "down", kashi: "down", kashiError: "can't find the server", kashiDownForSeconds: 70, live: false }, { pending: 3 });
    expect(s).toMatchObject({ tone: "bad", word: "No internet", sub: "3 waiting", icon: "mdi-wifi-off" });
    expect(s.spinner.on).toBe(false);
    expect(s.internetIcon).toBe("mdi-wifi-off");
    expect(s.internetTone).toBe("bad");
  });

  it("internet fine but kashi silent: names kashi, not the internet", () => {
    const s = d({ internet: "up", kashi: "down", kashiError: "the server took too long to answer", live: false });
    expect(s).toMatchObject({ tone: "bad", word: "No kashi" });
    expect(s.title).toContain("the server took too long to answer");
    expect(s.internetTone).toBe("ok");
  });

  it("the reported case: synced, but the live link is down", () => {
    const s = d({ live: false, liveError: "read: connection reset", liveRetryAt: ahead(8) });
    expect(s).toMatchObject({ tone: "attention", word: "Not live", icon: "mdi-lan-disconnect" });
    expect(s.liveTone).toBe("attention");
    expect(s.title).toContain("kashi shows this till as offline");
  });

  it("a till that has not yet tried anything is Connecting, not red", () => {
    const s = d({ internet: "unknown", kashi: "unknown", live: false, liveEverUp: false });
    expect(s).toMatchObject({ tone: "busy", word: "Connecting" });
    expect(s.spinner.on).toBe(true);
  });

  it("pulling shows the entity, the step and a determinate spinner", () => {
    const s = d({ phase: "pulling", label: "Products", done: 2, total: 21, detail: "500 changes" });
    expect(s).toMatchObject({ tone: "busy", word: "Updating", sub: "Products 3/21 · 500 changes" });
    expect(s.spinner).toEqual({ on: true, pct: 10 });
  });

  it("a single-entity pull has no meaningless 0/1", () => {
    const s = d({ phase: "pulling", label: "Stock", done: 0, total: 1 });
    expect(s.sub).toBe("Stock");
    expect(s.spinner).toEqual({ on: true, pct: null });
  });

  it("sending shows how many records are left", () => {
    const s = d({ phase: "pushing", label: "Sending to kashi", done: 3, total: 12 }, { pending: 9 });
    expect(s).toMatchObject({ tone: "busy", word: "Sending", sub: "Sending to kashi 3/12", icon: "mdi-cloud-upload" });
    expect(s.spinner).toEqual({ on: true, pct: 25 });
  });

  it("a full sync wins over the quiet states and has no percentage", () => {
    const s = d({}, { pending: 4 }, { running: true, progress: "products: 40 / 90" });
    expect(s).toMatchObject({ tone: "busy", word: "Syncing", sub: "products: 40 / 90" });
    expect(s.spinner).toEqual({ on: true, pct: null });
  });

  it("failed records are amber, not red, and say they are safe", () => {
    const s = d({}, { failed: 2 });
    expect(s).toMatchObject({ tone: "attention", word: "Check" });
    expect(s.title).toContain("safe on this till");
  });

  it("queued records with nothing moving: Waiting, no spinner", () => {
    const s = d({}, { pending: 2, confirmedElsewhere: 1 });
    expect(s).toMatchObject({ tone: "busy", word: "Waiting", sub: "3 to send" });
    expect(s.spinner.on).toBe(false);
  });

  it("being offline outranks everything else", () => {
    const s = d({ kashi: "down", internet: "down", phase: "pulling", label: "Products", total: 5 }, { failed: 1, pending: 1 }, { running: true, progress: "x" });
    expect(s.word).toBe("No internet");
  });

  it("percent is clamped", () => {
    expect(d({ phase: "pulling", label: "x", done: 50, total: 10 }).spinner.pct).toBe(100);
    expect(d({ phase: "pulling", label: "x", done: -3, total: 10 }).spinner.pct).toBe(0);
  });
});

describe("the detail rows", () => {
  const row = (s: ReturnType<typeof d>, key: string) => s.rows.find((r) => r.key === key)!;

  it("a healthy till reads healthy, with when things last happened", () => {
    const s = d();
    expect(row(s, "internet")).toMatchObject({ state: "ok", value: "Connected" });
    expect(row(s, "kashi")).toMatchObject({ state: "ok", value: "Reachable", detail: "Last answer just now" });
    expect(row(s, "live")).toMatchObject({ state: "ok", value: "Connected" });
    expect(row(s, "live").detail).toBe("Connected 10 min ago, heard from kashi 10 s ago");
    expect(row(s, "now")).toMatchObject({ state: "idle", value: "Nothing to do" });
    expect(row(s, "received").value).toBe("20 s ago");
    expect(row(s, "sent").value).toBe("40 s ago");
    expect(row(s, "queue").value).toBe("Nothing");
  });

  it("the live row tells the reported case in words and when it retries", () => {
    const r = row(d({ live: false, liveError: "read: closed", liveRetryAt: ahead(8) }), "live");
    expect(r).toMatchObject({ state: "warn", value: "Not connected" });
    expect(r.detail).toContain("read: closed");
    expect(r.detail).toContain("Trying again in 8 s");
    expect(r.detail).toContain("kashi shows this till as offline");
  });

  it("a retry that is due says now", () => {
    expect(row(d({ live: false, liveRetryAt: ago(1) }), "live").detail).toContain("Trying again now");
  });

  it("kashi down shows how long and why; internet down tells what to check", () => {
    const s = d({ internet: "down", kashi: "down", kashiError: "can't connect to the server", kashiDownForSeconds: 200, live: false });
    expect(row(s, "kashi")).toMatchObject({ state: "bad", value: "Not answering for 3 min", detail: "can't connect to the server" });
    expect(row(s, "internet")).toMatchObject({ state: "bad", value: "No connection" });
  });

  it("unknown states read as checking, never as a fault", () => {
    const s = d({ internet: "unknown", kashi: "unknown", live: false, liveEverUp: false });
    expect(row(s, "internet")).toMatchObject({ state: "idle", value: "Checking" });
    expect(row(s, "kashi")).toMatchObject({ state: "idle", value: "Checking" });
    expect(row(s, "live")).toMatchObject({ state: "idle", value: "Connecting" });
  });

  it("a failed pull shows its error; the queue counts waiting and not accepted", () => {
    const s = d({ lastPullError: "fetch changes: boom" }, { pending: 2, failed: 1, confirmedElsewhere: 1 });
    expect(row(s, "received")).toMatchObject({ state: "warn", detail: "fetch changes: boom" });
    expect(row(s, "queue").value).toBe("3 waiting, 1 not accepted");
    expect(row(s, "queue").state).toBe("warn");
  });

  it("never reports a time before anything has happened", () => {
    const s = d({ lastPullAt: "", lastPushAt: "", kashiLastOk: "" });
    expect(row(s, "received").value).toBe("never");
    expect(row(s, "sent").value).toBe("never");
    expect(row(s, "kashi").detail).toBe("");
  });

  it("null inputs still describe something", () => {
    const s = describeLink(null, null, idle, NOW);
    expect(s.word).toBe("Off");
    expect(s.rows.length).toBeGreaterThan(0);
  });
});

// Every combination of the facts gives one headline, and the picture and the
// words never disagree about being offline.
describe("every combination", () => {
  it("is consistent", () => {
    const states = ["up", "down", "unknown"];
    const phases = ["idle", "pulling", "pushing"];
    for (const internet of states)
      for (const kashi of states)
        for (const live of [true, false])
          for (const phase of phases)
            for (const failed of [0, 2])
              for (const pending of [0, 3])
                for (const running of [false, true]) {
                  const s = d({ internet, kashi, live, phase, label: phase === "idle" ? "" : "X", done: 1, total: 4, liveEverUp: true },
                    { failed, pending }, { running, progress: running ? "p" : "" });
                  const ctx = JSON.stringify({ internet, kashi, live, phase, failed, pending, running });
                  expect(s.word, ctx).toBeTruthy();
                  expect(["ok", "busy", "attention", "bad", "off"], ctx).toContain(s.tone);
                  if (kashi === "down") expect(s.tone, ctx).toBe("bad");
                  if (s.tone === "ok") {
                    expect(kashi, ctx).not.toBe("down");
                    expect(live, ctx).toBe(true);
                    expect(failed + pending, ctx).toBe(0);
                    expect(phase, ctx).toBe("idle");
                    expect(running, ctx).toBe(false);
                    expect(s.spinner.on, ctx).toBe(false);
                  }
                  if (s.spinner.on) expect(s.tone, ctx).toBe("busy");
                  expect(s.internetIcon === "mdi-wifi-off", ctx).toBe(internet === "down");
                  expect(s.liveIcon === "mdi-lan-connect", ctx).toBe(live);
                }
  });
});
