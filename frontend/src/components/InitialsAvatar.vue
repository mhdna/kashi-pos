<template>
  <v-avatar :color="color" :size="avatarSize">
    <span :style="{ letterSpacing: '1px', fontSize: fontSize }">{{ initials }}</span>
  </v-avatar>
</template>

<script setup lang="ts">
import { computed } from "vue";

const props = withDefaults(defineProps<{
  name: string;
  color?: string;
  size?: number;
}>(), {
  color: "grey-darken-1",
});

const avatarSize = computed(() => props.size ?? 36);

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
