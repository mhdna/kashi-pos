import { createVuetify } from "vuetify";
import "vuetify/styles";
import { aliases, mdi } from "../lib/icons";

export default createVuetify({
  theme: {
    defaultTheme: "light",
  },
  icons: {
    defaultSet: "mdi",
    aliases,
    sets: { mdi },
  },
});
