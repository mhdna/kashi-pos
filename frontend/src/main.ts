/**
 * main.ts
 *
 * Bootstraps Vuetify and other plugins then mounts the App`
 */

// Composables
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

const app = createApp(App);

registerPlugins(app);

app.mount("#app");
