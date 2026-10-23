import { ref } from "vue";

/**
 * Manager-only pages (Expenses, Loans) are opened from a button that asks for
 * the manager password first. Once it's checked, the button leaves a one-time
 * pass here for that page; the router guard lets the next visit to that path
 * through and spends the pass, so leaving the page locks it again.
 */
const pass = ref<string | null>(null);

export function grantManagerPass(path: string) {
  pass.value = path;
}

export function takeManagerPass(path: string) {
  const ok = pass.value === path;
  pass.value = null;
  return ok;
}
