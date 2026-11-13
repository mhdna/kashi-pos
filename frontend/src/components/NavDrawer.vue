<template>
  <!-- A permanent drawer: it has no swipe handler, so a drag that starts a
       little sideways still scrolls the page list. -->
  <k-navigation-drawer permanent touchless width="72" class="nav-drawer border-0">
    <k-list class="d-flex flex-column fill-height pa-0" nav>
      <div class="nav-items-group">
        <k-list-item
          v-for="item in items"
          :key="item.value"
          class="nav-item"
          :value="item.value"
          :active="route.path === item.to"
          @click.prevent="handleItemClick(item)"
        >
          <div class="nav-item-content">
            <span class="nav-icon-wrap">
              <k-icon :icon="item.icon" size="24" />
              <span
                v-if="item.status"
                :class="['nav-status-dot', `nav-status-dot--${item.status}`]"
                :aria-label="item.status === 'down' ? 'Disconnected' : 'Connected'"
              />
            </span>
            <span class="nav-item-label text-center mt-1">{{ item.t }}</span>
          </div>
        </k-list-item>
      </div>

      <div v-if="visitorCounterEnabled" class="visitor-counter" aria-label="People visits today">
        <k-btn
          class="visitor-counter-btn"
          icon
          variant="flat"
          rounded="lg"
          aria-label="Increase visitor count"
          @click="adjustVisitors(1)"
        >
          <k-icon icon="mdi-chevron-up" size="32" />
        </k-btn>
        <span class="visitor-count">{{ visitorCount }}</span>
        <k-btn
          class="visitor-counter-btn"
          icon
          variant="flat"
          rounded="lg"
          aria-label="Decrease visitor count"
          @click="adjustVisitors(-1)"
        >
          <k-icon icon="mdi-chevron-down" size="32" />
        </k-btn>
      </div>

      <SyncIndicator />

      <div class="device-panel" role="button" aria-label="Devices" @click="devicesOpen = true">
        <span class="device-panel-title">Devices</span>
        <div class="device-status">
          <k-icon
            v-for="device in deviceIcons"
            :key="device.kind"
            :icon="device.icon"
            :class="`device-status--${device.state}`"
            :title="device.title"
            size="20"
          />
        </div>
      </div>

      <k-list-item
        class="nav-item options-item"
        :value="undefined"
        :active="false"
        @click="optionsOpen = true"
      >
        <div class="nav-item-content">
          <k-icon icon="mdi-logout" size="24" />
          <span class="nav-item-label text-center mt-1">Quit</span>
        </div>
      </k-list-item>
    </k-list>
  </k-navigation-drawer>

  <k-dialog v-model="optionsOpen" max-width="520">
    <k-card class="options-dialog-card" rounded="xl">
      <div class="dialog-close-row px-3 pt-0">
        <k-btn
          icon="mdi-close"
          variant="text"
          aria-label="Close out menu"
          @click="optionsOpen = false"
        />
      </div>

      <k-card-text class="px-3 pt-0 pb-3">
        <section
          v-for="group in QUIT_GROUPS"
          :key="group.title"
          class="options-group"
          :class="`options-group--${group.tone}`"
          :aria-label="group.title"
          :data-test="`quit-group-${group.tone}`"
        >
          <div class="options-grid">
            <k-card
              v-for="option in group.options"
              :key="option.action"
              class="option-card"
              rounded="lg"
              variant="text"
              :data-test="`quit-${option.action}`"
              @click="runQuitOption(option.action)"
            >
              <k-icon :icon="option.icon" size="36" />
              <span>{{ option.label }}</span>
            </k-card>
          </div>
        </section>
      </k-card-text>
    </k-card>
  </k-dialog>

  <DevicesDialog v-model="devicesOpen" />

  <k-dialog v-model="reloadAppConfirmOpen" max-width="360">
    <k-card class="options-dialog-card" rounded="xl" data-test="reload-app-dialog">
      <k-card-title>Reload app</k-card-title>
      <k-card-text>
        Reload the app? It closes and opens again in a few seconds.
        <div v-if="reloadAppError" class="text-error mt-2" data-test="reload-app-error">{{ reloadAppError }}</div>
      </k-card-text>
      <k-card-actions>
        <k-spacer />
        <k-btn variant="text" @click="reloadAppConfirmOpen = false">Cancel</k-btn>
        <k-btn color="primary" variant="flat" data-test="reload-app-confirm" @click="confirmReloadApp">Reload</k-btn>
      </k-card-actions>
    </k-card>
  </k-dialog>

  <k-dialog v-model="closeAppConfirmOpen" max-width="360">
    <k-card class="options-dialog-card" rounded="xl">
      <k-card-title>Close app</k-card-title>
      <k-card-text>Close the app and go to Windows?</k-card-text>
      <k-card-actions>
        <k-spacer />
        <k-btn variant="text" @click="closeAppConfirmOpen = false">Cancel</k-btn>
        <k-btn color="error" variant="flat" @click="confirmCloseApp">Close App</k-btn>
      </k-card-actions>
    </k-card>
  </k-dialog>

  <k-dialog :model-value="powerConfirm !== null" max-width="360" @update:model-value="powerConfirm = null">
    <k-card class="options-dialog-card" rounded="xl">
      <k-card-title>{{ powerConfirm === 'restart' ? 'Restart PC' : 'Shut down PC' }}</k-card-title>
      <k-card-text>
        {{ powerConfirm === 'restart' ? 'Restart this computer now?' : 'Turn this computer off now?' }}
        <div v-if="powerError" class="text-error mt-2">{{ powerError }}</div>
      </k-card-text>
      <k-card-actions>
        <k-spacer />
        <k-btn variant="text" @click="powerConfirm = null">Cancel</k-btn>
        <k-btn color="error" variant="flat" @click="confirmPower">
          {{ powerConfirm === 'restart' ? 'Restart PC' : 'Shut Down PC' }}
        </k-btn>
      </k-card-actions>
    </k-card>
  </k-dialog>

