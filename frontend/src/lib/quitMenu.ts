// The Quit menu (the nav rail's bottom button): what each option does, grouped
// by what it affects, so "Restart" can't be mistaken for another restart.
//   Shift       the cashier's session
//   This app    GB Cloud itself (Reload closes and reopens it)
//   This computer  Windows

export type QuitAction =
  | "close-shift"
  | "logout"
  | "reload-app"
  | "close-app"
  | "restart-pc"
  | "shutdown-pc";

export interface QuitOption {
  action: QuitAction;
  label: string;
  icon: string;
  /** One line under the title in the confirmation. */
  hint?: string;
}

/** The colour family of a group: green for the shift, blue for the app, red for the computer. */
export type QuitTone = "shift" | "app" | "computer";

export interface QuitGroup {
  /** Not shown as a caption; names the group for screen readers and tests. */
  title: string;
  tone: QuitTone;
  options: QuitOption[];
}

export const QUIT_GROUPS: QuitGroup[] = [
  {
    title: "Shift",
    tone: "shift",
    options: [
      { action: "close-shift", label: "Close Shift", icon: "mdi-close-circle-outline" },
      { action: "logout", label: "Logout", icon: "mdi-logout" },
    ],
  },
  {
    title: "This app",
    tone: "app",
    options: [
      { action: "reload-app", label: "Reload App", icon: "mdi-autorenew", hint: "Close GB Cloud and open it again." },
      { action: "close-app", label: "Close App", icon: "mdi-close", hint: "Close GB Cloud and go to Windows." },
    ],
  },
  {
    title: "This computer",
    tone: "computer",
    options: [
      { action: "restart-pc", label: "Restart PC", icon: "mdi-restart", hint: "Restart this computer." },
      { action: "shutdown-pc", label: "Shut Down PC", icon: "mdi-power", hint: "Turn this computer off." },
    ],
  },
];

export const QUIT_OPTIONS: QuitOption[] = QUIT_GROUPS.flatMap((g) => g.options);
