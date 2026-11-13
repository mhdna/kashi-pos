// The calculator's total card shows the total on top and, under it, how to
// pay it in cash: whole units of one currency plus the rest in another
// ("$30 + LL 45,000"). When the rest is zero, that second line is a single
// amount -- and if it is in the same currency as the total above, it only
// repeats the total ("$30.00" over "$30"), so it is not shown as the split:
// the card shows the same total in the other currency there instead (see
// CalculatorStatus.vue), so both views have a second line.

export interface TotalSplit {
  first: string;
  second: string;
  // Currency of `first` when `second` is empty; "" when unknown.
  firstCode?: string;
}

export function splitRepeatsTotal(split: TotalSplit, displayedCode: string): boolean {
  if (split.second) return false;
  if (!split.firstCode || !displayedCode) return false;
  return split.firstCode === displayedCode;
}
