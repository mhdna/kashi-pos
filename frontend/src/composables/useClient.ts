import { reactive, ref } from "vue";
import { anonymousClient, toClient, type Client } from "@/lib/client";

const client = reactive<Client>(anonymousClient());
// Defaults to true (client selection required) so behavior matches every
// install that predates this setting, until the real value loads from
// GetAppSettings via setClientRequired.
const clientRequired = ref(true);
const clientSelected = ref(false);
const pulseClient = ref(false);

let pulseTimer: ReturnType<typeof setTimeout> | undefined;
function pulseMissingClient() {
  pulseClient.value = false;
  clearTimeout(pulseTimer);
  requestAnimationFrame(() => {
    pulseClient.value = true;
    pulseTimer = setTimeout(() => {
      pulseClient.value = false;
    }, 1500);
  });
}

function setClient(selected: Client) {
  Object.assign(client, toClient(selected));
  clientSelected.value = true;
}

function resetClient() {
  Object.assign(client, anonymousClient());
  // When client selection isn't required, a reset already counts as a
  // client being "selected" - Anonymous - so checkout doesn't pulse-warn
  // and the cashier isn't forced through the picker.
  clientSelected.value = !clientRequired.value;
}

function setClientRequired(required: boolean) {
  clientRequired.value = required;
  if (!required && !clientSelected.value) {
    clientSelected.value = true;
  }
}

export function useClient() {
  return {
    client,
    clientRequired,
    clientSelected,
    pulseClient,
    pulseMissingClient,
    setClient,
    resetClient,
    setClientRequired,
  };
}
