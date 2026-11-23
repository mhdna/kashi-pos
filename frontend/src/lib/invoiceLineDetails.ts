import { t } from "../i18n";

// The product details the invoice screen shows under a sold line, so two
// products that share a name ("Lady Leather Skirt" as TNV551219 and as
// TNV551219-16) can be told apart at a glance. GetInvoiceDetails carries the
// fields (TestInvoiceDetailsCarryEveryProductDetail writes a golden of them).

export interface InvoiceLineProduct {
  code?: string | null;
  barcode?: string | null;
  color?: string | null;
  size?: string | null;
  category?: string | null;
  brand?: string | null;
  kind?: string | null;
  family?: string | null;
  season?: string | null;
  productYear?: number | string | null;
}

export interface InvoiceLineDetail {
  key: keyof InvoiceLineProduct;
  label: string;
  value: string;
}

// Shown in this order; code and barcode first, they identify the product.
// A function, not a constant, so the labels follow the EN/FR toggle.
function fields(): Array<[keyof InvoiceLineProduct, string]> {
  return [
    ["code", t("Code")],
    ["barcode", t("Barcode")],
    ["color", t("Color")],
    ["size", t("Size")],
    ["category", t("Category")],
    ["brand", t("Brand")],
    ["kind", t("Kind")],
    ["family", t("Family")],
    ["season", t("Season")],
    ["productYear", t("Year")],
  ];
}

// A blank, "-" (the catalogue's "none") or a zero year shows nothing.
function shown(key: keyof InvoiceLineProduct, raw: unknown): string {
  if (raw === null || raw === undefined) return "";
  const value = String(raw).trim();
  if (value === "" || value === "-") return "";
  if (key === "productYear" && !(Number(value) > 0)) return "";
  return value;
}

export function invoiceLineDetails(line: InvoiceLineProduct | null | undefined): InvoiceLineDetail[] {
  if (!line) return [];
  const out: InvoiceLineDetail[] = [];
  for (const [key, label] of fields()) {
    const value = shown(key, line[key]);
    if (value) out.push({ key, label, value });
  }
  return out;
}
