<template>
  <v-menu v-model="menu" :close-on-content-click="false" location="bottom end">
    <template #activator="{ props }">
      <v-btn
        v-bind="props"
        class="salesperson-card py-0 px-2 mx-0 my-2 text-body-2"
        color="primary"
        variant="tonal"
        elevation="0"
        min-height="40px"
        rounded="lg"
        width="100%"
        :disabled="disabled"
        :class="{ 'missing-selection-pulse': pulseMissingSalesperson }"
      >
        <v-avatar class="salesperson-avatar" :color="selectedColor" size="24">
          <v-icon icon="mdi-account-tie" size="16" />
        </v-avatar>
        <span class="salesperson-name">{{ selectedName }}</span>
      </v-btn>
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

const selectedName = computed(() => selected.value?.fullName ?? "Salesperson");
const selectedColor = computed(() => salespersonAvatarColor);

function colorFor(id: number) {
  return salespersonAvatarColor;
}
function initials(name: string) {
  return name
    .split(" ")
    .map((w) => w[0])
    .join("");
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
.missing-selection-pulse {
  animation: missing-selection-pulse 700ms ease-in-out 2;
  border-radius: 12px;
}

.salesperson-card {
  position: relative;
  text-transform: none;
  justify-content: flex-start !important;
}

.salesperson-avatar {
  position: absolute;
  inset-inline-start: 8px;
  top: 50%;
  transform: translateY(-50%);
  z-index: 1;
}

.salesperson-name {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  text-align: center;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  padding-inline: 36px 6px;
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
