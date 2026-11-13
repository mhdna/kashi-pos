<template>
  <k-menu v-model="menu" :close-on-content-click="false" location="bottom center">
    <template #activator="{ props }">
      <div class="header-btn-wrap">
        <div class="header-btn-subtitle">Salesperson</div>
        <k-btn
          v-bind="props"
          size="small"
          color="blue-grey-darken-1"
          variant="tonal"
          class="header-fixed-width-btn"
          :disabled="disabled"
          :class="{ 'missing-selection-pulse': pulseMissingSalesperson }"
        >
          <k-icon icon="mdi-account-tie" size="16" class="me-1 flex-shrink-0" />
          <span class="header-fixed-width-btn-label">{{ selectedName }}</span>
        </k-btn>
      </div>
    </template>
    <k-card class="salespersons-grid">
      <div class="pa-2">
        <SalespersonGrid :salespersons="salespersons" :model-value="selected?.id ?? null" @update:model-value="pickById" />
      </div>
    </k-card>
  </k-menu>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { ListSalespersons } from "../../wailsjs/go/main/App";
import SalespersonGrid from "./SalespersonGrid.vue";
import { useSalesperson, type Salesperson } from "@/composables/useSalesperson";

defineProps<{ disabled?: boolean }>();
const menu = ref(false);

const salespersons = ref<Salesperson[]>([]);
const { selectedSalesperson: selected, pulseMissingSalesperson } = useSalesperson();

const selectedName = computed(() => selected.value?.fullName.split(" ")[0] ?? "Salesperson");


function pickById(id: number) {
  const p = salespersons.value.find((s) => s.id === id);
  if (p) selectSalesperson(p);
}

function selectSalesperson(p: Salesperson) {
  selected.value = p;
  menu.value = false;
}

onMounted(async () => {
  // Staff who only punch attendance (isSalesperson off in central) don't sell,
  // and the till's own leftovers aren't offered once central manages the staff.
  const list = ((await ListSalespersons()) as any[]).filter((s) => s.isSalesperson !== false && s.isCurrent !== false);
  if (list.length) {
    salespersons.value = [...(list as Salesperson[])].sort((a, b) =>
      a.fullName.localeCompare(b.fullName, undefined, { sensitivity: "base" }),
    );
  }
});
</script>

<style scoped>
.header-btn-wrap {
  display: flex;
  flex-direction: column;
  align-items: center;
}
.header-btn-subtitle {
  font-size: 0.8rem;
  line-height: 1.2;
  color: rgba(var(--k-theme-on-surface), 0.6);
  margin-bottom: 2px;
}
.header-fixed-width-btn {
  width: 108px;
  height: 35px;
}
.header-fixed-width-btn-label {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.missing-selection-pulse {
  animation: missing-selection-pulse 700ms ease-in-out 2;
  border-radius: 12px;
}

@keyframes missing-selection-pulse {
  0%,
  100% {
    box-shadow: 0 0 0 0 rgba(var(--k-theme-warning), 0);
  }
  50% {
    box-shadow: 0 0 0 8px rgba(var(--k-theme-warning), 0.32);
  }
}

.salespersons-grid {
  width: min(420px, calc(100vw - 32px));
  max-height: 400px;
  overflow-y: auto;
}
</style>
