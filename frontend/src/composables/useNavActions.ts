import { ref } from "vue";

export const pendingNavAction = ref<"onhold" | "exchange" | null>(null);
