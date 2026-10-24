<template>
  <!-- touchless: a permanent drawer never swipes shut, and Vuetify's swipe
       handler (on window, non-passive) can claim a drag that starts a little
       sideways, which then never scrolls the page list. -->
  <v-navigation-drawer permanent touchless width="72" class="nav-drawer border-0">
    <v-list class="d-flex flex-column fill-height pa-0" nav>
      <div class="nav-items-group">
        <v-list-item
          v-for="item in items"
          :key="item.value"
          class="nav-item"
          :value="item.value"
          :active="route.path === item.to"
          @click.prevent="handleItemClick(item)"
        >
          <div class="nav-item-content">
            <span class="nav-icon-wrap">
              <v-icon :icon="item.icon" size="24" />
              <span
                v-if="item.status"
                :class="['nav-status-dot', `nav-status-dot--${item.status}`]"
                :aria-label="item.status === 'down' ? 'Disconnected' : 'Connected'"
              />
            </span>
            <span class="nav-item-label text-center mt-1">{{ item.t }}</span>
          </div>
        </v-list-item>
      </div>

      <div v-if="visitorCounterEnabled" class="visitor-counter" aria-label="People visits today">
        <v-btn
          class="visitor-counter-btn"
          icon
          variant="flat"
          rounded="lg"
          aria-label="Increase visitor count"
          @click="adjustVisitors(1)"
        >
          <v-icon icon="mdi-chevron-up" size="32" />
        </v-btn>
        <span class="visitor-count">{{ visitorCount }}</span>
        <v-btn
          class="visitor-counter-btn"
          icon
          variant="flat"
          rounded="lg"
          aria-label="Decrease visitor count"
          @click="adjustVisitors(-1)"
        >
          <v-icon icon="mdi-chevron-down" size="32" />
        </v-btn>
      </div>

      <div
        :class="['sync-status', `sync-status--${syncState.tone}`, { 'sync-status--link': auth.isAdmin }]"
        :title="syncState.title"
        :role="auth.isAdmin ? 'button' : undefined"
        :aria-label="`Sync: ${syncState.word}`"
        @click="auth.isAdmin && openSyncSettings()"
      >
        <v-icon :icon="syncState.icon" size="20" />
        <span class="sync-status-word">{{ syncState.word }}</span>
      </div>

      <div class="device-status" aria-label="Devices">
        <v-icon
          v-for="device in deviceIcons"
          :key="device.icon"
          :icon="device.icon"
          :class="`device-status--${device.state}`"
          :title="device.title"
          size="20"
        />
      </div>

      <v-list-item
        class="nav-item options-item"
        :value="undefined"
        :active="false"
        @click="optionsOpen = true"
      >
        <div class="nav-item-content">
          <v-icon icon="mdi-logout" size="24" />
          <span class="nav-item-label text-center mt-1">Quit</span>
        </div>
      </v-list-item>
    </v-list>
  </v-navigation-drawer>

  <v-dialog v-model="optionsOpen" max-width="520">
    <v-card class="options-dialog-card" rounded="xl">
      <div class="dialog-close-row px-4 pt-0">
        <v-btn
          icon="mdi-close"
          variant="text"
          aria-label="Close out menu"
          @click="optionsOpen = false"
        />
      </div>

      <v-card-text class="options-grid options-grid--three px-4 pt-0 pb-4">
        <v-card class="option-card" rounded="lg" variant="text" @click="openCloseShift">
          <v-icon icon="mdi-close-circle-outline" size="44" />
          <span>Close Shift</span>
        </v-card>
        <v-card class="option-card" rounded="lg" variant="text" @click="logout">
          <v-icon icon="mdi-logout" size="44" />
          <span>Logout</span>
        </v-card>
        <v-card class="option-card" rounded="lg" variant="text" @click="closeApp">
          <v-icon icon="mdi-close" size="44" />
          <span>Close App</span>
        </v-card>
        <v-card class="option-card" rounded="lg" variant="text" @click="askPower('restart')">
          <v-icon icon="mdi-restart" size="44" />
          <span>Restart</span>
        </v-card>
        <v-card class="option-card" rounded="lg" variant="text" @click="askPower('shutdown')">
          <v-icon icon="mdi-power" size="44" />
          <span>Shut Down</span>
        </v-card>
      </v-card-text>
    </v-card>
  </v-dialog>

  <v-dialog v-model="closeAppConfirmOpen" max-width="360">
    <v-card class="options-dialog-card" rounded="xl">
      <v-card-title>Close app</v-card-title>
      <v-card-text>Close the app and go to Windows?</v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="closeAppConfirmOpen = false">Cancel</v-btn>
        <v-btn color="error" variant="flat" @click="confirmCloseApp">Close App</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-dialog :model-value="powerConfirm !== null" max-width="360" @update:model-value="powerConfirm = null">
    <v-card class="options-dialog-card" rounded="xl">
      <v-card-title>{{ powerConfirm === 'restart' ? 'Restart computer' : 'Shut down computer' }}</v-card-title>
      <v-card-text>
        {{ powerConfirm === 'restart' ? 'Restart this computer now?' : 'Turn this computer off now?' }}
        <div v-if="powerError" class="text-error mt-2">{{ powerError }}</div>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="powerConfirm = null">Cancel</v-btn>
        <v-btn color="error" variant="flat" @click="confirmPower">
          {{ powerConfirm === 'restart' ? 'Restart' : 'Shut Down' }}
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { usePageUnlocks } from "@/composables/usePageUnlocks";
import { useVisitorCounter } from "@/composables/useVisitorCounter";
import { useSyncStatus } from "@/composables/useSyncStatus";
import { useDeviceStatus } from "@/composables/useDeviceStatus";
import { useResyncStore } from "@/stores/resync";
import { useAttendanceStatus } from "@/composables/useAttendanceStatus";
import { Quit } from "../../wailsjs/runtime/runtime";
import { RestartComputer, ShutDownComputer } from "../../wailsjs/go/main/App";

