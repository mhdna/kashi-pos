export function formatLebanesePhone(value: unknown): string {
  const raw = String(value ?? "").trim();
  if (!raw) return "";

  let digits = raw.replace(/\D/g, "");
  if (digits.startsWith("961")) {
    const nationalNumber = digits.slice(3);
    digits = nationalNumber.length === 7 ? `0${nationalNumber}` : nationalNumber;
  }

  if (digits.length <= 2) return `(${digits})`;

  const prefix = digits.slice(0, 2);
  const middle = digits.slice(2, 5);
  const suffix = digits.slice(5);
  return `(${prefix}) ${middle}${suffix ? `-${suffix}` : ""}`;
}
