import { createVuetify } from "vuetify";
import "vuetify/styles";
import { aliases, mdi } from "../lib/icons";

export default createVuetify({
  theme: {
    defaultTheme: "light",
    themes: {
      light: {
        colors: {
          primary: "#4a352f",
        },
      },
      dark: {
        colors: {
          primary: "#cee5cb",
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
