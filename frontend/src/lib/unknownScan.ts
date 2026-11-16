// A scan the catalog doesn't know: when it is a real retail barcode (EAN-8,
// UPC-A, EAN-13 or GTIN-14 whose check digit holds), the till offers to sell
// it as an unknown item and sends the barcode to central with the picture,
// where the label reader makes it a product under that exact barcode
// (central's api/label_reader.go). Anything else (a typo of a code, a word)
// is just "not found", as before.

export function validGTIN(digits: string): boolean {
  if (!/^\d+$/.test(digits) || ![8, 12, 13, 14].includes(digits.length)) return false;
  let sum = 0;
  // Weights alternate 3, 1 from the digit left of the check digit.
  for (let i = digits.length - 2; i >= 0; i--) {
    const d = Number(digits[i]);
    sum += (digits.length - 2 - i) % 2 === 0 ? d * 3 : d;
  }
  return (10 - (sum % 10)) % 10 === Number(digits[digits.length - 1]);
}

// The barcode to carry onto the unknown item, or "" when the scan isn't one.
export function unknownScanBarcode(scanned: string): string {
  const value = String(scanned ?? "").trim();
  return validGTIN(value) ? value : "";
}

export type UnknownUnit = {
  label: string;
  unitPriceCents: number;
  discount: number;
  lineRef: string;
  slotRef: string;
  barcode?: string;
};

// One unit per piece sold of each unknown line, tagged with its cart line:
// central takes one picture per line, whatever its qty. A line scanned from
// a barcode the catalog didn't know carries it on every unit.
export function unknownUnitsOf(items: any[], isUnknown: (item: any) => boolean): UnknownUnit[] {
  const units: UnknownUnit[] = [];
  let line = 0;
  for (const item of items ?? []) {
    if (!isUnknown(item) || !(item.qty > 0)) continue;
    line++;
    // A line from before unknownRef (an old draft or hold) goes by position.
    const lineRef = String(item.unknownRef || line);
    for (let i = 0; i < item.qty; i++) {
      const price = Number(item.unitPrices?.[i] ?? item.price);
      units.push({
        label: `Unknown item $${price.toFixed(2)}`,
        unitPriceCents: Math.round(price * 100),
        discount: Math.trunc(Number(item.unitDiscounts?.[i] ?? item.discount ?? 0)),
        lineRef,
        slotRef: `${lineRef}:${i + 1}`,
        ...(item.unknownBarcode ? { barcode: String(item.unknownBarcode) } : {}),
      });
    }
  }
  return units;
}
