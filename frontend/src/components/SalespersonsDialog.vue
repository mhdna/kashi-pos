<template>
  <v-menu v-model="menu" :close-on-content-click="false" location="bottom center">
    <template #activator="{ props }">
      <div class="header-btn-wrap">
        <div class="header-btn-subtitle">Salesperson</div>
        <v-btn
          v-bind="props"
          size="small"
          color="blue-grey-darken-1"
          variant="tonal"
          class="header-fixed-width-btn"
          :disabled="disabled"
          :class="{ 'missing-selection-pulse': pulseMissingSalesperson }"
        >
          <v-icon icon="mdi-account-tie" size="16" class="me-1 flex-shrink-0" />
          <span class="header-fixed-width-btn-label">{{ selectedName }}</span>
        </v-btn>
      </div>
    </template>
    <v-card class="salespersons-grid">
      <v-row class="pa-2 ga-2 fill-height" no-gutters>
        <v-col v-for="p in salespersons" :key="p.id" cols="4" style="display: flex">
          <button
            type="button"
            class="salesperson-card d-flex flex-column align-center justify-center w-100"
            @click="selectSalesperson(p)"
          >
            <InitialsAvatar
              :name="p.fullName ?? ''"
              :color="salespersonColor(p)"
              :size="48"
              class="mb-2 flex-shrink-0"
            />
            <span class="text-caption text-center text-truncate w-100">{{ p.fullName }}</span>
          </button>
        </v-col>
      </v-row>
    </v-card>
  </v-menu>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { ListSalespersons } from "../../wailsjs/go/main/App";
import InitialsAvatar from "./InitialsAvatar.vue";
import { salespersonColor } from "@/lib/salespersonColor";
import { useSalesperson, type Salesperson } from "@/composables/useSalesperson";

defineProps<{ disabled?: boolean }>();
const menu = ref(false);

const salespersons = ref<Salesperson[]>([]);
const { selectedSalesperson: selected, pulseMissingSalesperson } = useSalesperson();

const selectedName = computed(() => selected.value?.fullName.split(" ")[0] ?? "Salesperson");


function selectSalesperson(p: Salesperson) {
  selected.value = p;
  menu.value = false;
}

onMounted(async () => {
  // Staff who only punch attendance (isSalesperson off in kashi) don't sell.
  const list = ((await ListSalespersons()) as any[]).filter((s) => s.isSalesperson !== false);
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
  color: rgba(var(--v-theme-on-surface), 0.6);
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
    box-shadow: 0 0 0 0 rgba(var(--v-theme-warning), 0);
  }
  50% {
    box-shadow: 0 0 0 8px rgba(var(--v-theme-warning), 0.32);
  }
}

.salespersons-grid {
  width: min(420px, calc(100vw - 32px));
  max-height: 400px;
  overflow-y: auto;
}

.salesperson-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 112px;
  padding: 12px;
  background: transparent;
  border: 1px solid rgba(var(--v-border-color), 0.18);
  border-radius: 8px;
  cursor: pointer;
  transition: all 120ms ease;
  font-family: inherit;
  font-size: inherit;
  width: 100%;
}

.salesperson-card:hover {
  background-color: rgba(var(--v-theme-on-surface), 0.08);
  border-color: rgba(var(--v-border-color), 0.36);
}

.salesperson-card:active {
  transform: scale(0.97);
}
</style>
