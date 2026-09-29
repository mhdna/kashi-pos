<template>
  <v-divider class="mx-4" />
  <div class="d-flex align-end ga-5 w-100 px-4 py-3 action-bar-row">
    <!--
    <v-card class="checkout-total-summary pa-3" variant="outlined" rounded="lg">
      <div class="checkout-total-line">
        <span class="checkout-total-label text-medium-emphasis">Total USD</span>
        <span class="checkout-total-value">{{ totalSummary.first }}</span>
      </div>
      <div class="checkout-total-line mt-2">
        <span class="checkout-total-label text-medium-emphasis">Equivalent LBP</span>
        <span class="checkout-total-value">{{ totalSummary.second }}</span>
      </div>
    </v-card>
    -->
    <div class="d-flex flex-row ga-2 hold-void-column">
      <v-btn
        height="96"
        rounded="lg"
        variant="text"
        color="grey-darken-2"
        :disabled="!hasItems"
        @click="ask('void')"
      >
        <div class="d-flex flex-column align-center">
          <v-icon icon="mdi-close-circle-outline" size="28" class="mb-1" />
          Cancel
        </div>
      </v-btn>
      <v-btn
        height="96"
        rounded="lg"
        variant="text"
        color="grey-darken-2"
        :disabled="!hasItems || restricted"
        @click="ask('hold')"
      >
        <div class="d-flex flex-column align-center">
          <v-icon icon="mdi-pause" size="28" class="mb-1" />
          Hold
        </div>
      </v-btn>
    </div>
    <v-btn
      class="checkout-button d-flex justify-center align-center font-weight-bold"
      height="96"
      :color="checkoutColor"
      rounded="lg"
      variant="outlined"
      :style="checkoutButtonStyle"
      @click="$emit('checkout')"
      :disabled="!canCheckout"
    >
      <div class="checkout-content d-flex flex-column align-center">
        <v-icon icon="mdi-cash" size="34" class="mb-1" />
        {{ checkoutLabel }}
      </div>
    </v-btn>
  </div>

  <v-dialog v-model="dialog" max-width="350">
    <v-card>
      <v-card-title>Confirm {{ action === "hold" ? "hold" : "cancel" }}</v-card-title>
      <v-card-text>
        <template v-if="action === 'hold'"> Save the current invoice to hold? </template>
        <template v-else>
          <template v-if="mode === 'exchange'">
            Cancel the exchange?
          </template>
          <template v-else-if="mode === 'settlement'">
            Cancel the settlement?
          </template>
          <template v-else>
            Cancel the current invoice?
          </template>
        </template>
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
import { ref, computed } from "vue";

const emit = defineEmits<{ hold: []; void: []; cancelMode: []; checkout: [] }>();
const props = defineProps<{
  total: number;
  totalSummary: { first: string; second: string };
  canCheckout: boolean;
  hasItems: boolean;
  mode?: "sale" | "exchange" | "settlement";
}>();

const mode = computed(() => props.mode ?? "sale");
// Both exchange and settlement mode lock the client and disable
// hold - only the calculator and the mode-specific checkout action
// are active. Cancel is available in all modes.
const restricted = computed(() => mode.value !== "sale");
const checkoutLabel = computed(() =>
  mode.value === "exchange" ? "Exchange" : mode.value === "settlement" ? "Change Settlement" : "Check Out",
);
const checkoutColor = computed(() => (mode.value === "exchange" ? "orange-darken-3" : "green-darken-3"));
const checkoutButtonStyle = computed(() =>
  mode.value === "exchange"
    ? "background-color: transparent !important; border-color: rgba(255, 152, 0, 0.5) !important"
    : "background-color: transparent !important; border-color: #2e7d32 !important; border-width: 2px !important",
);

const dialog = ref(false);
const action = ref<"hold" | "void">("hold");

function ask(act: "hold" | "void") {
  action.value = act;
  dialog.value = true;
}
function doConfirm() {
  if (action.value === "hold") emit("hold");
  else if (mode.value !== "sale") emit("cancelMode");
  else emit("void");
  dialog.value = false;
}
</script>

<style scoped>
.action-bar-row {
  flex-shrink: 0;
}

.hold-void-column {
  flex: 0.8 1 0;
  min-width: 0;
}

.hold-void-column > .v-btn {
  flex: 1 1 0;
  min-width: 0;
}

.checkout-total-summary {
  border-color: rgba(var(--v-theme-on-surface), 0.18);
  font-size: 1rem;
  flex: 1 1 0;
  min-width: 0;
}

.checkout-button {
  flex: 1.4 1 0;
  min-width: 0;
}

.checkout-content {
  font-size: 1rem;
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
</style>
