// Expense and loan categories show their icon in the category colour, with no
// tinted circle (tonal avatar) behind it.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

for (const page of ["expenses.vue", "loans.vue"]) {
  const src = readFileSync(resolve(__dirname, page), "utf8");
  describe(`${page} category icons`, () => {
    it("have no tonal avatar behind them", () => {
      expect(src).not.toMatch(/<k-avatar[^>]*variant="tonal"/);
    });
    it("keep the category colour on the icon itself", () => {
      expect(src).toMatch(/<k-icon :icon="category\.icon \|\| 'mdi-tag-outline'" :color="category\.color \|\| 'blue-grey'"/);
    });
  });
}