const auth = useAuthStore();
const route = useRoute();
const router = useRouter();
const { pageUnlocks } = usePageUnlocks();
const {
  enabled: visitorCounterEnabled,
  count: visitorCount,
  adjust: adjustVisitors,
} = useVisitorCounter();

const { status: syncStatus } = useSyncStatus();
const resync = useResyncStore();
const syncState = computed(() => {
  const st = syncStatus.value;
  if (resync.running)
    return { tone: 'busy', icon: 'mdi-sync', word: 'Syncing', title: resync.progress || 'Full sync running' };
  if (!st || !st.configured)
    return { tone: 'off', icon: 'mdi-cloud-off-outline', word: 'Off', title: 'Sync not set up' };
  if (st.failed > 0)
    return { tone: 'error', icon: 'mdi-alert-circle', word: 'Error', title: `${st.failed} record(s) rejected by kashi` };
  if (st.pending > 0 || st.confirmedElsewhere > 0)
    return {
      tone: 'busy',
      icon: 'mdi-cloud-upload',
      word: 'Syncing',
      title: `${st.pending + st.confirmedElsewhere} record(s) not yet confirmed here`,
    };
  return { tone: 'ok', icon: 'mdi-cloud-check-variant', word: 'Synced', title: 'Fully synced with kashi' };
});
const { status: attendanceStatus } = useAttendanceStatus();

const { status: deviceStatus } = useDeviceStatus();
const deviceWords: Record<string, string> = {
  ok: "connected",
  off: "not connected",
  unknown: "not seen yet",
};
const deviceIcons = computed(() => {
  const st = deviceStatus.value;
  const printerState = st?.printer ?? "unknown";
  return [
    { icon: "mdi-barcode-scanner-gun", state: st?.scanner ?? "unknown", title: `Scanner: ${deviceWords[st?.scanner ?? "unknown"]}` },
    {
      icon: "mdi-printer",
      state: printerState,
      title: st?.printerName
        ? `Printer ${st.printerName}: ${printerState === "ok" ? "ready" : "offline"}`
        : "Printer: none chosen in Settings",
    },
    { icon: "mdi-keyboard", state: st?.keyboard ?? "unknown", title: `Keyboard: ${deviceWords[st?.keyboard ?? "unknown"]}` },
    { icon: "mdi-mouse", state: st?.mouse ?? "unknown", title: `Mouse: ${deviceWords[st?.mouse ?? "unknown"]}` },
  ];
});

