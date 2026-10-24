// Fails when a literal `mdi-*` icon name used under src/ isn't registered in
// src/lib/icons.ts (the till's custom, tree-shaken icon set). An unregistered icon
// renders its name as text and logs a warning on every render, which can make
// the app crawl. Runs before every build (see the "build" script).
import { readdirSync, readFileSync } from "node:fs";
import { extname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(fileURLToPath(new URL(".", import.meta.url)), "..", "src");
const iconsFile = join(root, "lib", "icons.ts");

const registered = new Set(
  [...readFileSync(iconsFile, "utf8").matchAll(/^\s*["'](mdi-[a-z0-9-]+)["']:/gm)].map((m) => m[1]),
);

const used = new Map();
function walk(dir) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) {
      walk(path);
    } else if ([".vue", ".js", ".ts"].includes(extname(entry.name)) && path !== iconsFile) {
      const lines = readFileSync(path, "utf8").split("\n");
      for (const [index, line] of lines.entries()) {
        // A name that ends in "-" or sits in a ${...} is built at run time and
        // can't be checked here; the component's fallback covers it.
        for (const match of line.matchAll(/(?<![\w-])(mdi-[a-z0-9]+(?:-[a-z0-9]+)*)(?![\w${-])/g)) {
          const name = match[1];
          if (!used.has(name)) {
            used.set(name, []);
          }
          used.get(name).push(`${path.slice(root.length - 3)}:${index + 1}`);
        }
      }
    }
  }
}
walk(root);

const missing = [...used].filter(([name]) => !registered.has(name));
if (missing.length > 0) {
  console.error("Icons used but not registered in src/lib/icons.ts:\n");
  for (const [name, places] of missing) {
    console.error(`  ${name}  (${places.slice(0, 3).join(", ")}${places.length > 3 ? ", …" : ""})`);
  }
  console.error("\nImport each from @mdi/js and add it to iconPaths.");
  process.exitCode = 1;
} else {
  console.log(`icons ok: ${used.size} names used, all registered`);
}
