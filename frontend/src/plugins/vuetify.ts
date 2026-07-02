import { createVuetify } from "vuetify";
import "vuetify/styles";
import { aliases, mdi } from "../lib/icons";

export default createVuetify({
  theme: {
    defaultTheme: "light",
    variations: false,
  },
  icons: {
    defaultSet: "mdi",
    aliases,
    sets: { mdi },
  },
  defaults: {
    global: {
      transition: false,
    },
    VBtn: { ripple: false },
    VListItem: { ripple: false },
    VTextField: { ripple: false },
    VSelect: { ripple: false },
    VNavigationDrawer: { ripple: false },
    VCard: { ripple: false },
    VMenu: { ripple: false },
    VTabs: { ripple: false },
    VCheckbox: { ripple: false },
    VRadio: { ripple: false },
    VSwitch: { ripple: false },
  },
});
