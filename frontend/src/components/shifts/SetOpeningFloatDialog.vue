<template>
  <k-dialog :model-value="modelValue" max-width="380" @update:model-value="$emit('update:modelValue', $event)">
    <k-card>
      <k-card-title>Set opening cash</k-card-title>
      <k-card-text class="dialog-content">
        <div class="text-caption text-medium-emphasis mb-1">Opening amount (USD)</div>
        <k-text-field v-model.number="usd" placeholder="0.00" type="number" min="0" step="0.01" variant="outlined" density="compact" class="mb-2" />
        <div class="text-caption text-medium-emphasis mb-1">Opening amount (LBP)</div>
        <k-text-field v-model.number="lbp" placeholder="0" type="number" min="0" step="1" variant="outlined" density="compact" class="mb-2" />
        <div class="text-caption text-medium-emphasis mb-1">Manager password</div>
        <k-text-field
          v-model="password"
          placeholder="••••••••"
          :type="showPassword ? 'text' : 'password'"
          variant="outlined"
          density="compact"
          :append-inner-icon="showPassword ? 'mdi-eye-off' : 'mdi-eye'"
          @click:append-inner="showPassword = !showPassword"
          @keyup.enter="save"
        />
        <div class="alert-container">
          <k-alert v-if="negative" type="warning" variant="tonal" density="compact" class="mb-2">
            Opening amount cannot be negative — enter the actual cash amount in the drawer.
          </k-alert>
          <k-alert v-if="error" type="error" variant="tonal" density="compact">{{ error }}</k-alert>
        </div>
      </k-card-text>
      <k-card-actions>
        <k-spacer />
        <k-btn variant="text" @click="$emit('update:modelValue', false)">Cancel</k-btn>
        <k-btn color="primary" :loading="saving" :disabled="negative" @click="save">Apply</k-btn>
      </k-card-actions>
    </k-card>
  </k-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { ApplyOpeningFloatToCurrentShift } from "../../../wailsjs/go/main/App";

const props = defineProps<{
  modelValue: boolean;
  shiftId?: number;
  openingFloatUsdCents?: number;
  openingFloatLbp?: number;
}>();
const emit = defineEmits<{ "update:modelValue": [value: boolean]; saved: [] }>();

const usd = ref(0);
const lbp = ref(0);
const password = ref("");
const error = ref("");
const saving = ref(false);
const showPassword = ref(false);
const negative = computed(() => usd.value < 0 || lbp.value < 0);

watch(
  () => props.modelValue,
  (open) => {
    if (!open) return;
    usd.value = Number(props.openingFloatUsdCents ?? 0) / 100;
    lbp.value = Number(props.openingFloatLbp ?? 0);
    password.value = "";
    error.value = "";
  },
);

async function save() {
  if (!props.shiftId || negative.value) return;
  saving.value = true;
  error.value = "";
  try {
    const usdCents = Number.isFinite(usd.value) && usd.value > 0 ? Math.round(usd.value * 100) : 0;
    const lbpUnits = Number.isFinite(lbp.value) && lbp.value > 0 ? Math.round(lbp.value) : 0;
    await ApplyOpeningFloatToCurrentShift(usdCents, lbpUnits, password.value);
    emit("update:modelValue", false);
    emit("saved");
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err);
  } finally {
    saving.value = false;
  }
}
</script>

<style scoped>
.dialog-content {
  min-height: 280px;
  display: flex;
  flex-direction: column;
}

.alert-container {
  min-height: 60px;
  margin-top: auto;
  padding-top: 8px;
}
</style>
