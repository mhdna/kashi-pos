/**
 * main.ts
 *
 * Bootstraps Vuetify and other plugins then mounts the App`
 */

// Composables
import { createApp } from "vue";

// Plugins
import { registerPlugins } from "@/plugins";

// Components
import App from "./App.vue";
import "./styles/settings.scss";
import "./styles/compact.scss";

const app = createApp(App);

registerPlugins(app);

app.mount("#app");
