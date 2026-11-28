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
// gets a grey tag.
export type SourceLook = { icon: string; color: string };

const look = {
  social: { icon: "mdi-share-variant", color: "#8E24AA" },
  friend: { icon: "mdi-account-heart", color: "#E53935" },
  walking: { icon: "mdi-walk", color: "#EF6C00" },
  maps: { icon: "mdi-google-maps", color: "#1A73E8" },
  whatsapp: { icon: "mdi-whatsapp", color: "#25A244" },
  ad: { icon: "mdi-bullhorn", color: "#6D4C41" },
  instagram: { icon: "mdi-instagram", color: "#C13584" },
  facebook: { icon: "mdi-facebook", color: "#3B5998" },
  sms: { icon: "mdi-message-text", color: "#00838F" },
  other: { icon: "mdi-dots-horizontal-circle-outline", color: "#757575" },
} satisfies Record<string, SourceLook>;

const byUID: Record<string, SourceLook> = {
  "6f1c2a00-5e1d-4c7a-9b00-000000000001": look.social,
  "6f1c2a00-5e1d-4c7a-9b00-000000000003": look.friend,
  "6f1c2a00-5e1d-4c7a-9b00-000000000004": look.walking,
  "6f1c2a00-5e1d-4c7a-9b00-000000000006": look.maps,
  "6f1c2a00-5e1d-4c7a-9b00-000000000007": look.whatsapp,
  "6f1c2a00-5e1d-4c7a-9b00-000000000008": look.ad,
};

const byName: [RegExp, SourceLook][] = [
  [/instagram|insta\b/, look.instagram],
  [/facebook|\bfb\b/, look.facebook],
  [/whats ?app/, look.whatsapp],
  [/google|maps?\b|carte/, look.maps],
  [/social|réseaux|reseaux/, look.social],
  [/\bsms\b|text|message/, look.sms],
  [/friend|ami|family|famille|bouche/, look.friend],
  [/walk|passing|passage|street|rue/, look.walking],
  [/\bads?\b|flyer|poster|affiche|pub|billboard|panneau/, look.ad],
  [/^other$|^autre$/, look.other],
];

export const defaultSourceLook: SourceLook = { icon: "mdi-tag-outline", color: "#616161" };

export function sourceLook(option: ClientSourceOption): SourceLook {
  const known = byUID[option.uid];
  if (known) return known;
  const name = option.name.trim().toLowerCase();
  for (const [re, look] of byName) if (re.test(name)) return look;
  return defaultSourceLook;
}
