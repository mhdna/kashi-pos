<template>
  <!-- An update is downloaded and waiting. -->
  <k-snackbar v-model="showReady" :timeout="-1" location="bottom" color="primary">
    <k-icon icon="mdi-update" class="me-2" />
    GB Cloud {{ status.version }} is ready.
    <template #actions>
      <k-btn variant="text" @click="dismissed = true">Later</k-btn>
      <k-btn variant="flat" @click="confirmInstall = true">Install now</k-btn>
    </template>
  </k-snackbar>

  <k-dialog v-model="confirmInstall" max-width="440">
    <k-card title="Install the update?">
      <k-card-text>GB Cloud will close and reopen in about 10 seconds. Finish the current sale first.</k-card-text>
      <k-card-actions>
        <k-spacer />
        <k-btn variant="text" @click="confirmInstall = false">Cancel</k-btn>
        <k-btn color="primary" variant="flat" @click="install">Install now</k-btn>
      </k-card-actions>
    </k-card>
  </k-dialog>

  <!-- No shift open: install by itself unless the cashier says not now.
       Forced from kashi: install even mid-shift, and no "not now". -->
  <k-dialog v-model="idlePrompt" max-width="440" persistent>
    <k-card :title="status.force ? 'A required update' : 'Installing an update'">
      <k-card-text v-if="status.force">
        GB Cloud {{ status.version }} installs in {{ countdown }} seconds. Finish the current sale now; GB Cloud
        closes and reopens by itself.
      </k-card-text>
      <k-card-text v-else>No shift is open, so GB Cloud {{ status.version }} installs in {{ countdown }} seconds.</k-card-text>
      <k-card-actions>
        <k-spacer />
        <k-btn v-if="!status.force" variant="text" @click="cancelIdle">Not now</k-btn>
        <k-btn color="primary" variant="flat" @click="install">Install now</k-btn>
      </k-card-actions>
    </k-card>
  </k-dialog>

  <!-- Covers the whole app while the update installs: GB Cloud closes and
       reopens by itself, so there is nothing to press. -->
  <k-overlay
    :model-value="installing"
    persistent
    scrim="background"
    :opacity="0.97"
    class="align-center justify-center"
  >
    <div class="d-flex flex-column align-center text-center">
      <k-progress-circular indeterminate color="primary" size="88" width="3">
        <k-icon icon="mdi-update" size="36" color="primary" />
      </k-progress-circular>
      <div class="update-title mt-8">Updating GB Cloud</div>
      <div v-if="status.version" class="update-version text-medium-emphasis mt-1">Version {{ status.version }}</div>
      <div class="text-body-2 text-medium-emphasis mt-8">Back in a few seconds. Keep the computer on.</div>
    </div>
  </k-overlay>

  <!-- After a restart: what happened. -->
  <k-dialog v-model="resultDialog" max-width="480">
    <k-card :title="resultTitle">
      <k-card-text style="white-space: pre-wrap">{{ resultText }}</k-card-text>
      <k-card-actions>
        <k-spacer />
        <k-btn color="primary" variant="flat" @click="resultDialog = false">OK</k-btn>
      </k-card-actions>
    </k-card>
  </k-dialog>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { ConfirmAppStarted, GetAppUpdateStatus, InstallAppUpdate, WaitUntilReady } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";

// The cashier's side of remote updates (updater.go): a banner when one is
// ready, a cancellable countdown while no shift is open, an "Updating…"
// screen, and after the restart what happened.
const status = ref<Record<string, any>>({});
const dismissed = ref(false);
const confirmInstall = ref(false);
const idlePrompt = ref(false);
const countdown = ref(30);
const installing = ref(false);
const resultDialog = ref(false);
const resultTitle = ref("");
const resultText = ref("");
const showReady = computed({
  get: () => status.value.state === "ready" && status.value.supported && !dismissed.value && !installing.value,
  set: (v) => { if (!v) dismissed.value = true; },
});
let timer: ReturnType<typeof setInterval> | undefined;
const offs: Array<() => void> = [];

async function install() {
  confirmInstall.value = false;
  cancelIdle();
  installing.value = true;
  try {
    await InstallAppUpdate();
  } catch (error) {
    installing.value = false;
    resultTitle.value = "The update couldn't install";
    resultText.value = error instanceof Error ? error.message : String(error);
    resultDialog.value = true;
  }
}

function cancelIdle() {
  idlePrompt.value = false;
  if (timer) clearInterval(timer);
}

onMounted(async () => {
  offs.push(EventsOn("app_update_state", (s: Record<string, any>) => {
    if (s.state === "ready" && status.value.version !== s.version) dismissed.value = false;
    status.value = s;
  }));
  offs.push(EventsOn("app_update_installing", () => (installing.value = true)));
  offs.push(EventsOn("app_update_prompt", (s: Record<string, any>) => {
    status.value = s;
    countdown.value = s.force ? 60 : 30;
    idlePrompt.value = true;
    timer = setInterval(() => {
      countdown.value -= 1;
      if (countdown.value <= 0) void install();
    }, 1000);
  }));
  const ready = await WaitUntilReady();
  status.value = await GetAppUpdateStatus();
  // Only a launch that opened its database counts as started; otherwise the
  // update helper rolls back.
  if (ready.error) return;
  const started = await ConfirmAppStarted();
  if (started.result === "update") {
    resultTitle.value = `GB Cloud was updated to ${started.to}`;
    resultText.value = started.notes ? `What's new:\n${started.notes}` : "";
    resultDialog.value = true;
  } else if (started.result === "rollback") {
    resultTitle.value = `GB Cloud went back to ${started.to}`;
    resultDialog.value = true;
  } else if (started.result === "failed") {
    resultTitle.value = "The update didn't work";
    resultText.value = `${started.from ? `${started.to} didn't start, so ` : ""}GB Cloud went back to the previous version. Your data is safe.`;
    resultDialog.value = true;
  }
});

onUnmounted(() => {
  offs.forEach((off) => off());
  cancelIdle();
});
</script>

<style scoped>
/* Set here rather than with text-h5/-body-1, which compact.scss shrinks for
   the dense till screens; this is a full-screen message. */
.update-title {
  font-size: 1.75rem;
  font-weight: 500;
  letter-spacing: -0.01em;
}
.update-version {
  font-size: 1.05rem;
}
</style>
