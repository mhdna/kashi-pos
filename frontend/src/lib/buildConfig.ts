const profile = (import.meta.env.VITE_KASHI_BUILD_PROFILE || "branch").trim().toLowerCase();

const defaultPages = profile === "central" ? ["import-data", "inventory"] : ["import-data"];
const configuredPages = (import.meta.env.VITE_KASHI_ENABLED_PAGES || "")
  .split(",")
  .map((page: string) => page.trim())
  .filter(Boolean);
const centralOnlyPages = new Set(["inventory"]);

export const buildConfig = {
  profile,
  isCentral: profile === "central",
  branchNameDefault: profile === "central" ? "المركزي" : "Branch",
  enabledPages: new Set(configuredPages.length ? configuredPages : defaultPages),
  canManageProducts: profile === "central",
};

export function pageEnabled(page: string) {
  if (centralOnlyPages.has(page) && profile !== "central") return false;
  return buildConfig.enabledPages.has(page);
}
