<template>
  <v-menu v-model="menu" :close-on-content-click="false" location="bottom end">
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
    <v-list density="compact" nav style="max-height: 400px; overflow-y: auto">
      <v-list-item v-for="p in salespersons" :key="p.id" @click="selectSalesperson(p)">
        <template #prepend>
          <InitialsAvatar :name="p.fullName ?? ''" :color="colorFor(p.id) ?? 'grey'" :size="32" />
        </template>
        <v-list-item-title class="text-body-2">{{ p.fullName }}</v-list-item-title>
      </v-list-item>
    </v-list>
  </v-menu>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { ListSalespersons } from "../../wailsjs/go/main/App";
import InitialsAvatar from "./InitialsAvatar.vue";
import { useSalesperson, type Salesperson } from "@/composables/useSalesperson";

defineProps<{ disabled?: boolean }>();
const menu = ref(false);
const salespersonAvatarColor = "grey-darken-1";

const salespersons = ref<Salesperson[]>([]);
const { selectedSalesperson: selected, pulseMissingSalesperson } = useSalesperson();

const selectedName = computed(() => selected.value?.fullName.split(" ")[0] ?? "Salesperson");

function colorFor(id: number) {
  return salespersonAvatarColor;
}

function selectSalesperson(p: Salesperson) {
  selected.value = p;
  menu.value = false;
}

onMounted(async () => {
  const list = await ListSalespersons();
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
</style>
