<template>
  <k-dialog v-model="open" max-width="640">
    <k-card class="devices-dialog-card" rounded="xl">
      <div class="dialog-close-row px-4 pt-0">
        <k-btn icon="mdi-close" variant="text" aria-label="Close devices" @click="open = false" />
      </div>
      <k-card-text class="devices-grid px-4 pt-0 pb-6">
        <div v-for="device in devices" :key="device.kind" class="device-card">
          <k-icon :icon="device.icon" size="72" :class="`device-status--${device.state}`" />
          <span class="device-card-label">{{ device.label }}</span>
          <span class="device-card-name">{{ device.name }}</span>
          <span :class="['device-card-word', `device-status--${device.state}`]">{{ device.word }}</span>
          <!-- Only an admin changes which device the till uses. The wrapper is
               always there so every card keeps the same five rows. -->
          <div>
            <template v-if="auth.isAdmin && device.state !== 'ok'">
              <k-btn
                v-if="device.kind === 'printer'"
                class="mt-1"
                prepend-icon="mdi-magnify"
                variant="tonal"
                @click="openFindPrinter"
              >
                Find
              </k-btn>
              <k-btn
                v-else-if="device.kind === 'scanner' && device.state === 'off'"
                class="mt-1"
                prepend-icon="mdi-magnify"
                variant="tonal"
                @click="forgetScanner"
              >
                Use another
              </k-btn>
            </template>
          </div>
        </div>
      </k-card-text>
      <k-card-text v-if="notice" class="text-center pt-0 pb-4">{{ notice }}</k-card-text>
    </k-card>
  </k-dialog>

  <k-dialog v-model="findOpen" max-width="720" scrollable>
    <k-card class="devices-dialog-card" rounded="xl">
      <k-card-title class="d-flex align-center">
        Find the receipt printer
        <k-spacer />
        <k-btn icon="mdi-refresh" variant="text" aria-label="Search again" :loading="searching" @click="search" />
        <k-btn icon="mdi-close" variant="text" aria-label="Close" @click="findOpen = false" />
      </k-card-title>
      <k-card-text>
        <div v-if="searchError" class="text-error mb-3">{{ searchError }}</div>

        <template v-if="moveSuggestion">
          <div class="find-section-title">Printer plugged into {{ moveSuggestion.port }}</div>
          <p class="mb-2">
            <strong>{{ found?.receiptPrinter }}</strong> prints to {{ receiptPort || "another port" }}, but the
            printer that's plugged in now is on {{ moveSuggestion.port }}.
          </p>
          <k-btn color="primary" :loading="busy" class="mb-4" @click="movePrinter(moveSuggestion.port)">
            Move {{ found?.receiptPrinter }} to {{ moveSuggestion.port }}
          </k-btn>
        </template>

        <div class="find-section-title">Printers on this computer</div>
        <div v-if="found && found.printers.length === 0" class="text-medium-emphasis mb-4">No printers installed.</div>
        <div v-for="printer in found?.printers ?? []" :key="printer.name" class="find-row">
          <k-icon icon="mdi-printer" :class="printer.ready ? 'device-status--ok' : 'device-status--off'" />
          <div class="find-row-text">
            <div class="font-weight-bold">{{ printer.name }}</div>
            <div class="text-medium-emphasis">
              {{ printer.port || "no port" }} · {{ printer.ready ? "Ready" : "Offline" }}
            </div>
          </div>
          <k-chip v-if="printer.receipt" size="small" color="primary" variant="tonal">Receipt printer</k-chip>
          <k-btn v-else :loading="busy" variant="tonal" @click="usePrinter(printer.name)">Use</k-btn>
        </div>

        <template v-if="found && found.livePorts.length > 0">
          <div class="find-section-title mt-4">USB printers plugged in now</div>
          <div v-for="port in found.livePorts" :key="port.port" class="find-row">
            <k-icon icon="mdi-usb" class="device-status--ok" />
            <div class="find-row-text">
              <div class="font-weight-bold">{{ port.port }}</div>
              <div class="text-medium-emphasis">
                {{ port.usedBy.length ? `Used by ${port.usedBy.join(", ")}` : "No printer set to this port" }}
                <span v-if="port.device"> · {{ port.device }}</span>
              </div>
            </div>
            <k-btn
              v-if="found.receiptPrinter && !port.usedBy.includes(found.receiptPrinter)"
              :loading="busy"
              variant="tonal"
              @click="movePrinter(port.port)"
            >
              Move here
            </k-btn>
          </div>
        </template>
        <div v-else-if="found" class="text-medium-emphasis mt-4">
          No USB printer is plugged in right now. Check the cable and the power, then search again.
        </div>
      </k-card-text>
    </k-card>
  </k-dialog>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { useAuthStore } from "@/stores/auth";
