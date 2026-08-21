<template>
  <ClientDialog v-model="clientDialog" @select="onClientSelect" />
  <div class="d-flex flex-column w-100 h-100 px-4">
    <div class="text-caption text-medium-emphasis text-center mt-2">Salesperson</div>
    <SalespersonsDialog />
    <div class="text-caption text-medium-emphasis text-center mt-2">Client</div>
    <v-card
      class="client-card px-4 py-4 my-2 text-center"
      :class="{ 'missing-selection-pulse': pulseClient }"
      color="surface-light"
      elevation="0"
      min-height="100px"
      rounded="lg"
      width="100%"
      @click="openClientDialog"
    >
      <template v-if="clientSelected">
        <InitialsAvatar :name="client.name" color="grey-darken-1" />
        <div class="text-subtitle-1 font-weight-bold mt-2">{{ client.name }}</div>
        <div class="text-caption text-medium-emphasis mt-1">
          {{ formatLebanesePhone(client.phone) || "Walk-in customer" }}
        </div>
      </template>
      <template v-else>
        <InitialsAvatar name="?" color="grey" />
        <div class="text-subtitle-1 font-weight-bold mt-2">Select client</div>
      </template>
    </v-card>
    <div class="mt-auto pb-2">
    <!--
    <v-btn
      class="d-flex justify-center align-center mb-2 font-weight-bold"
      height="77"
      rounded="lg"
      text="Void"
      variant="outlined"
      style="color: rgba(var(--v-theme-on-surface), 0.7); border-color: rgba(var(--v-theme-on-surface), 0.18) !important"
      width="100%"
      @click="ask('void')"
      :disabled="!hasItems || exchangeMode"
    >
      <template #prepend>
        <v-icon icon="mdi-circle" size="20" class="me-2" />
      </template>
    </v-btn>
    -->
    <div class="d-flex flex-column ga-2 mb-2">
      <v-btn class="checkout-action-button" variant="tonal" :disabled="!hasItems || exchangeMode" @click="emit('account')">
        <v-icon icon="mdi-account" size="18" class="me-1" />
        On Account
      </v-btn>
      <v-btn class="checkout-action-button" variant="tonal" :disabled="exchangeMode" @click="emit('coupon')">
        <v-icon icon="mdi-sale-outline" size="18" class="me-1" />
        Apply Coupon
      </v-btn>
    </div>
    <v-card class="checkout-total-summary pa-3 mb-2" variant="outlined" rounded="lg">
      <div class="checkout-total-line">
        <span class="checkout-total-label text-medium-emphasis">Total USD</span>
        <span class="checkout-total-value">{{ totalSummary.usd }}</span>
      </div>
      <div class="checkout-total-line mt-2">
        <span class="checkout-total-label text-medium-emphasis">Total LBP</span>
        <span class="checkout-total-value">{{ totalSummary.lbp }}</span>
      </div>
    </v-card>
    <v-btn
      class="d-flex justify-center align-center font-weight-bold"
      height="96"
      color="green-darken-3"
      rounded="lg"
      text="Check Out"
      variant="outlined"
      style="background-color: rgba(var(--v-theme-success), 0.24); border-color: rgba(var(--v-theme-success), 0.3) !important"
      width="100%"
      @click="$emit('checkout')"
      :disabled="!hasItems || exchangeMode"
    >
      <template #prepend>
        <v-icon icon="mdi-cash" size="25" class="me-2" />
      </template>
    </v-btn>
    </div>
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
import SalespersonsDialog from "./SalespersonsDialog.vue";
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
  totalSummary: { usd: string; lbp: string };
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
.checkout-total-summary {
  border-color: rgba(var(--v-theme-on-surface), 0.18);
  font-size: 1rem;
}

.checkout-action-button {
  width: 100%;
}

.checkout-total-line {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  line-height: 1.15;
}

.checkout-total-label {
  font-size: 0.85rem;
}

.checkout-total-value {
  color: rgb(var(--v-theme-on-surface));
  font-weight: 600;
  font-size: 1.05rem;
  line-height: 1.2;
}

.client-card {
  min-height: 96px;
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
