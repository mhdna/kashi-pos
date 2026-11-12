// The in-place search results must end where the keyboard starts. Their root is
// ProductSearchResults' .product-list (height: 100%); home.vue overrides that
// height. The two live in different CSS chunks, and in the built app the chunk
// order made the equal-weight .product-list win, so the grid ran behind the
// keyboard. home.vue's rule must outweigh it, whatever the load order.
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { join } from "node:path";

const src = join(__dirname, "..");
// Each <style> block of a .vue file, and whether it is scoped.
function blocks(file: string): { scoped: boolean; css: string }[] {
  const text = readFileSync(join(src, file), "utf8");
  return [...text.matchAll(/<style([^>]*)>([\s\S]*?)<\/style>/g)].map((m) => ({ scoped: /scoped/.test(m[1] ?? ""), css: m[2] ?? "" }));
}

// Class + attribute count (scoped styles add one attribute to each).
function weight(selector: string): number {
  return (selector.match(/\.[\w-]+/g) ?? []).length + (selector.match(/\[[^\]]+\]/g) ?? []).length;
}

// The rule, and its weight as built: scoped styles add one attribute.
function rule(file: string, test: (selector: string) => boolean): { selector: string; body: string; weight: number } {
  for (const block of blocks(file)) {
    for (const m of block.css.matchAll(/([^{}]+)\{([^}]*)\}/g)) {
      const selector = (m[1] ?? "").replace(/\/\*[\s\S]*?\*\//g, "").trim();
      if (test(selector)) return { selector, body: m[2] ?? "", weight: weight(selector) + (block.scoped ? 1 : 0) };
    }
  }
  throw new Error("rule not found");
}

describe("in-place search results height", () => {
  const home = rule("pages/home.vue", (s) => /\.home-search-results$/.test(s));
  const list = rule("components/ProductSearchResults.vue", (s) => s === ".product-list");

  it("the reported case: home.vue's height outweighs .product-list's height: 100%", () => {
    expect(list.body).toMatch(/height:\s*100%/);
    expect(home.body).toMatch(/height:\s*calc\(100% - var\(--home-search-keyboard-height/);
    expect(list.weight).toBe(2);
    expect(home.weight).toBeGreaterThan(list.weight);
  });

  it("the results stop at the keyboard's top", () => {
    expect(home.body).toMatch(/bottom:\s*var\(--home-search-keyboard-height/);
  });
});
