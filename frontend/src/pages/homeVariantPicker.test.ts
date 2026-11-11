// @vitest-environment happy-dom
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

// The sale screen opens the swatch-and-size picker (the one the product
// search uses) when a scan comes back as several colours/sizes, not a plain
// list of barcodes. home.vue is too big to mount; its wiring is read here.
describe("sale screen variant picker", () => {
  const src = readFileSync(resolve(process.cwd(), "src/pages/home.vue"), "utf8");
  it("uses the VariantPicker component for scanned variants", () => {
    expect(src).toContain('import VariantPicker from "@/components/VariantPicker.vue"');
    expect(src).toMatch(/<VariantPicker[^>]*v-model="variantPicker\.show"/);
    expect(src).toMatch(/<VariantPicker[\s\S]*?@select="pickVariant"/);
  });
  it("no longer lists the barcodes in a plain dialog", () => {
    expect(src).not.toContain("Choose a variant of");
  });
  it("hands the picker the item's code, not the scanned barcode", () => {
    expect(src).toMatch(/variantPicker\.code = String\(matches\[0\]\?\.code \|\| value\.trim\(\)\)/);
  });
});
