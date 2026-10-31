# The Windows till

What the app does to the Windows PC it runs on. All of it is Windows-only
(`*_windows.go`); on other platforms these are no-ops. Most of it is off in
`wails dev` builds.

## Kiosk mode (`kiosk_windows.go`)

While the app runs, the till shows nothing else:

- **Explorer is asked to exit** (the same clean exit as the old "Exit Explorer"
  menu item, so Windows doesn't restart it). No taskbar, Start, tray, desktop
  or notifications.
- The till's window stays on top, and when another program takes focus the
  till takes it straight back, so a scan never types into another window.
- A system-wide keyboard hook makes the Windows key, Alt+Tab, Alt+Esc,
  Ctrl+Esc and Alt+F4 do nothing while the till is in front.
- Minimized windows aren't drawn.
- Other programs are closed (cash machine mode, below).

Programs allowed in front of the till (`kioskAllowedPrograms`): Task Manager,
the Windows admin prompt, Tailscale (`tailscale-ipn.exe`) and AnyDesk.

**Getting out:** Quit → Close App starts Explorer again. Shut Down and Restart
leave it off (the PC is going down anyway). If the app crashed: Ctrl+Shift+Esc
→ Task Manager → Run new task → `explorer`. Nothing is stored, so a reboot
always brings Explorer back.

Turn kiosk mode off on a till with `KASHI_KIOSK=false` (in a `.env` next to the
exe).

## Cash machine mode (`cash_machine.go`)

Tills came with dozens of startup programs eating CPU and memory.

- **Every launch, while kiosk mode is on:** the user's own startup entries are
  switched off (the same switch as Task Manager's Startup tab, so they can be
  turned back on there), and every other program the user runs is closed,
  again every 30 seconds. Windows' own programs and services are never
  touched.
- **Once, from Settings → General → "Make this a cash machine"** (needs a
  Windows admin's approval): machine-wide startup entries and third-party
  scheduled tasks off, apps a till never uses removed, background services off,
  no sleep, Windows Update outside opening hours. **Removed apps don't come
  back by themselves.**
- Kept: GB Cloud and its WebView2 and update helper, AnyDesk, touch screen
  drivers, the on-screen keyboard, Task Manager.

## Starting with Windows (`autostart_windows.go`)

The app writes its own entry under `HKCU\...\CurrentVersion\Run` on **every
launch** (not the installer), so tills that only ever got updates from kashi
have it too, and a moved install fixes itself. Turning it off in Task
Manager's Startup tab still works: that choice is kept in a separate
`StartupApproved` key the app never touches.

## WebView2 profile (`webview_windows.go`)

The webview's profile lives in `%LOCALAPPDATA%`, not the default
`%APPDATA%\GB-Cloud.exe`. The default folder caused "Microsoft Edge can't read
and write to its data directory" when an elevated run (the installer's "Run
GB Cloud" checkbox) created it first. The folder is test-written before use; if
that fails a temp folder is used, since losing the webview cache is harmless.

## Splash (`splash_windows.go`)

A plain native window with the logo appears within milliseconds of launch and
covers the seconds before WebView2 can paint. It closes on `OnDomReady`, or by
a safety timer if that never fires. It logs to
`%TEMP%\kashicloud-splash.log`.

## Devices on the nav rail (`devices.go`, `devices_windows.go`)

- **Scanner vs keyboard:** both reach Windows as keyboards. A hidden window
  reads raw keyboard input, which names the device each key came from, and the
  till learns which device is the scanner from how it types: 6 or more keys,
  each under 50 ms apart, ending in Enter or Tab. Nobody types that fast. It's
  remembered in `devices.json` (next to `branches.json`); a new scanner
  replaces the old one on its first scan.
- Keys injected by AnyDesk or the on-screen keyboard have no device and are
  ignored for this.
- Keyboard: any other USB keyboard plugged in. Mouse: any USB mouse that isn't
  a touch screen (Windows lists touch screens as mice).
- Printer: Windows' spooler status for the receipt printer in Settings; a USB
  printer shows offline when unplugged or off.
- **Finding the printer again** (Devices dialog, admins): a USB receipt
  printer plugged into another socket often comes back on a new port (USB002)
  or as a new printer ("POS-80 (Copy 1)"). The dialog lists printers, their
  ports, and which USB ports have a printer on them right now, so the admin
  can switch.

## Stuck keys (`stuck_keys.go`)

AnyDesk from a Mac sends ⌘ as the Windows key. When the Mac keeps a key's
release to itself (⌘+Tab away from AnyDesk), the till never hears it come up:
Windows keeps holding Win, so typing `1` does Win+1 and every scan fires
shortcuts, until someone presses Win on the till. AnyDesk's "Transmit keyboard
shortcuts" off doesn't prevent it.

The device listener tracks which modifiers a real keyboard or scanner on the
till is holding. Every second it checks which modifiers Windows thinks are
held; any (Win, Ctrl, Alt, Shift) held by nothing on the till for 3 seconds is
released with an injected key-up. The kiosk keyboard hook lets that key-up
through (it's marked with `stuckKeyRelease` in `dwExtraInfo`).

Switch: Settings → General → Stuck keys (`release_stuck_keys`, on by default).
**Till-only on purpose:** kashi doesn't see or push it. It exists for our own
remote sessions and isn't something to offer customers (noted in kashi's
roadmap, M9).

## Local servers on the till

| Address | What | Override |
|---|---|---|
| `0.0.0.0:8788` | Close-shift receipt photo upload (phones on the shop Wi-Fi) | `KASHI_SHIFT_UPLOAD_LISTEN` |
| `0.0.0.0:8789` | Attendance device ping | `KASHI_ATTENDANCE_PUSH_LISTEN` |
| `127.0.0.1:8787` | Legacy kashi-admin API, off unless enabled | `KASHI_ADMIN_ENABLED`, `KASHI_ADMIN_LISTEN`, `KASHI_ADMIN_TOKEN` |

Windows Firewall must allow the first two for phones and the attendance device
to reach them.

## The compact build

`make build-windows-compact` builds "GB Cloud Compact.exe" with the `compact`
Go tag and `VITE_KASHI_COMPACT=1`: a 1024x768 window for small screens. It's a
separate **flavor**: kashi only ever sends a till an update of the flavor it
runs (`app_flavor*.go`), so publish both when a change matters to both.
