<template>
  <v-menu v-model="menu" :close-on-content-click="false" location="bottom end">
    <template #activator="{ props }">
      <v-btn v-bind="props" class="py-6 px-0 me-4">
        <v-list-item
          subtitle="Salesperson"
          :title="selectedName"
          style="min-width: 180px"
        >
          <template #prepend>
            <v-avatar :color="selectedColor" size="42">
              <span class="text-headline-small">{{ selectedInitials }}</span>
            </v-avatar>
          </template>
        </v-list-item>
      </v-btn>
    </template>
    <v-list density="compact" nav style="max-height: 400px; overflow-y: auto">
      <v-list-item
        v-for="p in salespersons"
        :key="p.id"
        @click="selectSalesperson(p)"
      >
        <template #prepend>
          <v-avatar :color="colorFor(p.id)" size="32">
            <span class="text-body-small font-weight-bold">{{ initials(p.fullName) }}</span>
          </v-avatar>
        </template>
        <v-list-item-title class="text-body-2">{{ p.fullName }}</v-list-item-title>
      </v-list-item>
    </v-list>
  </v-menu>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { ListSalespersons } from "../../wailsjs/go/main/App";

const menu = ref(false);
const colors = ["yellow", "blue", "red", "purple", "pink", "green", "orange", "teal"];

interface Salesperson { id: number; fullName: string; }

const salespersons = ref<Salesperson[]>([]);
const selected = ref<Salesperson>({ id: 0, fullName: "Anonymous" });

const selectedName = computed(() => selected.value.fullName);
const selectedColor = computed(() => colorFor(selected.value.id));
const selectedInitials = computed(() => initials(selected.value.fullName));

function colorFor(id: number) { return colors[id % colors.length]; }
function initials(name: string) { return name.split(" ").map((w) => w[0]).join(""); }

function selectSalesperson(p: Salesperson) {
  selected.value = p;
  menu.value = false;
}

onMounted(async () => {
  const list = await ListSalespersons();
  if (list.length) {
    salespersons.value = list as Salesperson[];
    selected.value = list[0] as Salesperson;
  }
});
</script>
