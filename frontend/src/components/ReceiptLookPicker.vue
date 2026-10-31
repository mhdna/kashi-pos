<template>
  <div class="look-summary">
    <div class="look-thumb">
      <img v-if="currentPreview" :src="currentPreview" alt="The receipt look in use" />
      <k-progress-circular v-else indeterminate size="24" />
    </div>
    <div class="look-summary-text">
      <div class="text-subtitle-1 font-weight-medium">{{ currentTheme?.name }} · {{ currentLayout?.name }}</div>
      <div class="text-body-2 text-medium-emphasis">{{ currentTheme?.description }}</div>
      <k-btn class="mt-3" color="primary" prepend-icon="mdi-palette" variant="tonal" @click="openDialog">Change look</k-btn>
    </div>
  </div>

  <k-dialog v-model="dialog" max-width="1000" scrollable>
    <k-card>
      <k-card-title>Receipt look</k-card-title>
      <k-card-text class="look-dialog">
        <div class="look-choices">
          <div class="text-caption text-medium-emphasis mb-1">Theme</div>
          <k-list class="look-themes" density="compact" nav>
            <k-list-item
              v-for="theme in themes"
              :key="theme.id"
              :active="theme.id === pickedTheme"
              rounded="lg"
              :subtitle="theme.best_for"
              :title="theme.name"
              @click="pickTheme(theme.id)"
            >
              <template v-if="theme.id === currentThemeId" #append>
                <k-icon color="success" icon="mdi-check-circle" size="18" />
              </template>
            </k-list-item>
          </k-list>

          <div class="text-caption text-medium-emphasis mt-4 mb-1">Layout</div>
          <k-btn-toggle v-model="pickedLayout" class="look-layouts" color="primary" density="comfortable" divided mandatory variant="outlined">
            <k-btn v-for="layout in pickedThemeInfo?.layouts ?? []" :key="layout.id" :value="layout.id">
              {{ layout.id }}
            </k-btn>
          </k-btn-toggle>
          <div v-if="pickedLayoutInfo" class="mt-2">
            <div class="text-body-2 font-weight-medium">{{ pickedLayoutInfo.name }}</div>
            <div class="text-body-2 text-medium-emphasis">{{ pickedLayoutInfo.description }}</div>
          </div>
          <div v-if="pickedThemeInfo" class="text-caption text-medium-emphasis mt-4">{{ pickedThemeInfo.description }}</div>
        </div>

        <div class="look-preview">
          <img v-if="pickedPreview" :src="pickedPreview" :alt="`${pickedThemeInfo?.name} receipt preview`" />
          <k-progress-circular v-else-if="!previewError" indeterminate />
          <div v-else class="text-body-2 text-error">{{ previewError }}</div>
        </div>
      </k-card-text>
      <k-card-actions>
        <k-spacer />
        <k-btn variant="text" @click="dialog = false">Cancel</k-btn>
        <k-btn color="primary" :disabled="!changed" :loading="saving" variant="flat" @click="save">Use this look</k-btn>
      </k-card-actions>
    </k-card>
  </k-dialog>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { GetReceiptLooks, PreviewReceiptLook, SetReceiptLook } from "../../wailsjs/go/main/App";

interface LookLayout {
  id: string;
  name: string;
  description: string;
}
interface LookTheme {
  id: string;
  name: string;
  description: string;
  best_for: string;
  layouts: LookLayout[];
}

const emit = defineEmits<{ saved: [theme: string, layout: string]; error: [message: string] }>();

const themes = ref<LookTheme[]>([]);
const currentThemeId = ref("classic");
const currentLayoutId = ref("A");
const dialog = ref(false);
const saving = ref(false);
const pickedTheme = ref("classic");
const pickedLayout = ref("A");
const currentPreview = ref("");
const pickedPreview = ref("");
const previewError = ref("");

// A preview is a render of the whole receipt, so each look is fetched once.
const previews = new Map<string, string>();

