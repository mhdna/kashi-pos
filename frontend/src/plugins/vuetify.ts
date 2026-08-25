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
          primary: "#08306b",
        },
      },
      dark: {
        colors: {
          primary: "#08306b",
          background: "#000000",
          surface: "#000000",
          "surface-variant": "#000000",
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
