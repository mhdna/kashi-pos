<template>
  <v-card class="pa-4">
    <v-card-title class="d-flex align-center justify-space-between">
      Clients
      <v-text-field
        v-model="search"
        class="ms-4"
        clearable
        density="compact"
        hide-details
        label="Search"
        max-width="320"
        prepend-inner-icon="mdi-magnify"
        single-line
        variant="outlined"
      />
    </v-card-title>
    <v-data-table-server
      v-model:items-per-page="itemsPerPage"
      :headers="headers"
      :items="visibleItems"
      :items-length="totalItems"
      :loading="loading"
      density="compact"
      hover
      @update:options="loadItems"
    >
      <template #item.avatar="{ item }">
        <InitialsAvatar
          :name="item.name ?? ''"
          :color="colors[items.indexOf(item) % colors.length] ?? 'grey'"
        />
      </template>
      <template #item.totalLoyaltyPoints="{ item }">
        <span>{{ formatLoyaltyPoints(item.totalLoyaltyPoints) }}</span>
      </template>
      <template #item.validLoyaltyPoints="{ item }">
        <span>{{ formatLoyaltyPoints(item.validLoyaltyPoints) }}</span>
      </template>
      <template #item.tier="{ item }">
        <v-chip :color="loyaltyTierColor(loyaltyTier(Number(item.totalLoyaltyPoints ?? 0)))" size="small" variant="tonal">
          {{ loyaltyTier(Number(item.totalLoyaltyPoints ?? 0)) }}
        </v-chip>
      </template>
      <template #item.actions="{ item }">
        <template v-if="auth.isAdmin">
          <v-btn icon="mdi-pencil-outline" variant="text" density="comfortable" title="Edit client" @click.stop="openEdit(item)" />
          <v-btn icon="mdi-delete-outline" variant="text" density="comfortable" color="error" title="Delete client" @click.stop="openDelete(item)" />
        </template>
      </template>
    </v-data-table-server>
  </v-card>

  <v-dialog v-model="editOpen" max-width="420">
    <v-card>
      <v-card-title>Edit client</v-card-title>
      <v-card-text>
        <v-text-field v-model="editForm.name" label="Name" variant="outlined" autofocus />
        <v-text-field v-model="editForm.phone" label="Phone" variant="outlined" />
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="editOpen = false">Cancel</v-btn>
        <v-btn color="primary" :loading="saving" :disabled="!editForm.name.trim() || !editForm.phone.trim()" @click="saveEdit">Save</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-dialog v-model="deleteOpen" max-width="420">
    <v-card>
      <v-card-title>Delete client?</v-card-title>
      <v-card-text>This will delete {{ deleteTarget?.name || "this client" }}. Clients linked to invoices cannot be deleted.</v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="deleteOpen = false">Cancel</v-btn>
        <v-btn color="error" :loading="deleting" @click="deleteSelected">Delete</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-snackbar v-model="snackbar.show" timeout="3500">{{ snackbar.text }}</v-snackbar>
</template>

<script setup lang="ts">
import { ref, onMounted, watch, reactive } from "vue";
import { DeleteClient, ListClients, UpdateClient } from "../../wailsjs/go/main/App";
import InitialsAvatar from "@/components/InitialsAvatar.vue";
import { useAuthStore } from "@/stores/auth";
import { formatLoyaltyPoints, loyaltyTier, loyaltyTierColor } from "@/lib/client";

const colors = ["yellow", "blue", "red", "purple", "pink", "green", "orange", "teal"];
const auth = useAuthStore();

const headers = [
  { title: "", key: "avatar", sortable: false, width: 56 },
  { title: "Name", key: "name" },
  { title: "Phone", key: "phone" },
  { title: "Loyalty Points", key: "totalLoyaltyPoints" },
  { title: "Tier", key: "tier" },
  { title: "Valid Points", key: "validLoyaltyPoints" },
  { title: "Created", key: "createdAt" },
  { title: "", key: "actions", sortable: false, width: 110 },
];
const items = ref<any[]>([]);
const visibleItems = ref<any[]>([]);
const search = ref("");
const itemsPerPage = ref(10);
const totalItems = ref(0);
const loading = ref(true);
const editOpen = ref(false);
const deleteOpen = ref(false);
const saving = ref(false);
const deleting = ref(false);
const editTarget = ref<any>(null);
const deleteTarget = ref<any>(null);
const editForm = reactive({ name: "", phone: "" });
const snackbar = reactive({ show: false, text: "" });

function openEdit(item: any) {
  editTarget.value = item;
  editForm.name = item.name ?? "";
  editForm.phone = item.phone ?? "";
  editOpen.value = true;
}

function openDelete(item: any) {
  deleteTarget.value = item;
  deleteOpen.value = true;
}

async function saveEdit() {
  if (!auth.isAdmin || !editTarget.value) return;
  saving.value = true;
  try {
    await UpdateClient(editTarget.value.id, editForm.name.trim(), editForm.phone.trim());
    editOpen.value = false;
    await loadClients();
    snackbar.text = "Client updated";
    snackbar.show = true;
  } catch (e: any) {
    snackbar.text = e?.toString() || "Could not update client";
    snackbar.show = true;
  } finally {
    saving.value = false;
  }
}

async function deleteSelected() {
  if (!auth.isAdmin || !deleteTarget.value) return;
  deleting.value = true;
  try {
    await DeleteClient(deleteTarget.value.id);
    deleteOpen.value = false;
    await loadClients();
    snackbar.text = "Client deleted";
    snackbar.show = true;
  } catch (e: any) {
    snackbar.text = e?.toString() || "Could not delete client";
    snackbar.show = true;
  } finally {
    deleting.value = false;
  }
}

async function loadClients() {
  loading.value = true;
  try {
    items.value = await ListClients();
  } catch {
    items.value = [];
  } finally {
    loading.value = false;
  }
}

function loadItems(options: {
  page: number;
  itemsPerPage: number;
  sortBy: { key: string; order: string }[];
}) {
  let filtered = items.value;
  const query = search.value.trim().toLowerCase();
  if (query) {
    filtered = filtered.filter((item) =>
      [item.name, item.phone].some((value) =>
        String(value ?? "")
          .toLowerCase()
          .includes(query),
      ),
    );
  }
  const sort = options.sortBy[0];
  if (sort) {
    filtered = [...filtered].sort((a, b) => {
      const left = a[sort.key] ?? "";
      const right = b[sort.key] ?? "";
      return (left > right ? 1 : left < right ? -1 : 0) * (sort.order === "desc" ? -1 : 1);
    });
  }
  totalItems.value = filtered.length;
  const start = (options.page - 1) * options.itemsPerPage;
  visibleItems.value = filtered.slice(start, start + options.itemsPerPage);
}

watch([items, search, itemsPerPage], () =>
  loadItems({ page: 1, itemsPerPage: itemsPerPage.value, sortBy: [] }),
);

onMounted(loadClients);
</script>
