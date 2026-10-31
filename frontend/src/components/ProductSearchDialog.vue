<template>
  <k-dialog
    v-model="dialog"
    class="product-search-dialog"
    max-width="none"
    width="auto"
  >
    <template v-if="showActivator" #activator="{ props: activatorProps }">
      <k-btn
        v-bind="activatorProps"
        :icon="!activatorText"
        :aria-label="activatorText || 'Add products'"
        variant="text"
        class="ms-1"
      >
        <k-icon :icon="activatorIcon" size="20" :class="{ 'me-1': Boolean(activatorText) }" />
        <template v-if="activatorText">{{ activatorText }}</template>
      </k-btn>
    </template>
    <k-card class="product-list-card" @mousedown="keepFieldFocus">
      <ProductSearchResults
        class="product-search-results"
        :search="search"
        :active="dialog"
        :grid-mode="gridMode"
        :code-only="codeOnly"
        :hidden-fields="hiddenFields"
        @select="selectProduct"
      />
      <k-divider />
      <ProductSearchKeyboard
        v-model="search"
        v-model:grid-mode="gridMode"
        :code-only="codeOnly"
        @close="dialog = false"
      />
    </k-card>
  </k-dialog>
</template>

<script lang="ts" setup>
import { computed, inject, ref, watch } from "vue";
import ProductSearchKeyboard from "@/components/ProductSearchKeyboard.vue";
import ProductSearchResults, { type ProductItem } from "@/components/ProductSearchResults.vue";
import { useProductGridMode } from "@/composables/useProductGridMode";
import { keepFieldFocus } from "@/lib/keepFocus";

const emit = defineEmits<{ select: [product: ProductItem] }>();

const props = withDefaults(
  defineProps<{
    activatorText?: string;
    activatorIcon?: string;
    showActivator?: boolean;
    // Hides product names; the backend then matches only code/barcode.
    codeOnly?: boolean;
    // Columns the branch leaves out of the list (kashi's branch setting).
    hiddenFields?: string[];
  }>(),
  {
    activatorText: "Products List",
    activatorIcon: "mdi-format-list-bulleted-square",
    showActivator: true,
    codeOnly: false,
    hiddenFields: () => [],
  },
);
const activatorText = props.activatorText;
const activatorIcon = props.activatorIcon;
const showActivator = props.showActivator;

const gridMode = useProductGridMode(computed(() => props.codeOnly || props.hiddenFields.includes("name")));

const dialogOpen = inject<import("vue").Ref<boolean>>("dialogOpen", { value: false } as any);
const dialog = ref(false);
const search = ref("");

watch(dialog, (v) => {
  dialogOpen.value = v;
  // Don't let the previous query (and its results) linger for the next open.
  if (!v) search.value = "";
});

function selectProduct(product: ProductItem) {
  emit("select", product);
  dialog.value = false;
}

function openDialog() {
  dialog.value = true;
}

defineExpose({ openDialog });
</script>

<style scoped>
.product-list-card {
  height: 100%;
  position: relative;
  display: flex;
  flex-direction: column;
}

/* Fills the space left above the search bar + keyboard so the search bar
   stays anchored and never shifts as the result count changes. */
.product-search-results {
  flex: 1 1 auto;
  min-height: 0;
}

:global(.product-search-dialog .k-overlay__content) {
  position: fixed;
  top: 0;
  right: 0;
  bottom: 0;
  left: 72px;
  width: auto !important;
  height: 100vh !important;
  min-height: 100vh !important;
  max-width: none !important;
  max-height: none !important;
  margin: 0;
}

:global(.kashi-compact .product-search-dialog .k-overlay__content) {
  left: 80px;
}
</style>
