// Picking up where the cashier was after a forced update (resume.go).
// The till saves the screen's route before it restarts and gets it back
// after; only a plain in-app path is ever followed.

/** The route to go back to, or "" when there is none worth going to. */
export function safeResumeRoute(route: unknown): string {
  if (typeof route !== "string") return "";
  const r = route.trim();
  if (r.length === 0 || r.length > 500) return "";
  if (!r.startsWith("/") || r.startsWith("//") || /[\\\s]/.test(r)) return "";
  const path = r.split(/[?#]/)[0];
  if (path === "/" || path === "/login" || path.startsWith("/auth")) return "";
  return r;
}

export interface ResumedSession {
  user: { id: number; username: string; role: string };
  route: string;
}

/** What TakeResumeSession returned, as a session to restore, or null. */
export function resumedSession(raw: any): ResumedSession | null {
  const role = raw?.role;
  if (role !== "cashier" && role !== "admin") return null;
  return {
    user: { id: Number(raw.id) || 0, username: String(raw.username || role), role },
    route: safeResumeRoute(raw.route) || "/home",
  };
}
