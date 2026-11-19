// The Rewards button's logic: how a deal reads, and how many rewards a sale
// has on (the number by the button).

export type RewardDeal = {
  uid: string;
  name: string;
  kind: string; // "percent" | "amount"
  percent: number;
  amount: number; // cents
  minTotal: number; // cents
  cashierPicks: boolean;
};

function dollars(cents: number): string {
  const v = cents / 100;
  return Number.isInteger(v) ? `$${v}` : `$${v.toFixed(2)}`;
}

// dealSize is the deal's headline: "15%" or "$5".
export function dealSize(d: RewardDeal): string {
  return d.kind === "amount" ? dollars(d.amount) : `${d.percent}%`;
}

// dealTitle is a deal's name without the size the card already shows:
// "Students 15%" reads "Students"; a name that is only the size stays.
export function dealTitle(d: RewardDeal): string {
  const size = dealSize(d);
  const title = d.name.split(size).join(" ").replace(/\s+/g, " ").trim();
  return title || d.name;
}

// dealWhen is a deal's condition in a few words: "Whole invoice", "> $50".
// (A total of exactly the minimum counts too; the card's title says "or more".)
export function dealWhen(d: RewardDeal): string {
  return d.minTotal > 0 ? `> ${dollars(d.minTotal)}` : "Whole invoice";
}

// dealTerms is what a deal takes off and when: "15% off the whole invoice",
// "$5 off invoices of $100 or more".
export function dealTerms(d: RewardDeal): string {
  const off = d.kind === "amount" ? `${dollars(d.amount)} off` : `${d.percent}% off`;
  return d.minTotal > 0 ? `${off} invoices of ${dollars(d.minTotal)} or more` : `${off} the whole invoice`;
}

// appliedDealNames are the deals the quote took off: one, or several joined
// with " + " where the branch lets deals add up (invoiceDiscountsStacked).
export function appliedDealNames(appliedName: string): string[] {
  return appliedName ? appliedName.split(" + ") : [];
}

// rewardsActiveCount: the referral code given, each deal picked, and each
// deal that applied by itself (appliedName, from the quote) that isn't one of
// the picked ones, so a picked deal that applied is counted once.
export function rewardsActiveCount(code: string, picked: string[], appliedName: string, pickedNames: string[] = []): number {
  let n = (code ? 1 : 0) + picked.length;
  for (const name of appliedDealNames(appliedName)) {
    if (!pickedNames.includes(name)) n++;
  }
  return n;
}
