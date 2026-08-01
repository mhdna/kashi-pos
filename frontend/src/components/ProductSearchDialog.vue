<template>
  <v-dialog v-model="dialog" max-width="760" width="calc(100% - 32px)" height="670">
    <template v-if="showActivator" #activator="{ props: activatorProps }">
      <v-btn
        v-bind="activatorProps"
        :icon="!activatorText"
        :aria-label="activatorText || 'Add products'"
        variant="text"
        class="ms-1"
      >
        <v-icon :icon="activatorIcon" size="20" :class="{ 'me-1': Boolean(activatorText) }" />
        <template v-if="activatorText">{{ activatorText }}</template>
      </v-btn>
    </template>
    <v-card class="product-list-card">
      <v-card-title class="d-flex align-center">
        Products
        <v-spacer />
        <v-text-field
          v-model="search"
          class="product-search-control ms-4"
          density="compact"
          hide-details
          placeholder="Search products..."
          variant="outlined"
          autofocus
        />
      </v-card-title>
      <v-infinite-scroll
        :key="listKey"
        class="product-list-scroll"
        :height="400"
        @load="loadItems"
      >
        <v-list v-if="serverItems.length" density="compact" class="pa-0">
          <v-list-item
            v-for="(item, index) in serverItems"
            :key="item.id || `${item.barcode}-${index}`"
            :class="index % 2 === 0 ? 'bg-grey-lighten-5' : undefined"
            rounded="0"
            @click="selectProduct(item)"
          >
            <template #title>
              <span class="font-weight-medium">{{ item.name }}</span>
              <v-chip v-if="item.color" size="x-small" variant="tonal" class="ms-2">{{ item.color }}</v-chip>
              <v-chip v-if="item.size" size="x-small" variant="outlined" class="ms-1">{{ item.size }}</v-chip>
            </template>
            <template #subtitle>{{ item.code }} · {{ item.barcode }}</template>
            <template #append>${{ Number(item.price || 0).toFixed(2) }}</template>
          </v-list-item>
        </v-list>
        <div v-else-if="!loading" class="pa-6 text-center text-medium-emphasis">No products found.</div>
      </v-infinite-scroll>
      <v-divider />
      <v-card-text class="product-inline-keyboard">
        <InlineKeyboard v-model="search" :max-length="80" layout="code" />
      </v-card-text>
    </v-card>
  </v-dialog>
</template>

<script lang="ts" setup>
import { ref, inject, onBeforeUnmount, watch } from "vue";
import { ListProductsPage } from "../../wailsjs/go/main/App";
import InlineKeyboard from "@/components/InlineKeyboard.vue";

const emit = defineEmits<{
  select: [
    product: {
      name: string;
      code: string;
      category: string;
      price: number;
      discount: number;
      qty: number;
    },
  ];
}>();

const props = withDefaults(
  defineProps<{
    activatorText?: string;
    activatorIcon?: string;
    showActivator?: boolean;
  }>(),
  {
    activatorText: "Products List",
    activatorIcon: "mdi-format-list-bulleted-square",
    showActivator: true,
  },
);
const activatorText = props.activatorText;
const activatorIcon = props.activatorIcon;
const showActivator = props.showActivator;

const dialogOpen = inject<import("vue").Ref<boolean>>("dialogOpen", { value: false } as any);
const dialog = ref(false);
const search = ref("");
const debouncedSearch = ref("");
let searchTimer: ReturnType<typeof setTimeout> | undefined;
let latestRequest = 0;

watch(dialog, (v) => {
  dialogOpen.value = v;
});

watch(search, (value) => {
  clearTimeout(searchTimer);
  latestRequest++;
  searchTimer = setTimeout(() => {
    debouncedSearch.value = value.trim();
  }, 250);
});

const pageSize = 30;
const serverItems = ref<any[]>([]);
const loading = ref(false);
const nextPage = ref(1);
const listKey = ref(0);
const hasMore = ref(true);

async function loadItems({ done }: { done: (status: "ok" | "empty" | "error") => void }) {
  if (loading.value || !hasMore.value) {
    done(hasMore.value ? "ok" : "empty");
    return;
  }
  const request = latestRequest;
  loading.value = true;
  try {
    const result = await ListProductsPage(nextPage.value, pageSize, debouncedSearch.value);
    if (request !== latestRequest) {
      done("ok");
      return;
    }
    const pageItems = result.items.map((p) => ({ ...p, qty: 1 }));
    serverItems.value.push(...pageItems);
    nextPage.value += 1;
    hasMore.value = serverItems.value.length < result.total && pageItems.length > 0;
    done(hasMore.value ? "ok" : "empty");
  } catch (e) {
    console.error("ListProductsPage failed:", e);
    done("error");
  } finally {
    loading.value = false;
  }
}

watch(debouncedSearch, () => {
  latestRequest++;
  serverItems.value = [];
  nextPage.value = 1;
  hasMore.value = true;
  listKey.value += 1;
});

function selectProduct(item: any) {
  emit("select", item);
  dialog.value = false;
  search.value = "";
}

onBeforeUnmount(() => {
  clearTimeout(searchTimer);
  latestRequest++;
});

function openDialog() {
  latestRequest++;
  serverItems.value = [];
  nextPage.value = 1;
  hasMore.value = true;
  listKey.value += 1;
  dialog.value = true;
}

defineExpose({ openDialog });
</script>

<style scoped>
.product-search-control {
  max-width: 320px;
  width: 100%;
}

.product-list-card {
  height: 670px;
  display: flex;
  flex-direction: column;
}

.product-list-scroll {
  flex: 0 0 400px;
  height: 400px;
  min-height: 0;
}

.product-inline-keyboard {
  flex: 0 0 205px;
  height: 205px;
  display: flex;
  align-items: center;
  margin-inline: auto;
  max-width: 600px;
  padding-block: 12px;
}
</style>