import { useDeviceStatus } from "@/composables/useDeviceStatus";
import { FindPrinters, ForgetScanner, MovePrinterToPort, UseReceiptPrinter } from "../../wailsjs/go/main/App";
import type { main } from "../../wailsjs/go/models";

const open = defineModel<boolean>({ required: true });

const auth = useAuthStore();
const { devices } = useDeviceStatus();
const notice = ref("");

const findOpen = ref(false);
const found = ref<main.PrinterSearch | null>(null);
const searching = ref(false);
const searchError = ref("");
const busy = ref(false);

const receiptPort = computed(
  () => found.value?.printers.find((printer) => printer.receipt)?.port ?? "",
);

/**
 * The one-tap fix for the usual case: the receipt printer is offline, and
 * exactly one USB printer is plugged in, on a port the receipt printer
 * isn't using.
 */
const moveSuggestion = computed(() => {
  const result = found.value;
  if (!result?.receiptPrinter) return null;
  const receipt = result.printers.find((printer) => printer.receipt);
  const live = result.livePorts.length === 1 ? result.livePorts[0] : undefined;
  if (!receipt || receipt.ready || !live) return null;
  return live.usedBy.includes(result.receiptPrinter) ? null : live;
});

async function search() {
  searching.value = true;
  searchError.value = "";
  try {
    found.value = await FindPrinters();
  } catch (error) {
    searchError.value = String(error);
  } finally {
    searching.value = false;
  }
}

function openFindPrinter() {
  findOpen.value = true;
  void search();
}

async function act(action: () => Promise<unknown>) {
  busy.value = true;
  searchError.value = "";
  try {
    await action();
    await search();
  } catch (error) {
    searchError.value = String(error);
  } finally {
    busy.value = false;
  }
}

function usePrinter(name: string) {
  void act(() => UseReceiptPrinter(name));
}

function movePrinter(port: string) {
  const name = found.value?.receiptPrinter;
  if (!name) return;
  void act(() => MovePrinterToPort(name, port));
}

async function forgetScanner() {
  try {
    await ForgetScanner();
    notice.value = "Scan a barcode with the scanner you want to use.";
  } catch (error) {
    notice.value = String(error);
  }
}
</script>

<style scoped>
.devices-dialog-card {
  border: 4px dotted rgba(0, 0, 0, 0.45);
}
.dialog-close-row {
  display: flex;
  justify-content: flex-end;
}
/* The cards share the grid's rows (subgrid), so each line -- icon, label,
   name, status, button -- sits at one height across all four, however many
   lines a long device name takes. */
.devices-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  grid-template-rows: repeat(5, auto);
  column-gap: 16px;
}
.device-card {
  display: grid;
  grid-row: span 5;
  grid-template-rows: subgrid;
  justify-items: center;
  align-items: start;
  row-gap: 4px;
  text-align: center;
}
.device-card-label {
  font-size: 1.1rem;
  font-weight: 700;
}
.device-card-name {
  font-size: 0.85rem;
  overflow-wrap: anywhere;
}
.device-card-word {
  font-size: 0.85rem;
  font-weight: 700;
}
.device-status--ok {
  color: #43a047;
}
.device-status--off {
  color: #e53935;
}
.device-status--unknown {
  color: #9e9e9e;
}
.find-section-title {
  font-size: 0.8rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  margin-bottom: 8px;
}
.find-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 0;
  border-bottom: 1px solid rgba(0, 0, 0, 0.08);
}
.find-row-text {
  flex: 1 1 auto;
  min-width: 0;
  overflow-wrap: anywhere;
}
</style>