</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { usePageUnlocks } from "@/composables/usePageUnlocks";
import { useVisitorCounter } from "@/composables/useVisitorCounter";
import SyncIndicator from "@/components/SyncIndicator.vue";
import { useDeviceStatus } from "@/composables/useDeviceStatus";
import DevicesDialog from "@/components/DevicesDialog.vue";
import { useAttendanceStatus } from "@/composables/useAttendanceStatus";
import { Quit } from "../../wailsjs/runtime/runtime";
import { RestartApp, RestartComputer, ShutDownComputer } from "../../wailsjs/go/main/App";
import { QUIT_GROUPS, type QuitAction } from "@/lib/quitMenu";

const auth = useAuthStore();
const route = useRoute();
const router = useRouter();
const { pageUnlocks } = usePageUnlocks();
const {
  enabled: visitorCounterEnabled,
  count: visitorCount,
  adjust: adjustVisitors,
} = useVisitorCounter();

const { status: attendanceStatus } = useAttendanceStatus();

const { devices: deviceIcons } = useDeviceStatus();
const devicesOpen = ref(false);

const optionsOpen = ref(false);
const closeAppConfirmOpen = ref(false);
const reloadAppConfirmOpen = ref(false);
const reloadAppError = ref("");

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

function askReloadApp() {
  optionsOpen.value = false;
  reloadAppError.value = "";
  reloadAppConfirmOpen.value = true;
}

async function confirmReloadApp() {
  try {
    await RestartApp();
  } catch (err) {
    reloadAppError.value = String(err);
  }
}

