# Live view

An admin in kashi can watch this till's screen live and, if allowed, click and
type on it, instead of using AnyDesk. Design: `kashi/docs/plans/live-view.md`.
Code: `live_view.go` (Go), `frontend/src/lib/liveView.ts`,
`liveViewControl.ts`, `liveViewWiring.ts`, `components/LiveViewBanner.vue`.

## What it costs

- **Off means nothing runs.** Kashi has two switches (view, control), both off by
  default. Off, its hello never lists `live_view`, so a start message is ignored,
  no socket opens, and rrweb is never loaded.
- On, an idle till still does nothing. Only while an admin watches does the
  page record (rrweb, a lazy chunk) and stream.
- Stalled kashi or network: the till drops frames (`LiveViewSend` returns false),
  never waits. A watcher then gets a fresh full snapshot.

## Flow

1. Kashi pushes `live_view_start` on the branch socket (`sync_ws.go`
   `handleLiveMessage`).
2. Go dials `GET /branch/live` with the branch headers (`DialLive`, compression on).
   The webview never talks to kashi; credentials stay in Go.
3. Go emits `live_view:start`; the UI loads rrweb and records. Batches go out
   every 100 ms through the `LiveViewSend` binding.
4. `live_view:snapshot` (kashi wants a full picture) makes the UI take one, at most
   every 2 s. A dropped batch does the same.
5. `live_view_stop`, the feature being switched off (`live_view_settings`), a 409
   from the dial (nobody watches) or 2 minutes of silence from kashi ends it and
   emits `live_view:stop`. A dropped stream is redialed with backoff and the UI
   takes a new snapshot.

## Control

- Only when kashi's start message says `control` **and** its hello offers
  `live_view_control`. Messages from watchers: `click {id}`, `input {id,v}`,
  `key {key}`, with rrweb node ids (`liveViewControl.ts`). Unknown ids and keys are
  ignored; keys with Ctrl/Alt/Meta never arrive.
- The till shows no banner or mark while watched or controlled (removed on request).
- Control can be switched off mid-session (`live_view:mode`); the next message is dropped.

## Separate screen

A second, hidden copy of this app (`index.html?virtual=1` in an off-screen iframe,
`virtualSession.ts`) that one admin drives from kashi while the cashier keeps
selling. Go opens it on `live_virtual:start` (kashi `live_view_start` with
`virtual:true`; needs the hello's `live_view_control`) and closes it on `live_virtual:stop`.

- **Own sign-in in Go** (`live_virtual_call.go`): the copy cannot tell Go apart from
  the cashier's window, so all its calls go through one binding, `LiveVirtualCall`,
  which runs the same methods with the copy's own role and PIN lockout. The 11 gated
  methods are split into `X` (cashier) and `xAs(caller, ...)`; a new gate without a
  twin fails `TestGuardNoGateSkipsItsTwin`. Whole-till actions (switch branch,
  restart, update, cash machine, live view, live cart) are refused there.
- **Bridge** (`virtualBridge.ts`, first import in `main.ts`): in the copy only,
  `window.go` is a proxy onto `LiveVirtualCall`, the runtime is the parent's
  (events only), and `localStorage`/`sessionStorage` are prefixed `virtual:` so the
  cashier's sale draft is never touched.
- **Focus guard**: focus moving into the frame is handed back at once, so the
  cashier's keyboard and barcode scanner never reach the copy.
- The copy shows no toasts, banners or sounds, and never reports its cart to kashi.
- Its own recorder streams on `/branch/live?mode=virtual` (`LiveVirtualSend`, always
  with control). **Real hardware actions (printing) from the copy are real.**

## Privacy

Password inputs are masked by rrweb; `img, video, canvas` are blocked (blank boxes
of the same size, so no photo bytes travel). Nothing is stored on either side.

## Wire format

Packets are `{"full":true,"e":[...],"a":[...]}` or `{"e":..,"a":..}`.
**`full` must stay the first key**: kashi reads the text's prefix to tell a full
snapshot from an increment without parsing (`framePacket` in `liveView.ts`).
Actions are `{k:"click", l:label, at:ms}`.

## Tests

`live_view_test.go` (stand-in kashi: headers, streaming, control gating, 409, redial,
silence, a stalled kashi never blocks `send`), `liveView.test.ts`,
`liveViewControl.test.ts` (including a randomised typing model),
`liveViewWiring.test.ts` (rrweb not loaded until start), `LiveViewBanner.test.ts`;
separate screen: `live_virtual_call_test.go` (the reported case, randomised interleaved
logins), `live_virtual_test.go`, `virtualBridge.test.ts`, `virtualSession.test.ts`,
`virtualRecorder.test.ts`, `virtualGates.test.ts`.
