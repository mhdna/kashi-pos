<template>
  <v-avatar :color="color" :size="avatarSize">
    <span :style="{ letterSpacing: '1px', fontSize: fontSize }">{{ initials }}</span>
  </v-avatar>
</template>

<script setup lang="ts">
import { computed } from "vue";

const props = defineProps<{
  name: string;
  color: string;
  size?: number;
}>();

const avatarSize = computed(() => props.size ?? 42);

const initials = computed(() =>
  props.name
    .split(" ")
    .map((w) => w[0] || "")
    .join("")
    .slice(0, 2)
    .toUpperCase(),
);

const fontSize = computed(() => `${Math.round(avatarSize.value * 0.4)}px`);
</script>
