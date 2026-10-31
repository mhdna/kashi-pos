import { ref, readonly } from "vue";

type ThemeName = "light" | "dark";

// Colours as "r,g,b" triplets so CSS can write rgb(var(--k-theme-primary)) and
// rgba(var(--k-theme-primary), .12), exactly as it did with Vuetify.
const themes: Record<ThemeName, Record<string, string>> = {
  light: {
    background: "255,255,255",
    surface: "255,255,255",
    "surface-bright": "255,255,255",
    "surface-light": "238,238,238",
    "surface-variant": "66,66,66",
    "on-surface-variant": "238,238,238",
    primary: "13,71,161",
    secondary: "72,169,166",
    error: "176,0,32",
    info: "33,150,243",
    success: "76,175,80",
    warning: "251,140,0",
    "on-background": "0,0,0",
    "on-surface": "0,0,0",
    "on-surface-light": "0,0,0",
    "on-primary": "255,255,255",
    "on-secondary": "255,255,255",
    "on-error": "255,255,255",
    "on-info": "255,255,255",
    "on-success": "255,255,255",
    "on-warning": "255,255,255",
  },
  dark: {
    background: "18,18,18",
    surface: "33,33,33",
    "surface-bright": "204,191,214",
    "surface-light": "66,66,66",
    "surface-variant": "200,200,200",
    "on-surface-variant": "0,0,0",
    primary: "13,71,161",
    secondary: "84,182,178",
    error: "207,102,121",
    info: "33,150,243",
    success: "76,175,80",
    warning: "251,140,0",
    "on-background": "255,255,255",
    "on-surface": "255,255,255",
    "on-surface-light": "255,255,255",
    "on-primary": "255,255,255",
    "on-secondary": "255,255,255",
    "on-error": "255,255,255",
    "on-info": "255,255,255",
    "on-success": "255,255,255",
    "on-warning": "255,255,255",
  },
};

const levels: Record<ThemeName, Record<string, string>> = {
  light: {
    "border-color": "0,0,0",
    "shadow-color": "0,0,0",
    "border-opacity": "0.12",
    "high-emphasis-opacity": "0.87",
    "medium-emphasis-opacity": "0.6",
    "disabled-opacity": "0.38",
    "activated-opacity": "0.12",
    "hover-opacity": "0.04",
  },
  dark: {
    "border-color": "255,255,255",
    "shadow-color": "0,0,0",
    "border-opacity": "0.12",
    "high-emphasis-opacity": "1",
    "medium-emphasis-opacity": "0.7",
    "disabled-opacity": "0.5",
    "activated-opacity": "0.12",
    "hover-opacity": "0.04",
  },
};

// Every theme variable name, so switching themes can overwrite them in one go.
export const themeColorNames = Object.keys(themes.light);

const current = ref<ThemeName>("light");

export function applyTheme(name: ThemeName) {
  const root = document.documentElement;
  for (const [key, value] of Object.entries(themes[name])) root.style.setProperty(`--k-theme-${key}`, value);
  for (const [key, value] of Object.entries(levels[name])) root.style.setProperty(`--k-${key}`, value);
  root.classList.remove("k-theme--light", "k-theme--dark");
  root.classList.add(`k-theme--${name}`);
  current.value = name;
}

// Same shape the app used with Vuetify's useTheme: theme.name.value, theme.change(name).
export function useTheme() {
  return { name: readonly(current), change: applyTheme };
}

applyTheme("light");