// What each Quit-menu option does (the list itself is lib/quitMenu.ts).
function runQuitOption(action: QuitAction) {
  if (action === "close-shift") openCloseShift();
  else if (action === "logout") logout();
  else if (action === "reload-app") askReloadApp();
  else if (action === "close-app") closeApp();
  else if (action === "restart-pc") askPower("restart");
  else askPower("shutdown");
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
.device-panel {
  flex: 0 0 auto;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  margin: 0 6px 4px;
  padding: 4px 0 5px;
  background: rgba(0, 0, 0, 0.35);
  border: 1px solid #243653;
  border-radius: 8px;
  cursor: pointer;
}
.device-panel-title {
  font-size: 0.65rem;
  font-weight: 700;
  line-height: 1.1;
  color: #9e9e9e;
}
.device-status {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 4px 8px;
  /* Two icons a row: scanner and printer, keyboard and mouse under them. */
  width: 56px;
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
  background: linear-gradient(180deg, #13243d 0%, #0d1828 100%);
  border: 0 !important;
  border-right: 1px solid #243653 !important;
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
  position: relative;
  /* Quieter than the active page, so the current one stands out. */
  color: #9fb3d1;
  transition: background-color 0.15s ease, color 0.15s ease;
}

.nav-item:hover {
  background-color: rgba(255, 255, 255, 0.06);
  color: #e3ecf8;
}

/* The page you are on: lighter panel, bright text and an accent bar. */
.nav-item.k-list-item--active {
  background: linear-gradient(90deg, rgba(79, 140, 255, 0.28), rgba(79, 140, 255, 0.1));
  color: #ffffff;
}

.nav-item.k-list-item--active::before {
  content: "";
  position: absolute;
  left: 0;
  top: 12%;
  bottom: 12%;
  width: 4px;
  border-radius: 0 4px 4px 0;
  background: #6ea8ff;
}

.nav-item-content {
  align-items: center;
  display: flex;
  flex-direction: column;
  height: 100%;
  justify-content: center;
  width: 100%;
}

.nav-item :deep(.k-icon) {
  flex-shrink: 0;
  transform: translateZ(0);
}

/* Let the status dot stick out above the icon rather than being clipped. */
.nav-item :deep(.k-list-item__content) {
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
  border: 2px solid #0f1b2d; /* matches the sidebar */
}

.nav-status-dot--up {
  background-color: #43a047;
}

/* Recording-light blink, only with animations on (Settings); off, it is a
   solid red dot. !important to get past the global animation kill in
   styles/settings.scss, same as the progress-spinner exception there. */
.nav-status-dot--down {
  background-color: #e53935;
}

:root:not(.animations-disabled) .nav-status-dot--down {
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
  margin: 2px 6px 6px;
  padding: 6px 0;
  color: #ffffff;
  /* One panel, like the devices box under it. */
  background: rgba(0, 0, 0, 0.35);
  border: 1px solid #243653;
  border-radius: 12px;
}

/* Up adds a visitor (teal), down takes one off (muted slate): two clearly
   different buttons instead of two identical orange ones. */
.visitor-counter-btn {
  width: 40px;
  height: 36px;
  color: #ffffff !important;
  background-color: #1f9d8a !important;
}

.visitor-counter-btn:last-of-type {
  background-color: #3a5378 !important;
}

.visitor-counter-btn :deep(.k-icon) {
  font-size: 32px;
}

.visitor-count {
  font-size: 1.4rem;
  font-weight: 800;
  line-height: 1;
  min-width: 1.5rem;
  text-align: center;
  font-variant-numeric: tabular-nums;
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

.options-group + .options-group {
  margin-top: 8px;
}
/* The Quit menu is compact: shorter cards, tighter gaps. */
.options-group .options-grid {
  gap: 8px;
}
.options-group .option-card {
  min-height: 92px;
  padding: 8px 10px;
  gap: 6px;
}
/* No captions: each group has its own icon colour instead. Green for the
   shift, blue for the app, red for the computer; the cards stay plain. */
.options-group--shift { --option-color: 46, 125, 50; }
.options-group--app { --option-color: 25, 118, 210; }
.options-group--computer { --option-color: 211, 47, 47; }
.options-group .option-card :deep(.k-icon) {
  color: rgb(var(--option-color));
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
  background-color: rgba(var(--k-theme-primary), 0.12);
}
</style>
