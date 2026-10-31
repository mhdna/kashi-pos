/**
 * main.ts
 *
 * Registers the UI kit (src/ui) and other plugins, then mounts the App.
 */

// Composables
import { createApp } from "vue";
// Latin subsets only: the full weights also declare Cyrillic, Greek,
// Vietnamese, math and symbol faces the till never shows.
import "@fontsource/roboto/latin-400.css";
import "@fontsource/roboto/latin-ext-400.css";
import "@fontsource/roboto/latin-500.css";
import "@fontsource/roboto/latin-ext-500.css";
import "@fontsource/roboto/latin-700.css";
import "@fontsource/roboto/latin-ext-700.css";

// Plugins
import { registerPlugins } from "@/plugins";
import router from "@/router";
import { preventNewWindows } from "@/lib/noNewWindows";
import { preventAutofill } from "@/lib/noAutofill";

// Components
import App from "./App.vue";
import "./styles/typography.scss";
import "./styles/settings.scss";
import "./styles/compact.scss";

// The flag font (typography.scss) is otherwise only fetched when a flag is
// first drawn, which made the client dialog's first open wait on it.
void document.fonts.load('1em "Twemoji Country Flags"', "\u{1F1F1}\u{1F1E7}").catch(() => undefined);

const app = createApp(App);

registerPlugins(app);
preventNewWindows(router);
preventAutofill();

app.mount("#app");
