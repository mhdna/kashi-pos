<template>
  <ClientDialog v-model="clientDialog" @select="onClientSelect" />
  <div class="d-flex flex-column w-100 h-100 px-4">
    <v-card
      class="ma-0 d-flex justify-center text-title-medium my-2 w-100 flex-column"
      color="gray"
      rounded="lg"
      style="flex: 0 0 300px; height: 300px; min-height: 300px"
      title="Client"
    >
      <template #prepend>
        <v-icon class="me-2" icon="mdi-account" />
      </template>
      <v-card-text class="d-flex flex-column align-center pa-3">
        <v-card
          class="client-card px-4 py-4 text-center"
          :class="{ 'missing-selection-pulse': pulseClient }"
          color="surface-light"
          elevation="0"
          min-height="100px"
          rounded="lg"
          width="100%"
          @click="openClientDialog"
        >
          <InitialsAvatar :name="clientSelected ? client.name : '?'" color="primary" :size="48" />
          <div class="text-subtitle-1 font-weight-bold mt-2">
            {{ clientSelected ? client.name : "Choose a client" }}
          </div>
          <div class="text-caption text-medium-emphasis mt-1">
            {{ clientSelected ? client.phone || "Walk-in customer" : "Required before checkout" }}
          </div>
        </v-card>
        <div v-if="client.id !== 0" class="text-center">
          <v-icon
            :color="tierColor"
            class="mb-4"
            icon="mdi-cards-diamond"
            size="44"
            :title="tierLabel"
          />
          <div>Has: {{ client.totalLoyaltyPoints }} Points</div>
          <div>Earns: {{ earns }} Points</div>
        </div>
      </v-card-text>
    </v-card>
    <v-btn
      class="d-flex justify-center align-center my-2 flex-grow-1 font-weight-bold"
      rounded="lg"
      text="Void"
      variant="tonal"
      width="100%"
      @click="ask('void')"
      :disabled="!hasItems || exchangeMode"
    >
      <template #prepend>
        <v-icon icon="mdi-circle" size="20" class="me-2" />
      </template>
    </v-btn>
    <v-btn
      class="d-flex justify-center align-center my-2 flex-grow-1 font-weight-bold"
      rounded="lg"
      text="Hold"
      variant="tonal"
      width="100%"
      @click="ask('hold')"
      :disabled="!hasItems || exchangeMode"
    >
      <template #prepend>
        <v-icon icon="mdi-pause" size="25" class="me-2" />
      </template>
    </v-btn>
    <v-btn
      class="d-flex justify-center align-center my-2 flex-grow-1 font-weight-bold"
      color="primary"
      rounded="lg"
      text="Pay"
      variant="tonal"
      width="100%"
      @click="$emit('checkout')"
      :disabled="!hasItems || (!canCheckout && clientSelected && Boolean(selectedSalesperson))"
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
import { anonymousClient, toClient, type Client } from "@/lib/client";
import { useSalesperson } from "@/composables/useSalesperson";

const emit = defineEmits<{ hold: []; void: []; checkout: [] }>();
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
const { selectedSalesperson } = useSalesperson();

const earns = computed(() => Math.floor(props.total));

function openClientDialog() {
  if (!props.exchangeMode) clientDialog.value = true;
}

const tierLabel = computed(() => {
  const pts = client.totalLoyaltyPoints;
  if (pts >= 500) return "Gold";
  if (pts >= 200) return "Silver";
  if (pts >= 1) return "Bronze";
  return "None";
});
const tierColor = computed(() => {
  const pts = client.totalLoyaltyPoints;
  if (pts >= 500) return "yellow-darken-2";
  if (pts >= 200) return "grey-lighten-1";
  if (pts >= 1) return "brown";
  return "grey";
});

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
