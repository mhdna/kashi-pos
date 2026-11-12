// The first-run "Set up this till" screen's rules, kept out of the component
// so every case can be tested: the branch code is upper-case letters and
// digits, name and code are required, and the central key and server go
// together (both = connected to central, neither = this till runs on its own,
// one without the other is a mistake worth pointing out).

export interface SetupForm {
  name: string;
  code: string;
  apiKey: string;
  centralApiUrl: string;
}

export type ConnectionMode = "central" | "standalone" | "missing-key" | "missing-url";

export function normalizeBranchCode(raw: string): string {
  return String(raw ?? "").toUpperCase().replace(/[^A-Z0-9]/g, "");
}

export function connectionMode(form: Pick<SetupForm, "apiKey" | "centralApiUrl">): ConnectionMode {
  const key = form.apiKey.trim() !== "";
  const url = form.centralApiUrl.trim() !== "";
  if (key && url) return "central";
  if (!key && !url) return "standalone";
  return key ? "missing-url" : "missing-key";
}

/** The line under the Central section: what will happen with what is typed. */
export function connectionHint(mode: ConnectionMode): { tone: "ok" | "info" | "warn"; text: string } {
  switch (mode) {
    case "central":
      return { tone: "ok", text: "This till will sync with central." };
    case "standalone":
      return { tone: "info", text: "Leave both empty to run this till on its own. You can connect it later in Settings." };
    case "missing-url":
      return { tone: "warn", text: "Add the central server address too, or the key isn't used." };
    case "missing-key":
      return { tone: "warn", text: "Add this branch's central API key too, or the till can't sign in to central." };
  }
}

export function setupErrors(form: SetupForm): { name?: string; code?: string } {
  const out: { name?: string; code?: string } = {};
  if (form.name.trim() === "") out.name = "Give the branch a name";
  if (normalizeBranchCode(form.code) === "") out.code = "Give the branch a short code, e.g. CN";
  return out;
}

export function canSubmit(form: SetupForm): boolean {
  const e = setupErrors(form);
  return !e.name && !e.code;
}

/** What CreateBranch gets: trimmed, the code normalised. */
export function createBranchArgs(form: SetupForm): [string, string, string, string] {
  return [form.name.trim(), normalizeBranchCode(form.code), form.apiKey.trim(), form.centralApiUrl.trim()];
}
