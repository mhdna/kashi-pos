// Every notification has the same grey background (ui.css's
// .k-snackbar__wrapper); a colour may only change the foreground. A screen that
// paints its own snackbar background brings back toasts that don't match.
import { describe, expect, it } from "vitest";
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join } from "node:path";

const src = join(__dirname, "..");

function files(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) return files(path);
    return /\.(vue|css|scss)$/.test(name) ? [path] : [];
  });
}

// Each CSS rule (selector + body) that targets a snackbar's wrapper.
function wrapperRules(text: string): { selector: string; body: string }[] {
  const out: { selector: string; body: string }[] = [];
  for (const m of text.matchAll(/([^{}]*k-snackbar__wrapper[^{}]*)\{([^}]*)\}/g)) {
    out.push({ selector: m[1].trim(), body: m[2] });
  }
  return out;
}

describe("notification look", () => {
  const css = readFileSync(join(src, "ui/ui.css"), "utf8");

  it("gives the base snackbar the grey background", () => {
    const base = wrapperRules(css).find((r) => r.selector.endsWith(".k-snackbar__wrapper"));
    expect(base?.body).toMatch(/background:\s*rgba\(var\(--k-theme-surface-variant\)/);
  });

  it("lets a colour tint the foreground, never the background", () => {
    const colored = wrapperRules(css).filter((r) => r.selector.includes("--colored"));
    expect(colored.length).toBeGreaterThan(0);
    for (const rule of colored) expect(rule.body).not.toMatch(/background/);
  });

  it("has no screen paint its own snackbar background", () => {
    const offenders = files(src)
      .filter((path) => !path.endsWith(join("ui", "ui.css")))
      .flatMap((path) =>
        wrapperRules(readFileSync(path, "utf8"))
          .filter((r) => /background/.test(r.body))
          .map((r) => `${path.slice(src.length + 1)}: ${r.selector}`),
      );
    expect(offenders).toEqual([]);
  });
});
