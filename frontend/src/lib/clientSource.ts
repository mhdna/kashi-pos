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

// Each option's icon and colour. Central's starting options have fixed uids;
// one added in kashi is recognised by name (English or French), otherwise it
// gets a megaphone in grey.
export type SourceLook = { icon: string; color: string };

const byUID: Record<string, SourceLook> = {
  "6f1c2a00-5e1d-4c7a-9b00-000000000001": { icon: "mdi-share-variant", color: "#8E24AA" },
  "6f1c2a00-5e1d-4c7a-9b00-000000000002": { icon: "mdi-message-text", color: "#1E88E5" },
  "6f1c2a00-5e1d-4c7a-9b00-000000000003": { icon: "mdi-account-heart", color: "#E53935" },
  "6f1c2a00-5e1d-4c7a-9b00-000000000004": { icon: "mdi-walk", color: "#43A047" },
  "6f1c2a00-5e1d-4c7a-9b00-000000000005": { icon: "mdi-dots-horizontal-circle-outline", color: "#757575" },
};

const byName: [RegExp, SourceLook][] = [
  [/instagram|insta\b/, { icon: "mdi-instagram", color: "#C13584" }],
  [/facebook|\bfb\b/, { icon: "mdi-facebook", color: "#1877F2" }],
  [/whats ?app/, { icon: "mdi-whatsapp", color: "#25A244" }],
  [/social|réseaux|reseaux/, byUID["6f1c2a00-5e1d-4c7a-9b00-000000000001"]!],
  [/\bsms\b|text|message/, byUID["6f1c2a00-5e1d-4c7a-9b00-000000000002"]!],
  [/friend|ami|family|famille|bouche/, byUID["6f1c2a00-5e1d-4c7a-9b00-000000000003"]!],
  [/walk|passing|passage|street|rue/, byUID["6f1c2a00-5e1d-4c7a-9b00-000000000004"]!],
  [/^other$|^autre$/, byUID["6f1c2a00-5e1d-4c7a-9b00-000000000005"]!],
];

export const defaultSourceLook: SourceLook = { icon: "mdi-bullhorn", color: "#616161" };

export function sourceLook(option: ClientSourceOption): SourceLook {
  const known = byUID[option.uid];
  if (known) return known;
  const name = option.name.trim().toLowerCase();
  for (const [re, look] of byName) if (re.test(name)) return look;
  return defaultSourceLook;
}
