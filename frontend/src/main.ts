/**
 * main.ts
 *
 * Bootstraps Vuetify and other plugins then mounts the App`
 */

// Composables
import { polyfillCountryFlagEmojis } from "country-flag-emoji-polyfill";
import flagFontUrl from "@/assets/TwemojiCountryFlags.woff2?url";
import { createApp } from "vue";
import "@fontsource/roboto/400.css";
import "@fontsource/roboto/500.css";
import "@fontsource/roboto/700.css";

// Plugins
import { registerPlugins } from "@/plugins";

// Components
import App from "./App.vue";
import "./styles/typography.scss";
import "./styles/settings.scss";
import "./styles/compact.scss";

// Windows draws flag emoji as two letters ("LB"); this loads a bundled flag
// font only on platforms that can't render them, so macOS keeps its own. The
// font file ships with the app (Twemoji flags, MIT) rather than the
// polyfill's default CDN URL, so it works on a till with no internet.
// It's otherwise only fetched when a flag is first drawn, which made the
// client dialog's first open wait on it; load it now instead.
if (polyfillCountryFlagEmojis("Twemoji Country Flags", flagFontUrl)) {
  void document.fonts.load('1em "Twemoji Country Flags"', "\u{1F1F1}\u{1F1E7}").catch(() => undefined);
}

const app = createApp(App);

registerPlugins(app);

app.mount("#app");
