// Till language: French by default, English on request. English text is the
// key and FR holds its French, so a screen is converted by wrapping its text in
// t("...") and an untranslated string simply shows in English.
// {placeholders} must stay as they are.
import { ref } from "vue";
import { FR } from "./fr";

const KEY = "locale";
export type Locale = "fr" | "en";

function initial(): Locale {
  try {
    const saved = localStorage.getItem(KEY);
    if (saved === "en" || saved === "fr") return saved;
  } catch {
    // Storage blocked: use the default.
  }
  return "fr";
}

export const locale = ref<Locale>(initial());

function apply() {
  if (typeof document !== "undefined") document.documentElement.lang = locale.value;
}
apply();

export function setLocale(value: string) {
  locale.value = value === "en" ? "en" : "fr";
  try {
    localStorage.setItem(KEY, locale.value);
  } catch {
    // Storage blocked: the choice lasts until reload.
  }
  apply();
}

// t("Sales on {day}", { day }). Reads `locale`, so templates re-render on toggle.
export function t(text: string, params?: Record<string, unknown>): string {
  let out = (locale.value === "fr" && FR[text]) || text;
  if (params) {
    for (const [key, value] of Object.entries(params)) out = out.split(`{${key}}`).join(String(value));
  }
  return out;
}