function openSyncSettings() {
  router.push({ path: '/settings', query: { panel: 'general-sync' } });
}
const optionsOpen = ref(false);
const closeAppConfirmOpen = ref(false);

function handleItemClick(item: { locked?: boolean; to: string }) {
  if (item.locked) return;
  void router.push(item.to);
}

function logout() {
  optionsOpen.value = false;
  auth.logout();
  void router.push("/login");
}

function closeApp() {
  optionsOpen.value = false;
  closeAppConfirmOpen.value = true;
}

function confirmCloseApp() {
  closeAppConfirmOpen.value = false;
  Quit();
}

const powerConfirm = ref<"restart" | "shutdown" | null>(null);
const powerError = ref("");

function askPower(action: "restart" | "shutdown") {
  optionsOpen.value = false;
  powerError.value = "";
  powerConfirm.value = action;
}

async function confirmPower() {
  try {
    await (powerConfirm.value === "restart" ? RestartComputer() : ShutDownComputer());
  } catch (err) {
    powerError.value = String(err);
  }
}

function openCloseShift() {
  optionsOpen.value = false;
  void router.push("/close-shift");
}

const items = computed(() => {
  const all = [
    { value: "invoice", to: "/home", icon: "mdi-cash-register", t: "Invoice" },
    {
      value: "invoices-list",
      to: "/invoices-list",
      icon: "mdi-format-list-bulleted-square",
      t: "Invoices",
    },
    { value: "shifts", to: "/shifts", icon: "mdi-cash-fast", t: "Balance" },
    {
      value: "clients",
      to: "/clients",
      icon: "mdi-account-multiple",
      t: "Clients",
      locked: !auth.isAdmin && !pageUnlocks.value.clients,
    },
    {
      value: "inventory",
      to: "/inventory",
      icon: "mdi-package-variant-closed",
      t: "Inventory",
      locked: !auth.isAdmin && !pageUnlocks.value.inventory,
    },
    {
      value: "products",
      to: "/products",
      icon: "mdi-tag-multiple-outline",
      t: "Products",
      locked: !auth.isAdmin && !pageUnlocks.value.products,
    },
    {
      value: "salespersons",
      to: "/salespersons",
      icon: "mdi-chart-bar",
      t: "Sales",
      locked: !auth.isAdmin && !pageUnlocks.value.salespersons,
    },
    // Sales per visitor, so only while visitors are being counted (admins
    // keep it to look back). A manager opens it with the manager password.
    {
      value: "conversion",
      to: "/conversion",
      icon: "mdi-account-convert",
      t: "Conversion",
      locked: !auth.isAdmin && !visitorCounterEnabled.value,
    },
    {
      value: "transfers",
      to: "/transfers",
      icon: "mdi-truck",
      t: "Transfers",
      locked: !auth.isAdmin && !pageUnlocks.value.transfers,
    },
    {
      value: "attendance",
      to: "/attendance",
      icon: "mdi-account-clock",
      t: "Attend",
      locked: !auth.isAdmin && !pageUnlocks.value.attendance,
      status: attendanceStatus.value.reachable ? "up" : "down",
    },
    { value: "settings", to: "/settings", icon: "mdi-cog", t: "Settings" },
  ];
  return all.filter((item) => (item.value !== "settings" || auth.isAdmin) && !item.locked);
});
</script>

<style scoped>
.sync-status {
  flex: 0 0 auto;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  padding: 8px 0;
}
.sync-status--link {
  cursor: pointer;
}
.sync-status-word {
  font-size: 0.7rem;
  font-weight: 700;
  line-height: 1.1;
}
.sync-status--ok {
  color: #66bb6a;
}
.sync-status--busy {
  color: #ffb74d;
}
.sync-status--error {
  color: #ef5350;
}
.sync-status--off {
  color: #9e9e9e;
}
.device-status {
  flex: 0 0 auto;
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 4px 8px;
  /* Two icons a row: scanner and printer, keyboard and mouse under them. */
  width: 56px;
  margin: 0 auto;
  padding: 0 0 8px;
}
.device-status--ok {
  color: #66bb6a;
}
.device-status--off {
  color: #ef5350;
}
.device-status--unknown {
  color: #9e9e9e;
}
.nav-drawer {
  background-color: #0f1b2d;
  border: 0 !important;
  border-right: 1px solid #1e2c42 !important;
  box-shadow: none !important;
}

