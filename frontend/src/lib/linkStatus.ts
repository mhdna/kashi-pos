// What the nav-rail sync indicator and its detail dialog say, worked out from
// three things the till knows: the connection (internet, central, the live push
// channel, what it is doing right now), the push queue (pending / failed),
// and a "Sync everything" run. Kept as plain functions so every combination
// can be tested without mounting anything.

export type Tone = "ok" | "busy" | "attention" | "bad" | "off";
export type RowState = "ok" | "warn" | "bad" | "idle";

/** Mirrors Go's LinkStatus (sync_link_status.go). */
export interface LinkInfo {
  internet: string; // up | down | unknown
  kashi: string; // up | down | unknown
  kashiError: string;
  kashiLastOk: string;
  kashiDownForSeconds: number;
  live: boolean;
  liveSince: string;
  liveLastBeat: string;
  liveError: string;
  liveRetryAt: string;
  liveEverUp: boolean;
  phase: string; // idle | pulling | pushing
  label: string;
  detail: string;
  done: number;
  total: number;
  lastPullAt: string;
  lastPushAt: string;
  lastPullError: string;
}

/** The parts of Go's SyncStatusResult the indicator uses. */
export interface QueueInfo {
  configured: boolean;
  pending: number;
  failed: number;
  confirmedElsewhere: number;
}

export interface Row {
  key: "internet" | "kashi" | "live" | "now" | "received" | "sent" | "queue";
  label: string;
  state: RowState;
  value: string;
  detail: string;
}

export interface Summary {
  tone: Tone;
  icon: string;
  /** One or two words for the nav rail. */
  word: string;
  /** A line under the word while something moves ("Products 3/21"); else "". */
  sub: string;
  /** Spinner: on while work is moving; pct is null when the total is unknown. */
  spinner: { on: boolean; pct: number | null };
  /** The tooltip / first line of the dialog. */
  title: string;
  /** Small icons beside the word: internet and the live channel. */
  internetIcon: string;
  internetTone: Tone;
  liveIcon: string;
  liveTone: Tone;
  rows: Row[];
}

export interface Resync {
  running: boolean;
  progress: string;
}

const UNKNOWN_LINK: LinkInfo = {
  internet: "unknown", kashi: "unknown", kashiError: "", kashiLastOk: "", kashiDownForSeconds: 0,
  live: false, liveSince: "", liveLastBeat: "", liveError: "", liveRetryAt: "", liveEverUp: false,
  phase: "idle", label: "", detail: "", done: 0, total: 0, lastPullAt: "", lastPushAt: "", lastPullError: "",
};

/** "just now", "12 s ago", "3 min ago", "2 h ago", "4 d ago"; "never" when empty. */
export function agoText(iso: string, nowMs: number): string {
  if (!iso) return "never";
  const t = Date.parse(iso);
  if (Number.isNaN(t)) return "never";
  const s = Math.max(0, Math.round((nowMs - t) / 1000));
  if (s < 5) return "just now";
  if (s < 60) return `${s} s ago`;
  if (s < 3600) return `${Math.floor(s / 60)} min ago`;
  if (s < 86400) return `${Math.floor(s / 3600)} h ago`;
  return `${Math.floor(s / 86400)} d ago`;
}

/** "45 s", "3 min", "2 h": how long something has lasted. */
export function durationText(seconds: number): string {
  const s = Math.max(0, Math.round(seconds));
  if (s < 60) return `${s} s`;
  if (s < 3600) return `${Math.floor(s / 60)} min`;
  return `${Math.floor(s / 3600)} h`;
}

/** Seconds until an RFC 3339 time, never negative; null when empty. */
export function secondsUntil(iso: string, nowMs: number): number | null {
  if (!iso) return null;
  const t = Date.parse(iso);
  if (Number.isNaN(t)) return null;
  return Math.max(0, Math.round((t - nowMs) / 1000));
}

function workLine(link: LinkInfo): string {
  if (link.phase === "idle") return "";
  const parts = [link.label];
  if (link.total > 1) parts.push(`${Math.min(link.done + (link.phase === "pulling" ? 1 : 0), link.total)}/${link.total}`);
  else if (link.phase === "pushing" && link.total === 1) parts.push("1 record");
  return parts.filter(Boolean).join(" ") + (link.detail ? ` · ${link.detail}` : "");
}

