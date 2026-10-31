<template>
  <div v-if="sorted.length" class="targets-progress">
    <div class="targets-progress-title text-caption font-weight-bold text-medium-emphasis">
      <k-icon icon="mdi-flag-outline" size="14" class="me-1" />
      Targets
    </div>
    <div class="target-stack">
      <div v-for="target in sorted" :key="target.id" class="target-row">
        <div class="target-track">
          <div
            class="target-fill"
            :style="{ width: target.progressPct + '%', backgroundColor: target.color || undefined }"
          />
        </div>
        <div class="target-meta">
          <span>{{ formatDate(target.dateFrom) }} – {{ formatDate(target.dateTo) }}</span>
          <span class="font-weight-bold">
            {{ formatUsd(target.achievedCents / 100) }} / {{ formatUsd(target.targetAmountCents / 100) }}
            ({{ Math.round(target.progressPct) }}%)
          </span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { formatUsd } from "@/lib/format";

export interface BranchTarget {
  id: number;
  dateFrom: string;
  dateTo: string;
  targetAmountCents: number;
  achievedCents: number;
  color: string;
}

const props = defineProps<{ targets: BranchTarget[] }>();

// Listed low to high target amount, top to bottom.
const sorted = computed(() => {
  const list = [...props.targets].sort((a, b) => a.targetAmountCents - b.targetAmountCents);
  return list.map((target) => ({
    ...target,
    progressPct:
      target.targetAmountCents > 0
        ? Math.min(100, (target.achievedCents / target.targetAmountCents) * 100)
        : 0,
  }));
});

function formatDate(value: string) {
  return new Date(value).toLocaleDateString();
}
</script>

<style scoped>
.targets-progress-title {
  display: flex;
  align-items: center;
  margin-bottom: 6px;
}

.target-stack {
  display: flex;
  flex-direction: column;
  gap: 10px;
  width: 100%;
}

.target-row {
  display: flex;
  flex-direction: column;
  gap: 2px;
  width: 100%;
}

.target-track {
  width: 100%;
  height: 12px;
  border-radius: 6px;
  background: rgba(var(--k-theme-on-surface), 0.08);
  overflow: hidden;
}

.target-fill {
  height: 100%;
  border-radius: 6px;
  background-color: rgb(var(--k-theme-primary));
  background-image: repeating-linear-gradient(
    45deg,
    rgba(255, 255, 255, 0.18) 0,
    rgba(255, 255, 255, 0.18) 8px,
    transparent 8px,
    transparent 16px
  );
}

.target-meta {
  display: flex;
  justify-content: space-between;
  font-size: 0.75rem;
  color: rgba(var(--k-theme-on-surface), 0.7);
}
</style>