.nav-items-group {
  display: flex;
  flex-direction: column;
  flex: 1 1 0;
  min-height: 0;
  /* Too many pages for the screen scroll here instead of squeezing every
     item; the counter, sync status and Quit below stay pinned. A thin
     scrollbar shows there is more and can be dragged with a mouse;
     pan-y hands vertical finger drags to the scroll, and contain stops a
     drag at the end from moving the page behind. */
  overflow-y: auto;
  overflow-x: hidden;
  touch-action: pan-y;
  overscroll-behavior: contain;
  scrollbar-width: thin;
  scrollbar-color: #3a5378 transparent;
}

.nav-items-group::-webkit-scrollbar {
  width: 4px;
}

.nav-items-group::-webkit-scrollbar-thumb {
  background-color: #3a5378;
  border-radius: 2px;
}

/* Grow to share spare height up to 80px, but never shrink below 64px. */
.nav-item {
  flex: 1 0 64px;
  min-height: 64px;
  max-height: 80px;
  color: #e3ecf8;
}

.nav-item.v-list-item--active {
  background-color: #22375a;
}

.nav-item-content {
  align-items: center;
  display: flex;
  flex-direction: column;
  height: 100%;
  justify-content: center;
  width: 100%;
}

.nav-item :deep(.v-icon) {
  flex-shrink: 0;
  transform: translateZ(0);
}

/* Vuetify clips list-item content; let the status dot stick out above the icon. */
.nav-item :deep(.v-list-item__content) {
  overflow: visible;
}

.nav-icon-wrap {
  position: relative;
  display: inline-flex;
}

.nav-status-dot {
  position: absolute;
  top: -8px;
  right: -11px;
  z-index: 1;
  pointer-events: none;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  border: 2px solid #0f1b2d;
}

.nav-status-dot--up {
  background-color: #43a047;
}

/* Recording-light blink. !important to get past the global animation kill
   in styles/settings.scss, same as the progress-spinner exception there. */
.nav-status-dot--down {
  background-color: #e53935;
  animation: nav-status-pulse 1.4s ease-in-out infinite !important;
}

@keyframes nav-status-pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.2;
  }
}


.nav-item-label {
  font-size: 0.8rem;
  font-weight: 700;
  line-height: 1.2;
}

.options-item {
  flex: 0 0 auto;
  min-height: 0;
  max-height: 80px;
  height: 72px;
}

.visitor-counter {
  flex: 0 0 auto;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  margin: 6px 8px;
  padding: 8px 0;
  color: #ffffff;
}

.visitor-counter-btn {
  width: 40px;
  height: 40px;
  background-color: #d95d00 !important;
  color: #ffffff !important;
}

.visitor-counter-btn :deep(.v-icon) {
  font-size: 32px;
}

.visitor-count {
  font-size: 1.25rem;
  font-weight: 800;
  line-height: 1;
  min-width: 1.5rem;
  text-align: center;
}

.options-grid {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.dialog-close-row {
  display: flex;
  justify-content: flex-end;
}

.options-grid--three {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.options-dialog-card {
  border: 4px dotted rgba(0, 0, 0, 0.45);
}

.option-card {
  align-items: center;
  cursor: pointer;
  display: flex;
  flex-direction: column;
  gap: 12px;
  justify-content: center;
  min-height: 144px;
  padding: 18px 12px;
  text-align: center;
  font-size: var(--kashi-font-size-lg);
  font-weight: 600;
  transition: background-color 0.15s ease;
}

.option-card:hover {
  background-color: rgba(var(--v-theme-primary), 0.12);
}
</style>
