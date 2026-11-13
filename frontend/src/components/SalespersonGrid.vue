<template>
  <!-- Big tap targets: one tile per salesperson, an avatar over a name. The
       same picker as the invoice's header Salesperson button. -->
  <div
    class="sp-grid"
    :class="{ 'sp-grid--dense': dense }"
    :style="{ '--sp-columns': columns ?? 3 }"
    role="listbox"
    aria-label="Salesperson"
  >
    <button
      v-if="defaultLabel"
      type="button"
      role="option"
      class="sp-tile"
      :class="{ 'sp-tile--active': modelValue === 0 }"
      :aria-selected="modelValue === 0"
      data-test="sp-default"
      @click="emit('update:modelValue', 0)"
    >
      <span class="sp-default-avatar"><k-icon icon="mdi-account-tie" :size="dense ? 22 : 26" /></span>
      <span class="sp-name">{{ defaultLabel }}</span>
    </button>
    <button
      v-for="person in salespersons"
      :key="person.id"
      type="button"
      role="option"
      class="sp-tile"
      :class="{ 'sp-tile--active': modelValue === person.id }"
      :aria-selected="modelValue === person.id"
      :data-test="`sp-${person.id}`"
      @click="emit('update:modelValue', person.id)"
    >
      <InitialsAvatar :name="person.fullName ?? ''" :color="salespersonColor(person)" :size="dense ? 40 : 48" class="flex-shrink-0" />
      <span class="sp-name">{{ person.fullName }}</span>
    </button>
  </div>
</template>

<script setup lang="ts">
import InitialsAvatar from "@/components/InitialsAvatar.vue";
import { salespersonColor } from "@/lib/salespersonColor";

defineProps<{
  salespersons: { id: number; fullName: string; color?: string | null }[];
  // The picked salesperson's id; 0 is the "default" tile, null/-1 is none.
  modelValue: number | null;
  // Adds a first tile that stands for "nobody in particular" (id 0), e.g.
  // "Invoice salesperson". Left out when every pick must be a real person.
  defaultLabel?: string;
  // How many tiles per row (3 when left out).
  columns?: number;
  // Smaller tiles, for a grid laid inside a dialog rather than a menu.
  dense?: boolean;
}>();
const emit = defineEmits<{ "update:modelValue": [id: number] }>();
</script>

<style scoped>
.sp-grid {
  display: grid;
  grid-template-columns: repeat(var(--sp-columns, 3), minmax(0, 1fr));
  gap: 8px;
}
.sp-tile {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 104px;
  padding: 12px 8px;
  background: transparent;
  border: 1px solid rgba(var(--k-border-color), 0.18);
  border-radius: 8px;
  color: inherit;
  cursor: pointer;
  font: inherit;
  transition: background-color 120ms ease, border-color 120ms ease, transform 120ms ease;
}
.sp-tile:hover {
  background-color: rgba(var(--k-theme-on-surface), 0.08);
  border-color: rgba(var(--k-border-color), 0.36);
}
.sp-tile:active {
  transform: scale(0.97);
}
/* The one picked now. */
.sp-tile--active {
  background-color: rgba(var(--k-theme-primary), 0.1);
  border-color: rgb(var(--k-theme-primary));
  box-shadow: inset 0 0 0 1px rgb(var(--k-theme-primary));
}
.sp-name {
  width: 100%;
  overflow: hidden;
  text-align: center;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.8125rem;
}
.sp-grid--dense .sp-tile {
  gap: 6px;
  min-height: 88px;
  padding: 10px 6px;
}
.sp-grid--dense .sp-default-avatar {
  width: 40px;
  height: 40px;
}
.sp-default-avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  border-radius: 50%;
  background: rgba(var(--k-theme-on-surface), 0.1);
  color: rgba(var(--k-theme-on-surface), 0.65);
}
</style>
