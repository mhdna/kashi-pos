<template>
  <div class="record-photo" :style="{ width: `${size}px`, height: `${size}px` }">
    <img v-if="src" :src="src" alt="" class="record-photo__img" />
    <v-progress-circular v-else-if="pending" indeterminate size="18" width="2" color="grey" />
    <v-icon v-else :icon="emptyIcon" size="22" color="grey" />
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref, watch } from "vue";

// A record's photo thumbnail (an expense receipt, a transfer photo), fetched
// from kashi the first time its row is drawn. The virtual table only draws
// visible rows, so scrolling a long list loads just what's on screen. Results
// are kept for the session by `cacheKey`, so scrolling back or reopening the
// page doesn't fetch again; "" (no photo, or kashi unreachable) shows the icon.

const props = withDefaults(
  defineProps<{
    cacheKey: string;
    load: () => Promise<string>;
    size?: number;
    emptyIcon?: string;
  }>(),
  { size: 40, emptyIcon: "mdi-image-off-outline" },
);

const cache = ((globalThis as any).__recordPhotoCache ??= new Map<string, Promise<string>>()) as Map<string, Promise<string>>;

const src = ref("");
const pending = ref(false);

async function resolve() {
  if (!props.cacheKey) return;
  let job = cache.get(props.cacheKey);
  if (!job) {
    job = props.load().catch(() => "");
    cache.set(props.cacheKey, job);
  }
  pending.value = true;
  src.value = await job;
  pending.value = false;
}

onMounted(resolve);
watch(() => props.cacheKey, () => {
  src.value = "";
  void resolve();
});
</script>

<style scoped>
.record-photo {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  overflow: hidden;
  background: rgba(var(--v-theme-on-surface), 0.06);
  flex-shrink: 0;
}
.record-photo__img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
</style>
