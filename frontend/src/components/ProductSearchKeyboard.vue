<template>
  <div class="product-search-keyboard" @mousedown="keepFieldFocus">
    <div class="product-search-row">
      <k-text-field
        ref="searchField"
        :model-value="modelValue"
        class="product-search-control"
        density="compact"
        hide-details
        inputmode="none"
        :placeholder="codeOnly ? 'Enter a product code...' : 'Search products...'"
        variant="outlined"
        autofocus
        @update:model-value="emit('update:modelValue', $event)"
        @keydown.esc="emit('close')"
      />
      <k-btn
        class="product-layout-toggle"
        variant="tonal"
        :aria-label="gridMode ? 'Show results as a list' : 'Show results in columns'"
        :title="gridMode ? 'One per row' : 'Columns'"
        @click="emit('update:gridMode', !gridMode)"
      >
        <k-icon :icon="gridMode ? 'mdi-view-agenda-outline' : 'mdi-view-grid-outline'" size="24" />
      </k-btn>
    </div>
    <div class="product-inline-keyboard">
      <InlineKeyboard
        :model-value="modelValue"
        :input="searchInput"
        :max-length="80"
        layout="code"
        @update:model-value="emit('update:modelValue', $event)"
      />
    </div>
    <k-btn
      class="product-search-close"
      color="error"
      variant="tonal"
      aria-label="Close product search"
      @click="emit('close')"
    >
      <k-icon icon="mdi-close" size="28" class="me-1" />
      Close
    </k-btn>
  </div>
</template>

<script lang="ts" setup>
import { ref, watch } from "vue";
import InlineKeyboard from "@/components/InlineKeyboard.vue";
import { useTouchSelection } from "@/composables/useTouchSelection";
import { keepFieldFocus } from "@/lib/keepFocus";

// The products search's field, its on-screen keyboard and Close: the bottom
// of the search dialog, and of the sale screen while it searches in place.

defineProps<{ modelValue: string; gridMode: boolean; codeOnly?: boolean }>();
const emit = defineEmits<{
  "update:modelValue": [value: string];
  "update:gridMode": [value: boolean];
  close: [];
}>();

// The field's input, for the keyboard's caret and touch selection.
const searchField = ref<{ $el?: HTMLElement } | null>(null);
const searchInput = ref<HTMLInputElement | null>(null);
watch(
  searchField,
  (field) => {
    searchInput.value = field?.$el?.querySelector("input") ?? null;
  },
  { flush: "post" },
);
useTouchSelection(searchInput);

function focus() {
  searchInput.value?.focus({ preventScroll: true });
}

defineExpose({ focus });
</script>

<style scoped>
.product-search-keyboard {
  position: relative;
  display: flex;
  flex-direction: column;
}

.product-search-control {
  flex: 1 1 auto;
  min-width: 0;
}

.product-search-row {
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px 6px;
}

.product-layout-toggle {
  flex: 0 0 auto;
  height: 40px !important;
  min-width: 56px !important;
}

/* Must be tall enough for the full "code" layout keyboard (number row + 3
   letter rows + actions row = ~264px). If this is shorter the keyboard
   overflows upward and covers the search bar above it. */
.product-inline-keyboard {
  flex: 0 0 264px;
  height: 264px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  justify-content: flex-start;
  margin-bottom: 48px;
  margin-inline: auto;
  /* The keys flex to share whatever width this element ends up with, so they
     can't overflow; this cap just stops them looking oversized on a wide
     screen. overflow: hidden above only guards against vertical spill. */
  width: 100%;
  max-width: 748px;
  padding-inline: 8px;
  padding-top: 4px;
  padding-bottom: 0;
}

.product-search-close {
  position: absolute;
  right: 16px;
  bottom: 16px;
  height: 48px;
  z-index: 1;
}
</style>
