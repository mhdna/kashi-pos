<script setup lang="ts">
// Change Salesperson, from the invoice list: who the whole invoice is under,
// then who sold each item. Tap a row to open the picture grid under it;
// nothing is saved until Save.
import { computed, ref, watch } from "vue";
import { ChangeInvoiceLineSalesperson, ChangeInvoiceSalesperson, GetInvoiceDetails } from "../../wailsjs/go/main/App";
import { cachedSalespersons, loadSalespersons, type ChangeableInvoice, type SalespersonOption } from "@/lib/invoiceEditOptions";
import { lineSalespersonChanges } from "@/lib/lineSalespersons";
import { salespersonColor } from "@/lib/salespersonColor";
import InitialsAvatar from "@/components/InitialsAvatar.vue";
import SalespersonGrid from "@/components/SalespersonGrid.vue";

interface Line {
  lineId: number;
  name: string;
  code: string;
  qty: number;
  salespersonId: number;
  salespersonName: string;
}

const props = defineProps<{ modelValue: boolean; invoice: ChangeableInvoice | null }>();
const emit = defineEmits<{
  "update:modelValue": [value: boolean];
  saved: [];
  error: [message: string];
}>();

const lines = ref<Line[]>([]);
const salespersons = ref<SalespersonOption[]>([]);
const picks = ref<Record<number, number>>({});
// Which item's grid is open (the whole-invoice grid is always open).
const openLine = ref<number | null>(null);
// The whole-invoice pick, until saved; null while untouched.
const invoicePick = ref<number | null>(null);
const loading = ref(false);
const saving = ref(false);
let request = 0;

const savedInvoiceId = computed(() => Number(props.invoice?.salespersonId || 0));
const invoiceId = computed(() => invoicePick.value ?? savedInvoiceId.value);
const invoiceChanged = computed(() => invoiceId.value !== savedInvoiceId.value);
// Who the invoice is under now, counting a pick not saved yet.
const invoicePerson = computed(() => {
  const id = invoiceId.value;
  if (!id) return null;
  const known = salespersons.value.find((s) => s.id === id);
  if (known) return known;
  return { id, fullName: id === savedInvoiceId.value && props.invoice?.salespersonName ? props.invoice.salespersonName : "Former salesperson" };
});
const invoiceName = computed(() => invoicePerson.value?.fullName ?? "");
// One item is the whole sale, so the items are only listed from two up.
const showItems = computed(() => lines.value.length > 1);
const changes = computed(() => {
  const picked = lineSalespersonChanges(lines.value, picks.value);
  // The lone item follows the invoice: if it was set apart earlier, a new
  // invoice salesperson has to take it along or the change would not show.
  const only = lines.value.length === 1 ? lines.value[0]! : null;
  if (only && invoiceChanged.value && only.salespersonId !== 0) return [{ lineId: only.lineId, salespersonId: 0 }];
  return picked;
});
const saveDisabled = computed(
  () => saving.value || loading.value || (changes.value.length === 0 && !invoiceChanged.value),
);

// Whoever the line is under right now, counting what was picked but not saved.
function currentId(line: Line): number {
  return line.lineId in picks.value ? picks.value[line.lineId]! : line.salespersonId;
}
function personFor(line: Line): { id: number; fullName: string } | null {
  const id = currentId(line);
  if (!id) return null;
  const known = salespersons.value.find((s) => s.id === id);
  if (known) return known;
  return { id, fullName: id === line.salespersonId && line.salespersonName ? line.salespersonName : "Former salesperson" };
}
const rowChanged = (line: Line) => changes.value.some((c) => c.lineId === line.lineId);

async function open() {
  const mine = ++request;
  picks.value = {};
  invoicePick.value = null;
  openLine.value = null;
  lines.value = [];
  saving.value = false;
  salespersons.value = cachedSalespersons() ?? [];
  const id = props.invoice?.id;
  if (!id) return;
  loading.value = true;
  try {
    const [details, roster] = await Promise.all([GetInvoiceDetails(id), loadSalespersons().catch(() => salespersons.value)]);
    if (mine !== request) return;
    salespersons.value = roster;
    lines.value = ((details?.items ?? []) as any[]).map((item) => ({
      lineId: Number(item.lineId),
      name: String(item.name ?? ""),
      code: String(item.code ?? ""),
      qty: Number(item.qty ?? 0),
      salespersonId: Number(item.salespersonId || 0),
      salespersonName: String(item.salespersonName ?? ""),
    }));
  } catch (e: any) {
    if (mine === request) emit("error", e?.toString() || "Could not load the invoice's items");
  } finally {
    if (mine === request) loading.value = false;
  }
}

watch(
  () => props.modelValue,
  (isOpen) => {
    if (isOpen) void open();
    else request++;
  },
  { immediate: true },
);

function toggle(line: Line) {
  openLine.value = openLine.value === line.lineId ? null : line.lineId;
}
function pickInvoice(id: number) {
  invoicePick.value = id;
}
function pick(line: Line, id: number) {
  picks.value = { ...picks.value, [line.lineId]: id };
  openLine.value = null;
}
function close() {
  emit("update:modelValue", false);
}

