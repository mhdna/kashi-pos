// The per-item "who sold this" dialog for a finished invoice: what the
// cashier picked, compared with what is saved.

export interface SalespersonLine {
  lineId: number;
  salespersonId: number; // 0: the line follows the invoice's salesperson
}

// The lines whose pick differs from what is saved, in invoice order.
// `picks` holds only the lines the cashier touched (lineId -> salesperson id,
// 0 for "same as the invoice"); a line they never touched is never listed.
export function lineSalespersonChanges(
  lines: SalespersonLine[],
  picks: Record<number, number>,
): { lineId: number; salespersonId: number }[] {
  const changes: { lineId: number; salespersonId: number }[] = [];
  for (const line of lines) {
    if (!(line.lineId in picks)) continue;
    const next = Number(picks[line.lineId] || 0);
    if (next !== Number(line.salespersonId || 0)) changes.push({ lineId: line.lineId, salespersonId: next });
  }
  return changes;
}
