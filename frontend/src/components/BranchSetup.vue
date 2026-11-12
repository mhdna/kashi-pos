<script setup lang="ts">
import { computed, reactive, ref } from "vue";
import { CreateBranch, SwitchBranch } from "../../wailsjs/go/main/App";
import { WindowReloadApp } from "../../wailsjs/runtime/runtime";
import logo from "@/assets/app-logo.png";
import { canSubmit, connectionHint, connectionMode, createBranchArgs, normalizeBranchCode, setupErrors } from "@/lib/branchSetup";

// First run on a machine (or its branch database went missing): set up a
// branch, or pick one already set up here. The central key and server are
// masked like passwords, with an eye to check what was pasted.
const props = defineProps<{ knownBranches: { name: string; code: string }[]; startupError?: string }>();

const adding = ref(false);
const showCreate = computed(() => props.knownBranches.length === 0 || adding.value);
const form = reactive({ name: "", code: "", apiKey: "", centralApiUrl: "" });
const showKey = ref(false);
const showUrl = ref(false);
const touched = ref(false);
const loading = ref(false);
const error = ref("");

const errors = computed(() => (touched.value ? setupErrors(form) : {}));
const hint = computed(() => connectionHint(connectionMode(form)));
const hintIcon = { ok: "mdi-check-circle", info: "mdi-information-outline", warn: "mdi-alert-circle-outline" } as const;

function onCode(value: string) {
  form.code = normalizeBranchCode(value);
}

async function run(fn: () => Promise<unknown>) {
  loading.value = true;
  error.value = "";
  try {
    await fn();
    WindowReloadApp();
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    loading.value = false;
  }
}

function submit() {
  touched.value = true;
  if (!canSubmit(form)) return;
  run(() => CreateBranch(...createBranchArgs(form)));
}

function choose(code: string) {
  run(() => SwitchBranch(code));
}
</script>

