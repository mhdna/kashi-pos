<template>
  <ClientDialog v-model="clientDialog" @select="onClientSelect" />
  <v-sheet class="d-flex flex-column w-100" style="height: 91vh">
    <v-card
      class="ma-0 d-flex justify-center text-title-medium my-2 w-100 flex-column"
      color="gray" rounded="true" style="flex: 3" title="Client"
    >
      <template #prepend>
        <v-icon class="me-2" icon="mdi-account" />
      </template>
      <v-card-text class="d-flex flex-column align-center">
        <v-card
          class="px-3 py-1 ma-4 text-center" color="surface-light" elevation="0" rounded="lg" width="100%"
          @click="clientDialog = true"
        >
          <v-avatar class="font-weight-bold" color="yellow-darken-3" size="42">{{ client.initials }}</v-avatar>
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
    <v-btn class="d-flex justify-center align-center my-2 flex-grow-1 font-weight-bold"
      style="border-width: 1px" rounded="lg" color="gray" text="Void" variant="outlined" width="100%"
      @click="$emit('void')"
    >
      <template #prepend>
        <v-icon icon="mdi-circle" size="20" class="me-2" />
      </template>
    </v-btn>
    <v-btn class="d-flex justify-center align-center my-2 flex-grow-1 font-weight-bold"
      color="red-darken-4" style="border-width: 1px" rounded="lg" text="Hold" variant="outlined" width="100%"
      @click="$emit('hold')"
    >
      <template #prepend>
        <v-icon icon="mdi-pause" size="25" class="me-2" />
      </template>
    </v-btn>
    <v-btn class="d-flex justify-center align-center my-2 flex-grow-1 font-weight-bold"
      color="green-darken-4" style="border-width: 1px" rounded="lg" text="Check Out" variant="outlined" width="100%"
      @click="$emit('checkout')"
    >
      <template #prepend>
        <v-icon icon="mdi-cash" size="25" class="me-2" />
      </template>
    </v-btn>
  </v-sheet>
</template>

<script setup lang="ts">
import { ref, reactive, computed } from "vue";
import ClientDialog, { type Client } from "./ClientDialog.vue";

defineEmits<{ hold: []; void: []; checkout: [] }>();

const props = defineProps<{ total: number }>();

const clientDialog = ref(false);
const client = reactive<Client>({
  id: 0, name: "Anonymous", phone: "", initials: "AN", tier: "None",
  points: 0, totalLoyaltyPoints: 0, validLoyaltyPoints: 0,
});

const earns = computed(() => Math.floor(props.total));

function onClientSelect(selected: Client) {
  Object.assign(client, selected);
}
</script>