function pct(link: LinkInfo): number | null {
  if (link.total <= 1) return null;
  return Math.max(0, Math.min(100, Math.round((link.done / link.total) * 100)));
}

/** How long the light keeps looking busy after the last bit of work. */
export const ACTIVITY_HOLD_MS = 2500;

/**
 * One checkout queues several records and the till sends them in separate
 * passes, then pulls the stock changes that come back; between passes it is
 * idle for a moment. Showing every gap flashes "Synced" between "Sending" and
 * "Updating". So the calm "Synced" state waits until nothing has moved for
 * ACTIVITY_HOLD_MS. Anything that is a problem, or work, shows at once.
 */
export function shouldHoldBusy(next: Summary, nowMs: number, holdUntilMs: number): boolean {
  return next.tone === "ok" && !next.spinner.on && nowMs < holdUntilMs;
}

export function describeLink(
  linkIn: LinkInfo | null | undefined,
  queue: QueueInfo | null | undefined,
  resync: Resync,
  nowMs: number,
): Summary {
  const link = linkIn ?? UNKNOWN_LINK;
  const configured = !!queue?.configured;
  const queued = (queue?.pending ?? 0) + (queue?.confirmedElsewhere ?? 0);
  const failed = queue?.failed ?? 0;

  // "down" is a fact only when something failed; "unknown" at startup must not
  // paint the till red before its first request.
  const internetDown = link.internet === "down";
  const kashiDown = link.kashi === "down";
  const liveDown = !link.live && link.liveEverUp ? true : !link.live && link.kashi === "up";

  const internetTone: Tone = link.internet === "up" ? "ok" : internetDown ? "bad" : "off";
  const internetIcon = internetDown ? "mdi-wifi-off" : "mdi-wifi";
  const liveTone: Tone = link.live ? "ok" : liveDown ? "attention" : "off";
  const liveIcon = link.live ? "mdi-lan-connect" : "mdi-lan-disconnect";

  const working = resync.running || link.phase !== "idle";
  const spinnerPct = resync.running ? null : pct(link);

  const base = { internetIcon, internetTone, liveIcon, liveTone };
  const rows = buildRows(link, queue ?? null, nowMs);

  const make = (s: Omit<Summary, "internetIcon" | "internetTone" | "liveIcon" | "liveTone" | "rows">): Summary =>
    ({ ...s, ...base, rows });

  if (!configured) {
    return make({ tone: "off", icon: "mdi-cloud-off-outline", word: "Off", sub: "", spinner: { on: false, pct: null }, title: "Sync is not set up" });
  }

  if (internetDown && kashiDown) {
    return make({
      tone: "bad", icon: "mdi-wifi-off", word: "No internet", sub: queued > 0 ? `${queued} waiting` : "",
      spinner: { on: false, pct: null },
      title: "No internet connection. Sales are kept on this till and go to central when it is back.",
    });
  }
  if (kashiDown) {
    return make({
      tone: "bad", icon: "mdi-cloud-off-outline", word: "No central", sub: queued > 0 ? `${queued} waiting` : "",
      spinner: { on: false, pct: null },
      title: `The internet is fine but central does not answer (${link.kashiError || "unreachable"}). Sales are kept on this till.`,
    });
  }

  if (resync.running) {
    return make({
      tone: "busy", icon: "mdi-sync", word: "Syncing", sub: resync.progress || "Full sync", spinner: { on: true, pct: null },
      title: resync.progress || "Full sync running",
    });
  }
  if (link.phase !== "idle") {
    const line = workLine(link);
    return make({
      tone: "busy", icon: link.phase === "pushing" ? "mdi-cloud-upload" : "mdi-cloud-download", word: link.phase === "pushing" ? "Sending" : "Updating",
      sub: line, spinner: { on: true, pct: spinnerPct }, title: line,
    });
  }

  if (failed > 0) {
    return make({
      tone: "attention", icon: "mdi-cloud-alert-outline", word: "Check", sub: "", spinner: { on: false, pct: null },
      // Amber, not red: the sales are safe on the till, central just hasn't taken them yet.
      title: `${failed} record(s) central didn't accept yet; they're safe on this till`,
    });
  }
  if (queued > 0) {
    return make({
      tone: "busy", icon: "mdi-cloud-upload", word: "Waiting", sub: `${queued} to send`, spinner: { on: false, pct: null },
      title: `${queued} record(s) not yet confirmed by central`,
    });
  }
  if (liveDown) {
    return make({
      tone: "attention", icon: "mdi-lan-disconnect", word: "Not live", sub: "", spinner: { on: false, pct: null },
      title: "Synced, but the live link to central is down: central shows this till as offline and changes reach it a little later.",
    });
  }
  if (link.kashi === "unknown" && !link.live) {
    return make({ tone: "busy", icon: "mdi-sync", word: "Connecting", sub: "", spinner: { on: true, pct: null }, title: "Connecting to central" });
  }
  return make({ tone: "ok", icon: "mdi-cloud-check-variant", word: "Synced", sub: "", spinner: { on: false, pct: null }, title: "Fully synced with central, live" });
}

