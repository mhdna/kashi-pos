// Types
import type { App } from "vue";
import { createPinia } from "pinia";
/**
 * plugins/index.ts
 *
 * Automatically included in `./src/main.ts`
 */

import router from "../router";

// Plugins
import ui from "../ui";

export function registerPlugins(app: App) {
  app.use(ui);
  app.use(createPinia());
  app.use(router);
}
