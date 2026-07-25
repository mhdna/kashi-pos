export interface Client {
  id: number;
  name: string;
  phone: string;
  initials: string;
  tier: string;
  points: number;
  totalLoyaltyPoints: number;
  validLoyaltyPoints: number;
}

export function toClient(raw: Partial<Client> & Record<string, any>): Client {
  const loyaltyPoints = Number(raw.totalLoyaltyPoints ?? raw.points ?? 0);
  const name = String(raw.name ?? "Anonymous");
  return {
    id: Number(raw.id ?? 0),
    name,
    phone: String(raw.phone ?? ""),
    initials: raw.initials || initials(name),
    tier: raw.tier || loyaltyTier(loyaltyPoints),
    points: Number(raw.points ?? loyaltyPoints),
    totalLoyaltyPoints: loyaltyPoints,
    validLoyaltyPoints: Number(raw.validLoyaltyPoints ?? 0),
  };
}

export function anonymousClient(): Client {
  return toClient({ id: 0, name: "Anonymous", phone: "", tier: "None" });
}

function initials(name: string): string {
  return name
    .split(" ")
    .map((word) => word[0])
    .join("")
    .toUpperCase()
    .slice(0, 2);
}

function loyaltyTier(points: number): string {
  if (points >= 500) return "Gold";
  if (points >= 200) return "Silver";
  if (points >= 1) return "Bronze";
  return "None";
}
