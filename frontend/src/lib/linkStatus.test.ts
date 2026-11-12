import { describe, expect, it } from "vitest";
import { ACTIVITY_HOLD_MS, agoText, describeLink, durationText, secondsUntil, shouldHoldBusy, type LinkInfo, type QueueInfo } from "./linkStatus";

const NOW = Date.parse("2026-11-11T12:00:00Z");
const ago = (s: number) => new Date(NOW - s * 1000).toISOString();
const ahead = (s: number) => new Date(NOW + s * 1000).toISOString();

const link = (over: Partial<LinkInfo> = {}): LinkInfo => ({
  internet: "up", kashi: "up", kashiError: "", kashiLastOk: ago(3), kashiDownForSeconds: 0, updating: false, updatingForSeconds: 0,
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
    expect(s).toMatchObject({ tone: "ok", word: "Synced", sub: "", icon: "mdi-cloud-check-variant" });
    expect(s.spinner.on).toBe(false);
    expect(s.internetTone).toBe("ok");
    expect(s.liveTone).toBe("ok");
    expect(s.liveIcon).toBe("mdi-lan-connect");
  });

  it("not set up", () => {
    expect(describeLink(link(), queue({ configured: false }), idle, NOW)).toMatchObject({ tone: "off", word: "Off" });
    expect(describeLink(link(), null, idle, NOW)).toMatchObject({ tone: "off", word: "Off" });
  });

  it("no internet and central silent: says no internet, and what is waiting", () => {
    const s = d({ internet: "down", kashi: "down", kashiError: "can't find the server", kashiDownForSeconds: 70, live: false }, { pending: 3 });
    expect(s).toMatchObject({ tone: "bad", word: "No internet", sub: "3 waiting", icon: "mdi-wifi-off" });
    expect(s.spinner.on).toBe(false);
    expect(s.internetIcon).toBe("mdi-wifi-off");
    expect(s.internetTone).toBe("bad");
  });

  it("internet fine but central silent: names central, not the internet", () => {
    const s = d({ internet: "up", kashi: "down", kashiError: "the server took too long to answer", live: false });
    expect(s).toMatchObject({ tone: "bad", word: "No central" });
    expect(s.title).toContain("the server took too long to answer");
    expect(s.internetTone).toBe("ok");
  });

  it("the reported case: synced, but the live link is down", () => {
    const s = d({ live: false, liveError: "read: connection reset", liveRetryAt: ahead(8) });
    expect(s).toMatchObject({ tone: "attention", word: "Not live", icon: "mdi-lan-disconnect" });
    expect(s.liveTone).toBe("attention");
    expect(s.title).toContain("central shows this till as offline");
  });

  it("a till that has not yet tried anything is Connecting, not red", () => {
    const s = d({ internet: "unknown", kashi: "unknown", live: false, liveEverUp: false });
    expect(s).toMatchObject({ tone: "busy", word: "Connecting" });
    expect(s.spinner.on).toBe(true);
  });

  it("pulling shows only the entity on the rail, the full line in the title, no fake percentage", () => {
    const s = d({ phase: "pulling", label: "Products", done: 2, total: 21, detail: "500 changes" });
    expect(s).toMatchObject({ tone: "busy", word: "Updating", sub: "Products", title: "Products 3/21 · 500 changes" });
    expect(s.spinner).toEqual({ on: true, pct: null });
  });

  it("the reported case: a long clients catch-up never shows a climbing count on the rail", () => {
    for (const n of [100, 200, 300, 399, 400, 500, 12000]) {
      const s = d({ phase: "pulling", label: "Clients", done: 4, total: 21, detail: `${n} changes` });
      expect(s.sub).toBe("Clients");
      expect(s.sub).not.toMatch(/\d/);
      expect(s.spinner.pct).toBeNull();
      expect(s.rows.find(r => r.key === "now")!.value).toContain(`${n} changes`);
    }
  });

  it("a single-entity pull has no meaningless 0/1", () => {
    const s = d({ phase: "pulling", label: "Stock", done: 0, total: 1 });
    expect(s.sub).toBe("Stock");
    expect(s.spinner).toEqual({ on: true, pct: null });
  });

  it("sending shows how many records are left", () => {
    const s = d({ phase: "pushing", label: "Sending to central", done: 3, total: 12 }, { pending: 9 });
    expect(s).toMatchObject({ tone: "busy", word: "Sending", sub: "3 of 12", title: "Sending to central 3/12", icon: "mdi-cloud-upload" });
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
    expect(d({ phase: "pushing", label: "x", done: 50, total: 10 }).spinner.pct).toBe(100);
    expect(d({ phase: "pushing", label: "x", done: -3, total: 10 }).spinner.pct).toBe(0);
  });
});

