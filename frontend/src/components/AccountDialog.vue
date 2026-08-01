<template>
  <v-dialog v-model="dialog" max-width="520">
    <v-card>
      <v-card-title class="d-flex align-center">
        <v-icon icon="mdi-account-credit-card-outline" class="me-2" />
        Save sale on account
      </v-card-title>
      <v-card-text>
        <div class="text-body-2 text-medium-emphasis mb-3">
          Choose the customer who will pay later. The invoice total will be added to their account.
        </div>
        <v-text-field
          v-model="search"
          autofocus
          clearable
          density="compact"
          hide-details
          label="Search customers"
          prepend-inner-icon="mdi-magnify"
          variant="outlined"
        />
        <v-list v-if="filteredClients.length" class="mt-2 account-client-list" lines="two">
          <v-list-item
            v-for="client in filteredClients"
            :key="client.id"
            :title="client.name"
            :subtitle="client.phone || 'No phone number'"
            prepend-icon="mdi-account-outline"
            @click="selectClient(client)"
          >
            <template #append>
              <v-icon icon="mdi-chevron-right" />
            </template>
          </v-list-item>
        </v-list>
        <div v-else class="text-center text-medium-emphasis py-5">
          No existing customer matches this search.
        </div>
      </v-card-text>
      <v-divider />
      <v-card-actions>
        <v-btn prepend-icon="mdi-account-plus" color="primary" variant="tonal" @click="openNewClientDialog">
          New customer
        </v-btn>
        <v-spacer />
        <v-btn variant="text" @click="dialog = false">Cancel</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
  <ClientDialog v-model="newClientDialog" @select="selectClient" />
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from "vue";
import { SearchClients } from "../../wailsjs/go/main/App";
import ClientDialog from "@/components/ClientDialog.vue";
import { toClient, type Client } from "@/lib/client";

const emit = defineEmits<{ select: [client: Client] }>();
const dialog = defineModel<boolean>({ default: false });
const newClientDialog = ref(false);
const search = ref("");
const clients = ref<Client[]>([]);
let searchTimer: ReturnType<typeof setTimeout> | undefined;
let latestRequest = 0;

const filteredClients = computed(() => {
  const query = search.value.trim().toLowerCase();
  if (!query) return clients.value;
  return clients.value.filter((client) =>
    [client.name, client.phone].some((value) => value.toLowerCase().includes(query)),
  );
});

watch(dialog, (open) => {
  clearTimeout(searchTimer);
  latestRequest++;
  if (!open) return;
  search.value = "";
  void loadClients("");
});

watch(search, (value) => {
  if (!dialog.value) return;
  clearTimeout(searchTimer);
  searchTimer = setTimeout(() => void loadClients(value), 200);
});

async function loadClients(query: string) {
  const request = ++latestRequest;
  try {
    const result = await SearchClients(query.trim(), 0);
    if (request !== latestRequest || !dialog.value) return;
    clients.value = result
      .map((client: any) => toClient(client))
      .filter((client) => client.id > 0 && !/walk[\s-]?in|anonymous/i.test(client.name));
  } catch {
    if (request !== latestRequest) return;
    clients.value = [];
  }
}

function openNewClientDialog() {
  // Do not stack two v-dialog focus traps. Render the child after the parent
  // has closed so Vuetify only has one active modal overlay.
  dialog.value = false;
  void nextTick(() => {
    newClientDialog.value = true;
  });
}

function selectClient(client: Client) {
  emit("select", toClient(client));
  newClientDialog.value = false;
  dialog.value = false;
}
</script>

<style scoped>
.account-client-list {
  max-height: 280px;
  overflow-y: auto;
}
</style>
