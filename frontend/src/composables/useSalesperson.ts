import { ref } from "vue";

export interface Salesperson {
  id: number;
  fullName: string;
}

const selectedSalesperson = ref<Salesperson | null>(null);
const pulseMissingSalesperson = ref(false);

let pulseTimer: ReturnType<typeof setTimeout> | undefined;
function pulseSalesperson() {
  pulseMissingSalesperson.value = false;
  clearTimeout(pulseTimer);
  requestAnimationFrame(() => {
    pulseMissingSalesperson.value = true;
    pulseTimer = setTimeout(() => {
      pulseMissingSalesperson.value = false;
    }, 1500);
  });
}

export function useSalesperson() {
  return { selectedSalesperson, pulseMissingSalesperson, pulseSalesperson };
}
