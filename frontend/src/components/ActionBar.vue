<template>
  <ClientDialog v-model="clientDialog" @select="onClientSelect" />
  <div class="action-bar d-flex align-center ga-2 w-100 px-3 pb-3">
    <v-card
      class="client-card px-3 py-2 text-center"
      :class="{ 'missing-selection-pulse': pulseClient }"
      color="surface-light"
      elevation="0"
      height="88"
      rounded="lg"
      @click="openClientDialog"
    >
      <template v-if="clientSelected">
        <InitialsAvatar :name="client.name" color="grey-darken-1" :size="30" />
        <div class="text-body-2 font-weight-bold mt-1">{{ client.name }}</div>
        <div class="text-caption text-medium-emphasis">
          {{ formatLebanesePhone(client.phone) || "Walk-in customer" }}
        </div>
      </template>
      <template v-else>
        <InitialsAvatar name="?" color="grey" :size="30" />
        <div class="text-body-2 font-weight-bold mt-1">Select client</div>
      </template>
    </v-card>
    <v-btn
      class="action-button d-flex justify-center align-center font-weight-bold"
      height="88"
      rounded="lg"
      text="Void"
      variant="outlined"
      style="color: rgba(var(--v-theme-on-surface), 0.7); border-color: rgba(var(--v-theme-on-surface), 0.18) !important"
      @click="ask('void')"
      :disabled="!hasItems || exchangeMode"
    >
      <template #prepend>
        <v-icon icon="mdi-circle" size="20" class="me-2" />
      </template>
    </v-btn>
    <v-btn
      class="action-button d-flex justify-center align-center font-weight-bold"
      height="88"
      rounded="lg"
      text="Hold"
      variant="outlined"
      style="color: rgba(var(--v-theme-on-surface), 0.7); border-color: rgba(var(--v-theme-on-surface), 0.18) !important"
      @click="ask('hold')"
      :disabled="!hasItems || exchangeMode"
    >
      <template #prepend>
        <v-icon icon="mdi-pause" size="25" class="me-2" />
      </template>
    </v-btn>
    <v-btn
      class="action-button checkout-button d-flex justify-center align-center font-weight-bold"
      height="88"
      color="green-darken-3"
      rounded="lg"
      text="Check Out"
      variant="outlined"
      style="background-color: rgba(var(--v-theme-success), 0.24); border-color: rgba(var(--v-theme-success), 0.3) !important"
      @click="$emit('checkout')"
      :disabled="!hasItems || !canCheckout"
    >
      <template #prepend>
        <v-icon icon="mdi-cash" size="25" class="me-2" />
      </template>
    </v-btn>
  </div>

  <v-dialog v-model="dialog" max-width="350">
    <v-card>
      <v-card-title>Confirm {{ action }}</v-card-title>
      <v-card-text>
        {{
          action === "hold"
            ? "Save the current invoice to hold?"
            : "Void the current invoice? This cannot be undone."
        }}
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="dialog = false">Cancel</v-btn>
        <v-btn
          :color="action === 'hold' ? 'blue-grey-darken-2' : 'brown-darken-2'"
          variant="flat"
          @click="doConfirm"
          >Confirm</v-btn
        >
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script setup lang="ts">
import { ref, reactive, computed } from "vue";
import ClientDialog from "./ClientDialog.vue";
import InitialsAvatar from "./InitialsAvatar.vue";
import {
  anonymousClient,
  formatLoyaltyPoints,
  toClient,
  type Client,
} from "@/lib/client";
import { formatLebanesePhone } from "@/lib/phone";

const emit = defineEmits<{ hold: []; void: []; checkout: []; account: []; coupon: [] }>();
const props = defineProps<{
  total: number;
  canCheckout: boolean;
  hasItems: boolean;
  pulseClient?: boolean;
  exchangeMode?: boolean;
}>();

const dialog = ref(false);
const action = ref<"hold" | "void">("hold");

function ask(act: "hold" | "void") {
  action.value = act;
  dialog.value = true;
}
function doConfirm() {
  if (action.value === "hold") emit("hold");
  else emit("void");
  dialog.value = false;
}

const clientDialog = ref(false);
const client = reactive<Client>(anonymousClient());
const clientSelected = ref(false);
function openClientDialog() {
  if (!props.exchangeMode) clientDialog.value = true;
}

function onClientSelect(selected: Client) {
  Object.assign(client, toClient(selected));
  clientSelected.value = true;
}

function setClient(selected: Client) {
  Object.assign(client, toClient(selected));
  clientSelected.value = true;
}

function resetClient() {
  Object.assign(client, anonymousClient());
  clientSelected.value = false;
}

defineExpose({
  clientId: computed(() => client.id),
  client: computed(() => toClient(client)),
  clientSelected,
  setClient,
  resetClient,
});
</script>

<style scoped>
.client-card {
  flex: 1 1 0;
  width: auto !important;
  min-width: 100px;
  height: 88px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  border: 1px solid rgba(var(--v-border-color), 0.08);
  cursor: pointer;
  transition:
    background-color 120ms ease,
    border-color 120ms ease;
}

.action-button {
  flex: 1 1 0;
  min-width: 100px;
}

.checkout-button {
  flex: 1 1 0;
}

.client-card:hover {
  border-color: rgba(var(--v-theme-primary), 0.28);
  background: rgba(var(--v-theme-primary), 0.04);
}

.missing-selection-pulse {
  animation: missing-selection-pulse 700ms ease-in-out 2;
  border-color: rgb(var(--v-theme-warning));
}

@keyframes missing-selection-pulse {
  0%,
  100% {
    box-shadow: 0 0 0 0 rgba(var(--v-theme-warning), 0);
  }
  50% {
    box-shadow: 0 0 0 8px rgba(var(--v-theme-warning), 0.32);
  }
}
</style>