describe("the detail rows", () => {
  const row = (s: ReturnType<typeof d>, key: string) => s.rows.find((r) => r.key === key)!;

  it("a healthy till reads healthy, with when things last happened", () => {
    const s = d();
    expect(row(s, "internet")).toMatchObject({ state: "ok", value: "Connected" });
    expect(row(s, "kashi")).toMatchObject({ state: "ok", value: "Reachable", detail: "Last answer just now" });
    expect(row(s, "live")).toMatchObject({ state: "ok", value: "Connected" });
    expect(row(s, "live").detail).toBe("Connected 10 min ago, heard from central 10 s ago");
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
    expect(r.detail).toContain("Central shows this till as offline");
  });

  it("a retry that is due says now", () => {
    expect(row(d({ live: false, liveRetryAt: ago(1) }), "live").detail).toContain("Trying again now");
  });

  it("Central down shows how long and why; internet down tells what to check", () => {
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
describe("kashi being upgraded", () => {
  const upd = { updating: true, updatingForSeconds: 42, live: false };
  it("says Upgrading, calm and spinning, not a fault", () => {
    const s = d(upd);
    expect(s).toMatchObject({ word: "Upgrading", tone: "busy", icon: "mdi-update", spinner: { on: true, pct: null } });
    expect(s.title).toMatch(/upgraded/);
    expect(s.title).toMatch(/Nothing is lost/);
  });
  it("is not the wording of not connected, not live or no kashi", () => {
    const s = d(upd);
    expect(s.word).not.toBe("Not live");
    expect(s.word).not.toBe("No kashi");
    expect(s.rows.find(r => r.key === "live")).toMatchObject({ state: "warn", value: "Waiting for the upgrade" });
    expect(s.rows.find(r => r.key === "live")!.value).not.toBe("Not connected");
    expect(s.rows.find(r => r.key === "kashi")).toMatchObject({ state: "warn", value: "Being upgraded for 42 s" });
  });
  it("a plain 'not connected' still reads Not live (the two stay apart)", () => {
    const s = d({ live: false });
    expect(s.word).toBe("Not live");
    expect(s.rows.find(r => r.key === "live")!.value).toBe("Not connected");
  });
  it("shows what is waiting to be sent", () => {
    expect(d(upd, { pending: 3 }).sub).toBe("3 waiting");
    expect(d(upd).sub).toBe("central is updating");
  });
  it("no internet still outranks it, and so does not being set up", () => {
    expect(d({ ...upd, internet: "down", kashi: "down" }).word).toBe("No internet");
    expect(describeLink(link(upd), queue({ configured: false }), idle, NOW).word).toBe("Off");
  });
  it("beats a full sync and a pull in progress", () => {
    expect(d(upd, {}, { running: true, progress: "x" }).word).toBe("Upgrading");
    expect(d({ ...upd, phase: "pulling", label: "Products" }).word).toBe("Upgrading");
  });
  it("when the live link is already back it still reads as upgrading until the flag clears", () => {
    expect(d({ updating: true, live: true }).rows.find(r => r.key === "live")!.value).toBe("Connected");
  });
  it("a missing time reads as just upgrading", () => {
    expect(d({ updating: true, updatingForSeconds: 0, live: false }).rows.find(r => r.key === "kashi")!.value).toBe("Being upgraded");
  });
});

describe("every combination", () => {
  it("is consistent", () => {
    const states = ["up", "down", "unknown"];
    const phases = ["idle", "pulling", "pushing"];
    for (const internet of states)
      for (const central of states)
        for (const live of [true, false])
          for (const phase of phases)
            for (const updating of [false, true])
            for (const failed of [0, 2])
              for (const pending of [0, 3])
                for (const running of [false, true]) {
                  const s = d({ internet, kashi: central, live, updating, phase, label: phase === "idle" ? "" : "X", done: 1, total: 4, liveEverUp: true },
                    { failed, pending }, { running, progress: running ? "p" : "" });
                  const ctx = JSON.stringify({ internet, kashi: central, live, updating, phase, failed, pending, running });
                  expect(s.word, ctx).toBeTruthy();
                  expect(["ok", "busy", "attention", "bad", "off"], ctx).toContain(s.tone);
                  if (central === "down") expect(s.tone, ctx).toBe("bad");
                  if (updating && central !== "down") expect(s.word, ctx).toBe("Upgrading");
                  if (s.word === "Upgrading") expect(s.tone, ctx).toBe("busy");
                  if (s.tone === "ok") {
                    expect(central, ctx).not.toBe("down");
                    expect(updating, ctx).toBe(false);
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

describe("shouldHoldBusy (no Synced flicker between passes)", () => {
  const calm = () => d();
  it("holds the calm Synced state while work was moving a moment ago", () => {
    expect(shouldHoldBusy(calm(), 1000, 1000 + ACTIVITY_HOLD_MS)).toBe(true);
    expect(shouldHoldBusy(calm(), 1000 + ACTIVITY_HOLD_MS, 1000 + ACTIVITY_HOLD_MS)).toBe(false);
    expect(shouldHoldBusy(calm(), 5000, 0)).toBe(false);
  });
  it("never holds back work or a problem", () => {
    const hold = Number.MAX_SAFE_INTEGER;
    expect(shouldHoldBusy(d({ phase: "pushing", label: "x", total: 2 }), 0, hold)).toBe(false);
    expect(shouldHoldBusy(d({}, { pending: 1 }), 0, hold)).toBe(false);
    expect(shouldHoldBusy(d({}, { failed: 1 }), 0, hold)).toBe(false);
    expect(shouldHoldBusy(d({ live: false }), 0, hold)).toBe(false);
    expect(shouldHoldBusy(d({ kashi: "down", internet: "down" }), 0, hold)).toBe(false);
    expect(shouldHoldBusy(describeLink(link(), queue({ configured: false }), idle, NOW), 0, hold)).toBe(false);
  });
});

describe("the screen says central, never kashi", () => {
  it("every word the dialog shows for a healthy, a silent and a fresh till", () => {
    const texts = (l: Partial<LinkInfo>) => {
      const all = [d(l), d(l, { failed: 2 }), d(l, { pending: 3 }), d(l, {}, { running: true, progress: "Products 3/21" })];
      return all.map(s => [s.word, s.sub, s.title, ...s.rows.flatMap(r => [r.label, r.value, r.detail])].join(" | ")).join(" || ");
    };
    for (const l of [{}, { kashi: "down" as const, kashiError: "no" }, { kashi: "unknown" as const, live: false, liveEverUp: false },
      { live: false, kashiLastOk: "" }, { updating: true, updatingForSeconds: 30, live: false }, { internet: "down", kashi: "down" }, { lastPullError: "x" }]) {
      expect(texts(l as Partial<LinkInfo>), JSON.stringify(l)).not.toMatch(/kashi/i);
    }
    expect(texts({})).toMatch(/central server/);
  });
});
