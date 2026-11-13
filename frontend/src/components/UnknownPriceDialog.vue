<template>
  <k-dialog :model-value="modelValue" :max-width="step === 'name' ? 680 : 360" @update:model-value="emit('update:modelValue', $event)">
    <k-card class="pa-4">
      <!-- Step 1: the price, on a number pad. -->
      <template v-if="step === 'price'">
        <k-card-title class="px-0 pt-0 text-center text-h5">Unknown item</k-card-title>
        <div class="text-h5 text-medium-emphasis text-center mb-3">
          Type the price<br />
          <span dir="rtl" lang="ar">أدخل سعر القطعة</span>
        </div>
        <div class="unknown-price-display text-center mb-3" data-testid="unknown-price">
          ${{ display }}
        </div>
        <div class="unknown-keypad">
          <k-btn v-for="key in keys" :key="key" class="unknown-key" variant="tonal" :aria-label="key" @click="press(key)">
            {{ key }}
          </k-btn>
          <k-btn class="unknown-key" variant="tonal" aria-label="Delete" @click="backspace">
            <k-icon icon="mdi-backspace-outline" size="26" />
          </k-btn>
        </div>
        <k-card-actions class="px-0 pb-0 mt-3">
          <k-btn variant="text" @click="emit('update:modelValue', false)">Cancel</k-btn>
          <k-spacer />
          <k-btn color="primary" :disabled="!(price > 0)" variant="flat" data-test="unknown-next" @click="toName">Next</k-btn>
        </k-card-actions>
      </template>

      <!-- Step 2: what it is, on the on-screen keyboard, so the invoice list and
           the receipt say it instead of "Unknown item". -->
      <template v-else>
        <k-card-title class="px-0 pt-0 text-center text-h5">Name the item</k-card-title>
        <div class="text-h6 text-medium-emphasis text-center mb-3">
          What is it? <span dir="rtl" lang="ar">ما هي القطعة؟</span>
          <span class="unknown-name-price">${{ display }}</span>
        </div>
        <div class="unknown-name-display mb-3" :class="{ 'unknown-name-display--empty': !name }" data-testid="unknown-name">
          {{ name || "Name" }}
        </div>
        <InlineKeyboard v-model="name" layout="code" :max-length="MAX_NAME" />
        <k-card-actions class="px-0 pb-0 mt-3">
          <k-btn variant="text" data-test="unknown-back" @click="step = 'price'">Back</k-btn>
          <k-spacer />
          <k-btn color="primary" :disabled="!trimmedName" variant="flat" data-test="unknown-add" @click="add">Add</k-btn>
        </k-card-actions>
      </template>
    </k-card>
  </k-dialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";
import InlineKeyboard from "@/components/InlineKeyboard.vue";

// An "Unknown item" line: first its price, typed on a keypad (this till is a
// touch screen), then its name, typed on the on-screen keyboard. The name is
// what the invoice list and the receipt call it. Opening it starts afresh.
const props = defineProps<{ modelValue: boolean }>();
const emit = defineEmits<{
  "update:modelValue": [open: boolean];
  add: [price: number, name: string];
}>();

// The same limit the till enforces when it saves the sale (maxItemNameRunes).
const MAX_NAME = 80;

const keys = ["1", "2", "3", "4", "5", "6", "7", "8", "9", ".", "0"];
const step = ref<"price" | "name">("price");
const typed = ref("");
const name = ref("");

const display = computed(() => typed.value || "0");
const price = computed(() => Number(typed.value) || 0);
const trimmedName = computed(() => name.value.trim());

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

function toName() {
  if (!(price.value > 0)) return;
  step.value = "name";
}

function add() {
  if (!(price.value > 0) || !trimmedName.value) return;
  emit("add", Math.round(price.value * 100) / 100, trimmedName.value);
  emit("update:modelValue", false);
}

// A physical keyboard (or a scanner's keypad) works too: digits for the price,
// then letters for the name.
function onKeydown(event: KeyboardEvent) {
  if (step.value === "price") {
    if (/^[0-9.]$/.test(event.key)) press(event.key);
    else if (event.key === "Backspace") backspace();
    else if (event.key === "Enter") toName();
    else return;
  } else {
    if (event.key === "Enter") add();
    else if (event.key === "Backspace") name.value = name.value.slice(0, -1);
    else if (event.key.length === 1 && !event.ctrlKey && !event.metaKey && !event.altKey) {
      if (name.value.length < MAX_NAME) name.value += event.key;
    } else return;
  }
  event.preventDefault();
}

watch(
  () => props.modelValue,
  (open) => {
    if (open) {
      step.value = "price";
      typed.value = "";
      name.value = "";
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
.unknown-name-price {
  display: block;
  margin-top: 2px;
  font-size: 1.5rem;
  font-weight: 500;
  color: rgb(var(--k-theme-on-surface));
}
.unknown-name-display {
  min-height: 56px;
  padding: 12px 16px;
  border: 2px solid rgba(var(--k-theme-primary), 0.7);
  border-radius: 8px;
  font-size: 1.75rem;
  line-height: 1.2;
  text-align: center;
  overflow-wrap: anywhere;
}
.unknown-name-display--empty {
  color: rgba(var(--k-theme-on-surface), 0.4);
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
  border: 1px solid rgba(var(--k-border-color), 0.18);
  border-radius: 4px;
  font-size: 2rem !important;
  height: 100% !important;
  min-width: 0;
  padding: 0;
}
</style>
