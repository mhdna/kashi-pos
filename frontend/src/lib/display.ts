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

/** Currency code shown first on the calculator's total. Cycled by the
 * flip icon on the calculator itself, or set as a default from Settings. */
const storedTotalCurrency = localStorage.getItem("kashi-calculator-total-currency");
export const calculatorTotalCurrency = ref(storedTotalCurrency || "USD");

export function setCalculatorTotalCurrency(code: string) {
  calculatorTotalCurrency.value = code;
  localStorage.setItem("kashi-calculator-total-currency", code);
}