function buildRows(link: LinkInfo, queue: QueueInfo | null, nowMs: number): Row[] {
  const rows: Row[] = [];

  rows.push(
    link.internet === "up"
      ? { key: "internet", label: "Internet", state: "ok", value: "Connected", detail: "" }
      : link.internet === "down"
        ? { key: "internet", label: "Internet", state: "bad", value: "No connection", detail: "Check the network cable or Wi-Fi." }
        : { key: "internet", label: "Internet", state: "idle", value: "Checking", detail: "" },
  );

  if (link.kashi === "up") {
    rows.push({ key: "kashi", label: "kashi server", state: "ok", value: "Reachable", detail: link.kashiLastOk ? `Last answer ${agoText(link.kashiLastOk, nowMs)}` : "" });
  } else if (link.kashi === "down") {
    const down = link.kashiDownForSeconds > 0 ? ` for ${durationText(link.kashiDownForSeconds)}` : "";
    rows.push({ key: "kashi", label: "kashi server", state: "bad", value: `Not answering${down}`, detail: link.kashiError });
  } else {
    rows.push({ key: "kashi", label: "kashi server", state: "idle", value: "Checking", detail: "" });
  }

  if (link.live) {
    const since = link.liveSince ? `Connected ${agoText(link.liveSince, nowMs)}` : "";
    const beat = link.liveLastBeat ? `heard from central ${agoText(link.liveLastBeat, nowMs)}` : "";
    rows.push({ key: "live", label: "Live link", state: "ok", value: "Connected", detail: [since, beat].filter(Boolean).join(", ") });
  } else {
    const retry = secondsUntil(link.liveRetryAt, nowMs);
    const retryText = retry === null ? "" : retry === 0 ? "Trying again now" : `Trying again in ${durationText(retry)}`;
    rows.push({
      key: "live", label: "Live link", state: link.liveEverUp || link.kashi === "up" ? "warn" : "idle",
      value: link.liveEverUp || link.kashi === "up" ? "Not connected" : "Connecting",
      detail: [link.liveError, retryText, "Central shows this till as offline until it connects."].filter(Boolean).join(" "),
    });
  }

  const work = workLine(link);
  rows.push({ key: "now", label: "Right now", state: work ? "ok" : "idle", value: work || "Nothing to do", detail: "" });

  rows.push({
    key: "received", label: "Last update from central", state: link.lastPullError ? "warn" : "idle",
    value: agoText(link.lastPullAt, nowMs), detail: link.lastPullError,
  });
  rows.push({ key: "sent", label: "Last sent to central", state: "idle", value: agoText(link.lastPushAt, nowMs), detail: "" });

  if (queue) {
    const queued = queue.pending + queue.confirmedElsewhere;
    rows.push({
      key: "queue", label: "Waiting to send",
      state: queue.failed > 0 ? "warn" : "idle",
      value: queued === 0 && queue.failed === 0 ? "Nothing" : `${queued} waiting${queue.failed > 0 ? `, ${queue.failed} not accepted` : ""}`,
      detail: queue.failed > 0 ? "Not-accepted records are safe on this till; open Settings > Sync to retry." : "",
    });
  }
  return rows;
}
