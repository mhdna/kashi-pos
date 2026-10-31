<template>
  <k-dialog v-model="dialog" max-width="520">
    <k-card>
      <k-card-title class="d-flex align-center">
        <k-icon icon="mdi-account-credit-card-outline" class="me-2" />
        Save sale on account
      </k-card-title>
      <k-card-text>
        <div class="text-body-2 text-medium-emphasis mb-3">
          Choose the customer who will pay later. The invoice total will be added to their account.
        </div>
        <k-text-field
          v-model="search"
          autofocus
          clearable
          density="compact"
          hide-details
          placeholder="Search customers"
          prepend-inner-icon="mdi-magnify"
          variant="outlined"
        />
        <k-list v-if="filteredClients.length" class="mt-2 account-client-list" lines="two">
          <k-list-item
            v-for="client in filteredClients"
            :key="client.id"
            :title="client.name"
            :subtitle="formatLebanesePhone(client.phone) || 'No phone number'"
            prepend-icon="mdi-account-outline"
            @click="selectClient(client)"
          >
            <template #append>
              <k-icon icon="mdi-chevron-right" />
            </template>
          </k-list-item>
        </k-list>
        <div v-else class="text-center text-medium-emphasis py-5">
          No existing customer matches this search.
        </div>
      </k-card-text>
      <k-divider />
      <k-card-actions>
        <k-btn prepend-icon="mdi-account-plus" color="primary" variant="tonal" @click="openNewClientDialog">
          New customer
        </k-btn>
        <k-spacer />
        <k-btn variant="text" @click="dialog = false">Cancel</k-btn>
      </k-card-actions>
    </k-card>
  </k-dialog>
  <ClientDialog v-model="newClientDialog" @select="selectClient" />
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from "vue";
import { SearchClients } from "../../wailsjs/go/main/App";
import ClientDialog from "@/components/ClientDialog.vue";
import { toClient, type Client } from "@/lib/client";
import { formatLebanesePhone } from "@/lib/phone";

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
  // has closed so only one modal overlay is active.
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
