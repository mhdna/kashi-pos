import { setLocale } from "@/i18n";
import { GetAppSettings } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";

// Shows the screen in the till's language setting, and again whenever it
// changes (from Settings here or from kashi).
export async function applyTillLanguage(): Promise<void> {
  try {
    const language = (await GetAppSettings())?.tillLanguage;
    if (language) setLocale(String(language));
  } catch {
    // Settings unreadable: keep the language already showing.
  }
}

export function startTillLanguage(): () => void {
  void applyTillLanguage();
  return EventsOn("settings_changed", () => void applyTillLanguage());
}
