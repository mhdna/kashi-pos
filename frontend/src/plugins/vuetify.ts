import { createVuetify } from "vuetify";
import "vuetify/styles";
import { aliases, mdi } from "../lib/icons";

export default createVuetify({
  defaults: {
    VBtn: { ripple: false },
    VCard: { ripple: false },
    VListItem: { ripple: false },
    VDialog: { transition: false },
    VMenu: { transition: false },
    VOverlay: { transition: false },
    VSnackbar: { transition: false },
    VTooltip: { transition: false },
  },
  theme: {
    defaultTheme: "light",
    themes: {
      light: {
        colors: {
          primary: "#1976D2",
        },
      },
      dark: {
        colors: {
          primary: "#1976D2",
        },
      },
    },
  },
  // TODO: add compact to mostly everything including v-row
  icons: {
    defaultSet: "mdi",
    aliases,
    sets: { mdi },
  },
});
