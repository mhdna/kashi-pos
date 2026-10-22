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
polyfillCountryFlagEmojis("Twemoji Country Flags", flagFontUrl);

const app = createApp(App);

registerPlugins(app);

app.mount("#app");
