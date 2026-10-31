<template>
  <k-card class="settings-card">
    <k-card-title class="d-flex align-center ga-3">
      <k-icon icon="mdi-update" size="20" />
      App updates
    </k-card-title>
    <div class="text-caption text-medium-emphasis px-4 pb-2">
      Updates are sent from kashi's App Updates page and install at a safe moment.
    </div>
    <k-card-text>
      <div class="mb-1">
        This till runs <strong>{{ status.currentVersion }}</strong>
        <span v-if="status.flavor === 'compact'" class="text-medium-emphasis">(compact)</span>
      </div>
      <div class="mb-3" :class="stateColor ? `text-${stateColor}` : 'text-medium-emphasis'">{{ stateText }}</div>
      <k-progress-linear
        v-if="status.state === 'downloading'"
        class="mb-3"
        color="info"
        height="6"
        :model-value="status.progress"
        rounded
      />
      <k-alert v-if="error" type="error" density="compact" class="mb-3">{{ error }}</k-alert>
      <div class="d-flex flex-wrap ga-2">
        <k-btn
          :disabled="!status.enabled"
          :loading="checking"
          prepend-icon="mdi-refresh"
          size="small"
          variant="tonal"
          @click="check"
        >
          Check for updates
        </k-btn>
        <k-btn
          v-if="status.state === 'ready' && status.supported"
          color="primary"
          prepend-icon="mdi-download"
          size="small"
          variant="flat"
          @click="install"
        >
          Install {{ status.version }} now
        </k-btn>
        <k-btn
          v-if="status.canRollBack && auth.isAdmin"
          prepend-icon="mdi-backup-restore"
          size="small"
          variant="text"
          @click="confirmRollback = true"
        >
          Roll back to {{ status.previousVersion }}
        </k-btn>
      </div>
    </k-card-text>

    <k-dialog v-model="confirmRollback" max-width="440">
      <k-card :title="`Roll back to ${status.previousVersion}?`">
        <k-card-text>
          GB Cloud closes and reopens on {{ status.previousVersion }} in about 10 seconds. It won't
          install {{ status.currentVersion }} again until it's sent a different version or you
          check for updates here. Your data is not changed.
        </k-card-text>
        <k-card-actions>
          <k-spacer />
          <k-btn variant="text" @click="confirmRollback = false">Cancel</k-btn>
          <k-btn color="primary" variant="flat" @click="rollBack">Roll back</k-btn>
        </k-card-actions>
      </k-card>
    </k-dialog>
  </k-card>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { useAuthStore } from "@/stores/auth";
import { CheckForAppUpdate, GetAppUpdateStatus, InstallAppUpdate, RollBackAppUpdate } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";

// Settings > Admin: this till's version and update, see updater.go.
const auth = useAuthStore();
const status = ref<Record<string, any>>({});
const checking = ref(false);
const confirmRollback = ref(false);
const error = ref("");
let off: (() => void) | undefined;

const stateText = computed(() => {
  const s = status.value;
  if (!s.enabled) return "Remote updates are off in this build.";
  switch (s.state) {
    case "downloading":
      return `Downloading ${s.version}… ${s.progress}%`;
    case "ready":
      return s.supported ? `${s.version} is downloaded and ready to install.` : `${s.version} is downloaded (installs only on Windows).`;
    case "installing":
      return `Installing ${s.version}…`;
    case "updated":
      return s.detail || "Updated.";
    case "failed":
    case "blocked":
    case "rolled_back":
      return s.detail;
    default:
      return "Up to date.";
  }
});
const stateColor = computed(() =>
  ({ failed: "error", blocked: "error", rolled_back: "warning", ready: "primary", updated: "success" } as Record<string, string>)[status.value.state] ?? "",
);

async function check() {
  checking.value = true;
  error.value = "";
  try {
    status.value = await CheckForAppUpdate();
  } finally {
    setTimeout(() => (checking.value = false), 1500);
  }
}

async function install() {
  error.value = "";
  try {
    await InstallAppUpdate();
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  }
}

async function rollBack() {
  confirmRollback.value = false;
  error.value = "";
  try {
    await RollBackAppUpdate();
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  }
}

onMounted(async () => {
  off = EventsOn("app_update_state", (s: Record<string, any>) => (status.value = s));
  status.value = await GetAppUpdateStatus();
});
onUnmounted(() => off?.());
</script>
