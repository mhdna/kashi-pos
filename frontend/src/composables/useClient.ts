import { reactive, ref } from "vue";
import { anonymousClient, toClient, type Client } from "@/lib/client";

const client = reactive<Client>(anonymousClient());
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
  clientSelected.value = false;
}

export function useClient() {
  return { client, clientSelected, pulseClient, pulseMissingClient, setClient, resetClient };
}
