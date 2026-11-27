// "Where did they hear about us?" on the new-customer step. The options come
// from central (GetClientSources); with none, the form doesn't ask.

export type ClientSourceOption = { uid: string; name: string };

// The options to show, in central's order, without blanks or repeats.
export function sourceOptions(list: ClientSourceOption[] | null | undefined): ClientSourceOption[] {
  const seen = new Set<string>();
  const out: ClientSourceOption[] = [];
  for (const o of list ?? []) {
    const uid = (o?.uid ?? "").trim();
    const name = (o?.name ?? "").trim();
    if (!uid || !name || seen.has(uid)) continue;
    seen.add(uid);
    out.push({ uid, name });
  }
  return out;
}

// Tapping an option picks it; tapping the picked one again clears it.
export function toggleSource(picked: string, uid: string): string {
  return picked === uid ? "" : uid;
}

// The pick to send: "" when it isn't one of the options (they changed while
// the form was open).
export function sourceToSend(options: ClientSourceOption[], picked: string): string {
  return options.some((o) => o.uid === picked) ? picked : "";
}

// Save needs a name and, when there are options, an answer.
export function canSaveNewClient(name: string, options: ClientSourceOption[], picked: string): boolean {
  if (!name.trim()) return false;
  return options.length === 0 || sourceToSend(options, picked) !== "";
}
