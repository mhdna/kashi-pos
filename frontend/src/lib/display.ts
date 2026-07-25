import { ref } from "vue";

export const isCompactBuild = import.meta.env.VITE_KASHI_COMPACT === "1";

const storedAnimations = localStorage.getItem("kashi-animations-enabled");
export const animationsEnabled = ref(
  storedAnimations === null ? !isCompactBuild : storedAnimations === "true",
);

export function setAnimationsEnabled(enabled: boolean) {
  animationsEnabled.value = enabled;
  localStorage.setItem("kashi-animations-enabled", String(enabled));
}
