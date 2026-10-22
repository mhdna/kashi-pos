<template>
  <!-- mousedown.prevent keeps the phone field focused (and its caret where
       it was) while a key is tapped. -->
  <div class="phone-pad">
    <v-btn
      v-for="key in visibleKeys"
      :key="key.value"
      class="phone-pad-key"
      :class="{
        'phone-pad-key--clear': key.value === 'clear',
        'phone-pad-key--arrow': !!key.icon,
      }"
      size="small"
      variant="tonal"
      :aria-label="key.label"
      @mousedown.prevent
      @click="press(key.value)"
    >
      <v-icon v-if="key.icon" :icon="key.icon" />
      <template v-else>{{ key.text }}</template>
    </v-btn>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { playClickSound, playDtmfSound } from "@/lib/sound";

const props = withDefaults(defineProps<{ showArrows?: boolean }>(), { showArrows: true });
const emit = defineEmits<{ press: [key: string] }>();

type Key = { value: string; label: string; text?: string; icon?: string };
const digit = (d: string): Key => ({ value: d, label: d, text: d });
const keys: Key[] = [
  ...["1", "2", "3", "4", "5", "6", "7", "8", "9"].map(digit),
  { value: "clear", label: "Clear phone number", text: "C" },
  digit("0"),
  { value: "back", label: "Delete digit", text: "⌫" },
  { value: "left", label: "Move cursor left", icon: "mdi-chevron-left" },
  { value: "right", label: "Move cursor right", icon: "mdi-chevron-right" },
];

const visibleKeys = computed(() => (props.showArrows ? keys : keys.filter((key) => !key.icon)));

function press(key: string) {
  if (/^\d$/.test(key)) playDtmfSound(key);
  else playClickSound();
  emit("press", key);
}
</script>

<style scoped>
.phone-pad {
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  gap: 6px;
  max-width: 360px;
  margin-inline: auto;
}

/* Six columns: three keys a row, and the two arrows half each at the foot. */
.phone-pad-key {
  grid-column: span 2;
  height: 52px !important;
  min-width: 0;
  padding: 0;
  border: 1px solid rgba(var(--v-border-color), 0.18);
  border-radius: 4px;
  font-size: 2rem !important;
  transition:
    border-color 120ms ease,
    background-color 120ms ease;
}

.phone-pad-key:hover {
  border-color: currentColor;
}

.phone-pad-key--arrow {
  grid-column: span 3;
}

.phone-pad-key--clear {
  color: rgb(var(--v-theme-error));
}
</style>
