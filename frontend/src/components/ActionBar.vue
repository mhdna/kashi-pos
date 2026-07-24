<template>
  <ClientDialog v-model="clientDialog" @select="onClientSelect" />
  <div class="d-flex flex-column w-100 h-100 px-4">
    <v-card
      class="ma-0 d-flex justify-center text-title-medium my-2 w-100 flex-column"
      color="gray"
      rounded="lg"
      style="flex: 3"
      title="Client"
    >
      <template #prepend>
        <v-icon class="me-2" icon="mdi-account" />
      </template>
      <v-card-text class="d-flex flex-column align-center">
        <v-card
          class="px-3 py-1 ma-4 text-center"
          color="surface-light"
          elevation="0"
          rounded="lg"
          width="100%"
          @click="clientDialog = true"
        >
          <v-avatar class="font-weight-bold" color="yellow-darken-3" size="42">{{
            client.initials
          }}</v-avatar>
          <div>{{ client.name }}</div>
          <div v-if="client.phone">{{ client.phone }}</div>
        </v-card>
        <div class="text-center">
          <div>{{ client.tier }} Tier</div>
          <div>Has: {{ client.totalLoyaltyPoints }} Points</div>
          <div>Earns: {{ earns }} Points</div>
        </div>
      </v-card-text>
    </v-card>
    <v-btn
      class="d-flex justify-center align-center my-2 flex-grow-1 font-weight-bold"
      rounded="lg"
      text="Void"
      color="yellow-darken-4"
      variant="tonal"
      width="100%"
      @click="ask('void')"
      :disabled="!hasItems"
    >
      <template #prepend>
        <v-icon icon="mdi-circle" size="20" class="me-2" />
      </template>
    </v-btn>
    <v-btn
      class="d-flex justify-center align-center my-2 flex-grow-1 font-weight-bold"
      color="red-darken-4"
      rounded="lg"
      text="Hold"
      variant="tonal"
      width="100%"
      @click="ask('hold')"
      :disabled="!hasItems"
    >
      <template #prepend>
        <v-icon icon="mdi-pause" size="25" class="me-2" />
      </template>
    </v-btn>
    <v-btn
      class="d-flex justify-center align-center my-2 flex-grow-1 font-weight-bold"
      color="green-darken-4"
      rounded="lg"
      text="Check Out"
      variant="tonal"
      width="100%"
      @click="$emit('checkout')"
      :disabled="!canCheckout"
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
          :color="action === 'hold' ? 'red-darken-4' : 'yellow-darken-4'"
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
import ClientDialog, { type Client } from "./ClientDialog.vue";

const emit = defineEmits<{ hold: []; void: []; checkout: [] }>();

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

const props = defineProps<{ total: number; canCheckout: boolean; hasItems: boolean }>();

const clientDialog = ref(false);
const client = reactive<Client>({
  id: 0,
  name: "Anonymous",
  phone: "",
  initials: "AN",
  tier: "None",
  points: 0,
  totalLoyaltyPoints: 0,
  validLoyaltyPoints: 0,
});

const earns = computed(() => Math.floor(props.total));

function onClientSelect(selected: Client) {
  Object.assign(client, selected);
}
</script>
