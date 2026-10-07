import { ref } from "vue";

export interface Salesperson {
  id: number;
  fullName: string;
  // Set in kashi; empty for a till-only salesperson (see lib/salespersonColor).
  color?: string;
}

const selectedSalesperson = ref<Salesperson | null>(null);
const pulseMissingSalesperson = ref(false);
// When on, the salesperson selection is cleared for every new invoice so
// the cashier must pick one each time. Off (the default) carries the
// previous invoice's salesperson forward. Loaded from GetAppSettings via
// setSalespersonPerInvoiceRequired.
const salespersonPerInvoiceRequired = ref(false);

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

function setSalespersonPerInvoiceRequired(required: boolean) {
  salespersonPerInvoiceRequired.value = required;
}

export function useSalesperson() {
  return {
    selectedSalesperson,
    pulseMissingSalesperson,
    pulseSalesperson,
    salespersonPerInvoiceRequired,
    setSalespersonPerInvoiceRequired,
  };
}
