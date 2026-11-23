<template>
  <k-divider class="mx-4" />
  <div class="d-flex align-end ga-5 w-100 px-4 py-3 action-bar-row">
    <!--
    <k-card class="checkout-total-summary pa-3" variant="outlined" rounded="lg">
      <div class="checkout-total-line">
        <span class="checkout-total-label text-medium-emphasis">Total USD</span>
        <span class="checkout-total-value">{{ totalSummary.first }}</span>
      </div>
      <div class="checkout-total-line mt-2">
        <span class="checkout-total-label text-medium-emphasis">Equivalent LBP</span>
        <span class="checkout-total-value">{{ totalSummary.second }}</span>
      </div>
    </k-card>
    -->
    <div class="d-flex flex-row ga-2 hold-void-column">
      <k-btn
        height="96"
        rounded="lg"
        variant="text"
        color="grey-darken-2"
        :disabled="!hasItems"
        @click="ask('void')"
      >
        <div class="d-flex flex-column align-center">
          <k-icon icon="mdi-close-circle-outline" size="28" class="mb-1" />
          {{ t("Cancel") }}
        </div>
      </k-btn>
      <k-btn
        height="96"
        rounded="lg"
        variant="text"
        color="grey-darken-2"
        :aria-label="t('Invoice note')"
        data-test="note-open"
        @click="$emit('note')"
      >
        <div class="d-flex flex-column align-center">
          <k-icon
            :icon="hasNote ? 'mdi-check-circle' : 'mdi-note-text-outline'"
            :color="hasNote ? 'green-darken-2' : undefined"
            size="28"
            class="mb-1"
            data-test="note-icon"
          />
          {{ t("Note") }}
        </div>
      </k-btn>
      <k-btn
        height="96"
        rounded="lg"
        variant="text"
        color="grey-darken-2"
        :disabled="!hasItems || mode === 'settlement'"
        @click="ask('hold')"
      >
        <div class="d-flex flex-column align-center">
          <k-icon icon="mdi-pause" size="28" class="mb-1" />
          {{ t("Hold") }}
        </div>
      </k-btn>
    </div>
    <k-btn
      class="checkout-button d-flex justify-center align-center font-weight-bold"
      height="96"
      :color="checkoutColor"
      rounded="0"
      variant="outlined"
      :style="checkoutButtonStyle"
      :class="{ 'checkout-button--blocked': !canCheckout }"
      @click="$emit('checkout')"
    >
      <div class="checkout-content d-flex flex-column align-center">
        <k-icon icon="mdi-cash" size="34" class="mb-1" />
        {{ checkoutLabel }}
      </div>
    </k-btn>
  </div>

  <k-dialog v-model="dialog" max-width="350">
    <k-card>
      <k-card-title>{{ t("Confirm") }} {{ action === "hold" ? t("hold") : t("cancel") }}</k-card-title>
      <k-card-text>
        <template v-if="action === 'hold'"> {{ t("Save the current invoice to hold?") }} </template>
        <template v-else>
          <template v-if="mode === 'exchange'">
            {{ t("Cancel the exchange?") }}
          </template>
          <template v-else-if="mode === 'settlement'">
            {{ t("Cancel the settlement?") }}
          </template>
          <template v-else>
            {{ t("Cancel the current invoice?") }}
          </template>
        </template>
      </k-card-text>
      <k-card-actions>
        <k-spacer />
        <k-btn variant="text" @click="dialog = false">{{ t("Cancel") }}</k-btn>
        <k-btn
          color="primary"
          variant="flat"
          @click="doConfirm"
          >{{ t("Confirm") }}</k-btn
        >
      </k-card-actions>
    </k-card>
  </k-dialog>
</template>

<script setup lang="ts">
import { ref, computed } from "vue";
import { t } from "@/i18n";

const emit = defineEmits<{ hold: []; void: []; cancelMode: []; checkout: []; note: [] }>();
const props = defineProps<{
  total: number;
  totalSummary: { first: string; second: string };
  canCheckout: boolean;
  hasItems: boolean;
  mode?: "sale" | "exchange" | "settlement";
  // The sale has a note: the Note button shows it.
  hasNote?: boolean;
}>();

const mode = computed(() => props.mode ?? "sale");
// Both exchange and settlement mode lock the client. An exchange can be
// held (it comes back as the same exchange); a settlement can't. Cancel is
// available in all modes.
const checkoutLabel = computed(() =>
  mode.value === "exchange" ? t("Exchange") : mode.value === "settlement" ? t("Change Settlement") : t("Check Out"),
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

.hold-void-column > .k-btn {
  flex: 1 1 0;
  min-width: 0;
}

.checkout-total-summary {
  border-color: rgba(var(--k-theme-on-surface), 0.18);
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
  color: rgb(var(--k-theme-on-surface));
  font-weight: 600;
  font-size: 1.05rem;
  line-height: 1.2;
}
/* Not disabled: a click while blocked tells the cashier what's missing. */
.checkout-button--blocked {
  opacity: 0.45;
}
</style>
