<script setup lang="ts">
// Change Client / Change Salesperson for one invoice, from the invoice list.
//
// It opens at once with the invoice's current client or salesperson already
// named (from the list row) and the options as last loaded, then swaps in
// the fresh options. Before, the salesperson field showed the bare id until
// the roster arrived, and kept showing it for a salesperson no longer offered.
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { ChangeInvoiceClient, ChangeInvoiceSalesperson } from "../../wailsjs/go/main/App";
import {
  cachedFirstClients,
  cachedSalespersons,
  loadSalespersons,
  searchClients,
  type ChangeableInvoice,
  type SalespersonOption,
} from "@/lib/invoiceEditOptions";
import { formatLebanesePhone } from "@/lib/phone";

const props = defineProps<{
  modelValue: boolean;
  type: "client" | "salesperson";
  invoice: ChangeableInvoice | null;
}>();
const emit = defineEmits<{
  "update:modelValue": [value: boolean];
  saved: [];
  error: [message: string];
}>();

const SEARCH_DELAY_MS = 150;

const value = ref<number | null>(null);
const original = ref<number | null>(null);
const salespersons = ref<SalespersonOption[]>([]);
const clients = ref<any[]>([]);
// Undefined until the user types, so the box shows the picked client's name.
const search = ref<string | undefined>(undefined);
const loading = ref(false);
const saving = ref(false);

const title = computed(() => (props.type === "client" ? "Change Client" : "Change Salesperson"));
const saveDisabled = computed(() => {
  if (saving.value) return true;
  if (props.type === "client" && !value.value) return true;
  return (value.value || 0) === (original.value || 0);
});

function clientTitle(client: any): string {
  return client.phone ? `${client.name} · ${formatLebanesePhone(client.phone)}` : client.name;
}

// The invoice's own salesperson stays on offer (and named) even when the
// roster leaves them out, e.g. a till-only salesperson after kashi took over.
function withCurrentSalesperson(roster: SalespersonOption[]): SalespersonOption[] {
  const inv = props.invoice;
  const id = Number(inv?.salespersonId || 0);
  if (!id || roster.some((s) => s.id === id)) return roster;
  return [{ id, fullName: inv?.salespersonName || "Former salesperson" }, ...roster];
}

// The picked client stays in the list whatever the search returns, so the
// field keeps showing their name.
function withSelectedClient(list: any[]): any[] {
  const id = value.value;
  if (!id || list.some((c) => c.id === id)) return list;
  const known = clients.value.find((c) => c.id === id);
  if (known) return [known, ...list];
  const inv = props.invoice;
  if (inv && Number(inv.clientId) === id && inv.clientName) {
    return [{ id, name: inv.clientName, phone: inv.clientPhone || "" }, ...list];
  }
  return list;
}

let request = 0;
let searchTimer: ReturnType<typeof setTimeout> | undefined;

async function refreshSalespersons() {
  const mine = ++request;
  loading.value = true;
  try {
    const roster = await loadSalespersons();
    if (mine === request && props.modelValue) salespersons.value = withCurrentSalesperson(roster);
  } catch (e: any) {
    if (mine === request) emit("error", e?.toString() || "Could not load salespersons");
  } finally {
    if (mine === request) loading.value = false;
  }
}

async function refreshClients(term: string) {
  const mine = ++request;
  loading.value = true;
  try {
    const found = await searchClients(term);
    if (mine === request && props.modelValue) clients.value = withSelectedClient(found);
  } catch {
    if (mine === request) emit("error", "Could not search clients");
  } finally {
    if (mine === request) loading.value = false;
  }
}

function open() {
  const inv = props.invoice;
  clearTimeout(searchTimer);
  saving.value = false;
  search.value = undefined;
  if (props.type === "salesperson") {
    value.value = Number(inv?.salespersonId || 0) || null;
    original.value = value.value;
    salespersons.value = withCurrentSalesperson(cachedSalespersons() ?? []);
    void refreshSalespersons();
  } else {
    value.value = Number(inv?.clientId || 0) || null;
    original.value = value.value;
    clients.value = [];
    clients.value = withSelectedClient(cachedFirstClients() ?? []);
    void refreshClients("");
  }
}

watch(
  () => props.modelValue,
  (isOpen) => {
    if (isOpen) open();
    else {
      clearTimeout(searchTimer);
      request++;
      loading.value = false;
    }
  },
  { immediate: true },
);

function onPickClient(id: number | null) {
  value.value = id;
}

function onSearch(text: string | null) {
  const term = text ?? "";
  search.value = term;
  clearTimeout(searchTimer);
  // Picking a client puts their name in the box; that isn't a new search.
  const picked = clients.value.find((c) => c.id === value.value);
  if (picked && term === clientTitle(picked)) return;
  searchTimer = setTimeout(() => void refreshClients(term), SEARCH_DELAY_MS);
}

function close() {
  emit("update:modelValue", false);
}

async function save() {
  const inv = props.invoice;
  if (!inv || saveDisabled.value) return;
  saving.value = true;
  try {
    if (props.type === "client") await ChangeInvoiceClient(inv.id, value.value || 0);
    else await ChangeInvoiceSalesperson(inv.id, value.value || 0);
    emit("saved");
    close();
  } catch (e: any) {
    emit("error", e?.toString() || "Could not update invoice");
  } finally {
    saving.value = false;
  }
}

onBeforeUnmount(() => clearTimeout(searchTimer));
</script>

<template>
  <k-dialog :model-value="modelValue" max-width="420" @update:model-value="emit('update:modelValue', $event)">
    <k-card class="invoice-change-dialog" :data-type="type">
      <k-card-title>{{ title }}</k-card-title>
      <k-card-text>
        <k-autocomplete
          v-if="type === 'client'"
          :model-value="value"
          :search="search"
          :items="clients"
          :item-title="clientTitle"
          item-value="id"
          :loading="loading"
          no-filter
          placeholder="Search by name or phone"
          variant="outlined"
          @update:model-value="onPickClient"
          @update:search="onSearch"
        />
        <k-select
          v-else
          v-model="value"
          :items="salespersons"
          item-title="fullName"
          item-value="id"
          placeholder="Select a salesman"
          clearable
          :loading="loading"
          variant="outlined"
        />
      </k-card-text>
      <k-card-actions>
        <k-spacer />
        <k-btn variant="text" class="invoice-change-cancel" @click="close">Cancel</k-btn>
        <k-btn color="primary" class="invoice-change-save" :loading="saving" :disabled="saveDisabled" @click="save">Save</k-btn>
      </k-card-actions>
    </k-card>
  </k-dialog>
</template>