const currentTheme = computed(() => themes.value.find((t) => t.id === currentThemeId.value));
const currentLayout = computed(() => currentTheme.value?.layouts.find((l) => l.id === currentLayoutId.value));
const pickedThemeInfo = computed(() => themes.value.find((t) => t.id === pickedTheme.value));
const pickedLayoutInfo = computed(() => pickedThemeInfo.value?.layouts.find((l) => l.id === pickedLayout.value));
const changed = computed(() => pickedTheme.value !== currentThemeId.value || pickedLayout.value !== currentLayoutId.value);

async function preview(theme: string, layout: string): Promise<string> {
  const key = `${theme}/${layout}`;
  const cached = previews.get(key);
  if (cached) return cached;
  const url = await PreviewReceiptLook(theme, layout);
  previews.set(key, url);
  return url;
}

async function load() {
  const looks = await GetReceiptLooks();
  themes.value = looks.themes ?? [];
  currentThemeId.value = looks.theme ?? "classic";
  currentLayoutId.value = looks.layout ?? "A";
  try {
    currentPreview.value = await preview(currentThemeId.value, currentLayoutId.value);
  } catch {
    currentPreview.value = "";
  }
}

function openDialog() {
  pickedTheme.value = currentThemeId.value;
  pickedLayout.value = currentLayoutId.value;
  dialog.value = true;
}

// Switching theme lands on that theme's first layout (or keeps the same
// letter when the new theme has it, so A stays A while browsing).
function pickTheme(id: string) {
  const info = themes.value.find((t) => t.id === id);
  if (!info) return;
  pickedTheme.value = id;
  if (!info.layouts.some((l) => l.id === pickedLayout.value)) pickedLayout.value = info.layouts[0]?.id ?? "A";
}

let previewSeq = 0;
watch([pickedTheme, pickedLayout, dialog], async () => {
  if (!dialog.value) return;
  const seq = ++previewSeq;
  previewError.value = "";
  pickedPreview.value = previews.get(`${pickedTheme.value}/${pickedLayout.value}`) ?? "";
  try {
    const url = await preview(pickedTheme.value, pickedLayout.value);
    if (seq === previewSeq) pickedPreview.value = url;
  } catch (error) {
    if (seq === previewSeq) previewError.value = error instanceof Error ? error.message : String(error);
  }
});

async function save() {
  saving.value = true;
  try {
    await SetReceiptLook(pickedTheme.value, pickedLayout.value);
    currentThemeId.value = pickedTheme.value;
    currentLayoutId.value = pickedLayout.value;
    currentPreview.value = pickedPreview.value;
    dialog.value = false;
    emit("saved", pickedTheme.value, pickedLayout.value);
  } catch (error) {
    emit("error", error instanceof Error ? error.message : String(error));
  } finally {
    saving.value = false;
  }
}

onMounted(load);
defineExpose({ reload: load });
</script>

<style scoped>
.look-summary {
  display: flex;
  gap: 16px;
  align-items: flex-start;
}

/* The top of the current receipt, cropped: enough to recognise the look. */
.look-thumb {
  flex: 0 0 140px;
  height: 150px;
  overflow: hidden;
  border: 1px solid rgba(var(--k-border-color), var(--k-border-opacity));
  border-radius: 8px;
  background: #fff;
  display: flex;
  align-items: flex-start;
  justify-content: center;
}

.look-thumb img {
  width: 100%;
  image-rendering: auto;
}

.look-dialog {
  display: grid;
  grid-template-columns: 260px minmax(0, 1fr);
  gap: 20px;
  align-items: start;
}

.look-preview {
  display: flex;
  justify-content: center;
  padding: 12px;
  border-radius: 8px;
  background: rgba(var(--k-theme-on-surface), 0.06);
}

/* The receipt is white paper in either app theme. */
.look-preview img {
  width: min(100%, 380px);
  background: #fff;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.25);
}

@media (max-width: 760px) {
  .look-dialog {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
