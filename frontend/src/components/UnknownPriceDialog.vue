<template>
  <v-dialog :model-value="modelValue" max-width="360" @update:model-value="emit('update:modelValue', $event)">
    <v-card class="pa-4">
      <v-card-title class="px-0 pt-0 text-center text-h5">Unknown item</v-card-title>
      <div class="text-h5 text-medium-emphasis text-center mb-3">
        Type the price<br />
        <span dir="rtl" lang="ar">أدخل سعر القطعة</span>
      </div>
      <div class="unknown-price-display text-center mb-3" data-testid="unknown-price">
        ${{ display }}
      </div>
      <div class="unknown-keypad">
        <v-btn v-for="key in keys" :key="key" class="unknown-key" variant="tonal" :aria-label="key" @click="press(key)">
          {{ key }}
        </v-btn>
        <v-btn class="unknown-key" variant="tonal" aria-label="Delete" @click="backspace">
          <v-icon icon="mdi-backspace-outline" size="26" />
        </v-btn>
      </div>
      <v-card-actions class="px-0 pb-0 mt-3">
        <v-btn variant="text" @click="emit('update:modelValue', false)">Cancel</v-btn>
        <v-spacer />
        <v-btn color="primary" :disabled="!(price > 0)" variant="flat" @click="add">Add</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";

// The price of an "Unknown item" line, typed on a keypad (this till is a
// touch screen). Opening it starts from zero each time.
const props = defineProps<{ modelValue: boolean }>();
const emit = defineEmits<{
  "update:modelValue": [open: boolean];
  add: [price: number];
}>();

const keys = ["1", "2", "3", "4", "5", "6", "7", "8", "9", ".", "0"];
const typed = ref("");

const display = computed(() => typed.value || "0");
const price = computed(() => Number(typed.value) || 0);

function press(key: string) {
  if (key === ".") {
    if (typed.value.includes(".")) return;
    typed.value = (typed.value || "0") + ".";
    return;
  }
  const [whole, cents] = typed.value.split(".");
  if (cents !== undefined && cents.length >= 2) return;
  if (cents === undefined && (whole ?? "").length >= 6) return;
  typed.value = typed.value === "0" ? key : typed.value + key;
}

function backspace() {
  typed.value = typed.value.slice(0, -1);
}

function add() {
  if (!(price.value > 0)) return;
  emit("add", Math.round(price.value * 100) / 100);
  emit("update:modelValue", false);
}

// A physical keyboard (or a scanner's keypad) works too.
function onKeydown(event: KeyboardEvent) {
  if (/^[0-9.]$/.test(event.key)) press(event.key);
  else if (event.key === "Backspace") backspace();
  else if (event.key === "Enter") add();
  else return;
  event.preventDefault();
}

watch(
  () => props.modelValue,
  (open) => {
    if (open) {
      typed.value = "";
      window.addEventListener("keydown", onKeydown, true);
    } else {
      window.removeEventListener("keydown", onKeydown, true);
    }
  },
);
onBeforeUnmount(() => window.removeEventListener("keydown", onKeydown, true));
</script>

<style scoped>
.unknown-price-display {
  font-size: 2.25rem;
  font-weight: 500;
  line-height: 1.2;
}
.unknown-keypad {
  display: grid;
  gap: 6px;
  grid-auto-rows: 56px;
  grid-template-columns: repeat(3, 1fr);
  margin: 0 auto;
  width: min(100%, 280px);
}
.unknown-key {
  border: 1px solid rgba(var(--v-border-color), 0.18);
  border-radius: 4px;
  font-size: 2rem !important;
  height: 100% !important;
  min-width: 0;
  padding: 0;
}
</style>