<template>
  <div class="setup" data-test="branch-setup">
    <div class="setup-card">
      <header class="setup-head">
        <img :src="logo" alt="" class="setup-logo" />
        <div>
          <h1 class="setup-title">{{ showCreate ? "Set up this till" : "Choose a branch" }}</h1>
          <p class="setup-sub">
            {{ showCreate
              ? "Name the branch this till belongs to, and connect it to central if you have a key."
              : "More than one branch is set up on this machine. Pick the one this till runs." }}
          </p>
        </div>
      </header>

      <k-alert v-if="startupError" type="warning" density="compact" class="mb-4" data-test="setup-startup-error">
        {{ startupError }}
      </k-alert>

      <k-form v-if="showCreate" data-test="setup-form" @submit.prevent="submit">
        <section class="setup-section">
          <div class="setup-section-head">
            <span class="setup-section-icon"><k-icon icon="mdi-storefront-outline" size="18" /></span>
            <span>Branch</span>
          </div>
          <div class="setup-row">
            <k-text-field
              v-model="form.name"
              label="Branch name"
              placeholder="Centro"
              prepend-inner-icon="mdi-store"
              :error-messages="errors.name"
              autofocus
              class="setup-grow"
              data-test="setup-name"
            />
            <k-text-field
              :model-value="form.code"
              label="Code"
              placeholder="CN"
              prepend-inner-icon="mdi-tag-outline"
              :error-messages="errors.code"
              maxlength="8"
              class="setup-code"
              data-test="setup-code"
              @update:model-value="onCode"
            />
          </div>
        </section>

        <section class="setup-section">
          <div class="setup-section-head">
            <span class="setup-section-icon"><k-icon icon="mdi-cloud-sync-outline" size="18" /></span>
            <span>Central</span>
            <span class="setup-optional">optional</span>
          </div>
          <k-text-field
            v-model="form.apiKey"
            label="Central API key"
            :type="showKey ? 'text' : 'password'"
            prepend-inner-icon="mdi-key-variant"
            :append-inner-icon="showKey ? 'mdi-eye-off' : 'mdi-eye'"
            autocomplete="off"
            hide-details
            class="mb-3"
            data-test="setup-key"
            @click:append-inner="showKey = !showKey"
          />
          <k-text-field
            v-model="form.centralApiUrl"
            label="Central server address"
            :type="showUrl ? 'text' : 'password'"
            prepend-inner-icon="mdi-web"
            :append-inner-icon="showUrl ? 'mdi-eye-off' : 'mdi-eye'"
            autocomplete="off"
            hide-details
            data-test="setup-url"
            @click:append-inner="showUrl = !showUrl"
          />
          <div :class="['setup-hint', `setup-hint--${hint.tone}`]" data-test="setup-hint">
            <k-icon :icon="hintIcon[hint.tone]" size="16" />
            <span>{{ hint.text }}</span>
          </div>
        </section>

        <k-alert v-if="error" type="error" density="compact" class="mb-3" data-test="setup-error">{{ error }}</k-alert>

        <div class="setup-actions">
          <k-btn v-if="knownBranches.length > 0" variant="text" prepend-icon="mdi-arrow-left" @click="adding = false">
            Back
          </k-btn>
          <k-spacer />
          <k-btn type="submit" color="primary" variant="flat" size="large" prepend-icon="mdi-check" :loading="loading" data-test="setup-submit">
            Create branch
          </k-btn>
        </div>
      </k-form>

      <template v-else>
        <div class="setup-branches">
          <button
            v-for="branch in knownBranches"
            :key="branch.code"
            type="button"
            class="setup-branch"
            :disabled="loading"
            :data-test="`setup-branch-${branch.code}`"
            @click="choose(branch.code)"
          >
            <span class="setup-branch-avatar"><k-icon icon="mdi-store" size="20" /></span>
            <span class="setup-branch-text">
              <span class="setup-branch-name">{{ branch.name }}</span>
              <span class="setup-branch-code">{{ branch.code }}</span>
            </span>
            <k-icon icon="mdi-chevron-right" size="20" class="setup-branch-go" />
          </button>
        </div>
        <k-alert v-if="error" type="error" density="compact" class="mt-3" data-test="setup-error">{{ error }}</k-alert>
        <div class="setup-actions">
          <k-btn variant="text" prepend-icon="mdi-plus" :disabled="loading" data-test="setup-add" @click="adding = true">
            Set up another branch
          </k-btn>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.setup {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 24px;
  background:
    radial-gradient(1200px 600px at 10% -10%, rgba(var(--k-theme-primary), 0.18), transparent 60%),
    radial-gradient(900px 500px at 110% 110%, rgba(var(--k-theme-primary), 0.12), transparent 60%),
    rgb(var(--k-theme-background));
}
.setup-card {
  width: 100%;
  max-width: 520px;
  padding: 28px 28px 20px;
  border-radius: 20px;
  background: rgb(var(--k-theme-surface));
  box-shadow: 0 18px 50px rgba(0, 0, 0, 0.18), 0 2px 6px rgba(0, 0, 0, 0.08);
}
.setup-head {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 22px;
}
.setup-logo {
  width: 56px;
  height: 56px;
  border-radius: 14px;
  object-fit: contain;
  flex: 0 0 auto;
}
.setup-title {
  margin: 0;
  font-size: 1.4rem;
  font-weight: 700;
  line-height: 1.2;
}
.setup-sub {
  margin: 4px 0 0;
  font-size: 0.9rem;
  opacity: 0.72;
}
.setup-section {
  padding: 16px;
  margin-bottom: 14px;
  border-radius: 14px;
  border: 1px solid rgba(var(--k-theme-on-surface), 0.1);
}
.setup-section-head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
  font-weight: 600;
}
.setup-section-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  border-radius: 50%;
  color: rgb(var(--k-theme-primary));
  background: rgba(var(--k-theme-primary), 0.12);
}
.setup-optional {
  margin-left: auto;
  font-size: 0.72rem;
  font-weight: 500;
  padding: 2px 8px;
  border-radius: 999px;
  background: rgba(var(--k-theme-on-surface), 0.07);
  opacity: 0.8;
}
.setup-row {
  display: flex;
  gap: 12px;
}
.setup-grow { flex: 1 1 auto; min-width: 0; }
.setup-code { flex: 0 0 130px; }
.setup-hint {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  margin-top: 12px;
  font-size: 0.82rem;
  line-height: 1.35;
}
.setup-hint--ok { color: rgb(var(--k-theme-success)); }
.setup-hint--info { opacity: 0.72; }
.setup-hint--warn { color: rgb(var(--k-theme-warning)); }
.setup-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 8px;
}
.setup-branches {
  display: grid;
  gap: 8px;
}
.setup-branch {
  display: flex;
  align-items: center;
  gap: 14px;
  width: 100%;
  padding: 12px 14px;
  border-radius: 14px;
  border: 1px solid rgba(var(--k-theme-on-surface), 0.1);
  background: transparent;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
}
.setup-branch:hover:not(:disabled) {
  border-color: rgba(var(--k-theme-primary), 0.5);
  background: rgba(var(--k-theme-primary), 0.06);
}
.setup-branch:disabled { opacity: 0.6; cursor: default; }
.setup-branch-avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  border-radius: 50%;
  color: rgb(var(--k-theme-primary));
  background: rgba(var(--k-theme-primary), 0.12);
}
.setup-branch-text { display: flex; flex-direction: column; min-width: 0; flex: 1 1 auto; }
.setup-branch-name { font-weight: 600; }
.setup-branch-code { font-size: 0.8rem; opacity: 0.65; }
.setup-branch-go { opacity: 0.5; }
@media (max-width: 480px) {
  .setup-row { flex-direction: column; gap: 0; }
  .setup-code { flex-basis: auto; }
}
</style>
