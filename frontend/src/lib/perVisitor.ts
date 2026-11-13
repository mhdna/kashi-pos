// What the Balance month list's visitors chip adds when hovered: the day's
// takings per visitor.

/** The takings per visitor, in the same units as `total`; null without visitors. */
export function perVisitor(total: number, visitors: number | null | undefined): number | null {
  if (!visitors || visitors <= 0 || !Number.isFinite(total)) return null;
  return Math.round(total / visitors);
}