async function save() {
  const todo = changes.value;
  if (!props.invoice || saveDisabled.value) return;
  saving.value = true;
  try {
    // The invoice first, then the items that were set apart from it.
    if (invoiceChanged.value) await ChangeInvoiceSalesperson(props.invoice.id, invoiceId.value);
    for (const change of todo) {
      await ChangeInvoiceLineSalesperson(props.invoice.id, change.lineId, change.salespersonId);
    }
    emit("saved");
    close();
  } catch (e: any) {
    emit("error", e?.toString() || "Could not update the items");
    // Some lines may already be saved; reload so the list shows the truth.
    await open();
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <k-dialog :model-value="modelValue" :max-width="showItems ? 980 : 520" @update:model-value="emit('update:modelValue', $event)">
    <k-card class="line-sp-dialog">
      <div class="line-sp-header">
        <k-avatar color="primary" variant="tonal" size="40"><k-icon icon="mdi-account-group" /></k-avatar>
        <div class="line-sp-header__text">
          <div class="text-h6">Change Salesperson</div>
          <div v-if="invoice?.invoiceCode" class="text-body-2 text-medium-emphasis">Invoice {{ invoice.invoiceCode }}</div>
        </div>
      </div>

      <k-card-text class="line-sp-body" :class="{ 'line-sp-body--split': showItems }">
        <!-- Left: who the whole invoice is under. Right: each item. -->
        <div class="line-sp-col">
          <div class="line-sp-label">Whole invoice</div>
          <!-- Always open: pick who the whole invoice is under. -->
          <div class="line-sp-block" data-test="invoice-block">
            <SalespersonGrid
              :salespersons="salespersons"
              :model-value="invoiceId"
              default-label="No salesperson"
              @update:model-value="pickInvoice"
            />
          </div>
        </div>
        <div v-if="showItems" class="line-sp-col">
          <div class="line-sp-label">Items</div>
          <div v-for="line in lines" :key="line.lineId" class="line-sp-item" :class="{ 'line-sp-item--open': openLine === line.lineId }">
            <button type="button" class="line-sp-row" :data-test="`line-${line.lineId}`" @click="toggle(line)">
              <span class="line-sp-row__text">
                <span class="line-sp-row__name">{{ line.name }}</span>
                <span class="line-sp-row__meta">{{ line.qty }} × {{ line.code }}</span>
              </span>
              <span class="line-sp-who" :class="{ 'line-sp-who--changed': rowChanged(line) }">
                <template v-if="personFor(line)">
                  <InitialsAvatar :name="personFor(line)!.fullName" :color="salespersonColor(personFor(line)!)" :size="28" />
                  <span class="line-sp-who__name">{{ personFor(line)!.fullName }}</span>
                </template>
                <span v-else class="line-sp-who__name line-sp-who__name--muted">{{ invoiceName ? `Invoice: ${invoiceName}` : "No salesperson" }}</span>
                <k-icon :icon="openLine === line.lineId ? 'mdi-chevron-up' : 'mdi-chevron-down'" size="20" class="line-sp-chevron" />
              </span>
            </button>
            <div v-if="openLine === line.lineId" class="line-sp-picker">
              <SalespersonGrid
                :salespersons="salespersons"
                :model-value="currentId(line)"
                :default-label="invoiceName ? `Same as invoice (${invoiceName.split(' ')[0]})` : 'Same as invoice'"
                @update:model-value="pick(line, $event)"
              />
            </div>
          </div>
        </div>
      </k-card-text>

      <k-card-actions class="px-6 pb-4">
        <k-spacer />
        <k-btn variant="text" class="line-sp-cancel" @click="close">Cancel</k-btn>
        <k-btn color="primary" variant="flat" prepend-icon="mdi-check" class="line-sp-save" :loading="saving" :disabled="saveDisabled" @click="save">Save</k-btn>
      </k-card-actions>
    </k-card>
  </k-dialog>
</template>

<style scoped>
.line-sp-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 20px 24px 8px;
}
.line-sp-body {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 20px;
  max-height: 68vh;
}
.line-sp-body--split {
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
}
/* Each column scrolls on its own; nothing in it shrinks to fit. */
.line-sp-col {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-height: 0;
  overflow-y: auto;
}
@media (max-width: 760px) {
  .line-sp-body--split {
    grid-template-columns: minmax(0, 1fr);
    overflow-y: auto;
  }
}
.line-sp-col > * {
  flex: 0 0 auto;
}
.line-sp-label {
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: rgba(var(--k-theme-on-surface), 0.6);
}
.line-sp-empty {
  padding: 24px 0;
  text-align: center;
  color: rgba(var(--k-theme-on-surface), 0.6);
}
.line-sp-block {
  padding: 10px;
  border: 1px solid rgba(var(--k-border-color), 0.2);
  border-radius: 10px;
}
.line-sp-item {
  border: 1px solid rgba(var(--k-border-color), 0.2);
  border-radius: 10px;
  overflow: hidden;
}
.line-sp-item--open {
  border-color: rgba(var(--k-theme-primary), 0.6);
}
.line-sp-row {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  padding: 10px 12px;
  background: transparent;
  border: 0;
  color: inherit;
  cursor: pointer;
  font: inherit;
  text-align: left;
}
.line-sp-row:hover {
  background: rgba(var(--k-theme-on-surface), 0.05);
}
.line-sp-row__text {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-width: 0;
}
.line-sp-row__name {
  overflow: hidden;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.line-sp-row__meta {
  font-size: 0.75rem;
  color: rgba(var(--k-theme-on-surface), 0.6);
}
.line-sp-who {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 8px;
  max-width: 50%;
}
.line-sp-who__name {
  overflow: hidden;
  font-size: 0.875rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.line-sp-who__name--muted {
  color: rgba(var(--k-theme-on-surface), 0.55);
}
/* A pick not saved yet. */
.line-sp-who--changed .line-sp-who__name {
  color: rgb(var(--k-theme-primary));
  font-weight: 600;
}
.line-sp-chevron {
  opacity: 0.6;
}
.line-sp-picker {
  padding: 8px 12px 12px;
  border-top: 1px solid rgba(var(--k-border-color), 0.2);
}
</style>
