// The two halves of resuming after a forced update, with what they touch
// passed in so they can be tested without the app (lib/resume.ts, resume.go).
import { resumedSession, type ResumedSession } from "./resume";

interface RestoreDeps {
  take: () => Promise<any>;
  isLoggedIn: () => boolean;
  setUser: (user: ResumedSession["user"]) => void;
  go: (route: string) => Promise<unknown>;
}

/** Signs the same role back in and returns to the saved screen. Never throws. */
export async function restoreSession(deps: RestoreDeps): Promise<boolean> {
  try {
    const session = resumedSession(await deps.take());
    // Someone already signed in by hand: leave them where they are.
    if (!session || deps.isLoggedIn()) return false;
    deps.setUser(session.user);
    await deps.go(session.route);
    return true;
  } catch {
    return false;
  }
}

interface SaveDeps {
  flushDraft: () => void;
  route: () => string;
  save: (route: string) => Promise<unknown>;
}

/** Answers "where are you?" before a restart. Never throws. */
export async function saveRoute(deps: SaveDeps): Promise<void> {
  try {
    // The open sale's draft is written on a short delay; write it now.
    deps.flushDraft();
    await deps.save(deps.route());
  } catch {
    // The restart goes ahead without it; the cashier signs in as usual.
  }
}
