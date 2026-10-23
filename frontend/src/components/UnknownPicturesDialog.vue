<template>
  <v-dialog :model-value="modelValue" max-width="720" persistent @update:model-value="emit('update:modelValue', $event)">
    <v-card class="pa-4">
      <v-card-title class="px-0 pt-0 text-h5">Pictures of the unknown items</v-card-title>

      <div class="d-flex ga-6 align-start">
        <div style="flex: none">
          <UnknownPhonePairing />
          <div class="text-h6 mt-2 text-center">Photograph each item.</div>
          <div class="text-h6 font-weight-regular text-medium-emphasis text-center" dir="rtl" lang="ar">
            التقط صورة لكل قطعة غير معروف كودها طبقا للسعر
          </div>
        </div>

        <div class="flex-grow-1" style="min-width: 0">
          <div v-if="error" class="text-body-2 py-3">
            {{ error }}
            <div class="mt-2"><v-btn color="primary" variant="tonal" @click="emit('retry')">Try again</v-btn></div>
          </div>
          <v-list class="text-body-1">
            <v-list-item v-for="(slot, index) in slots" :key="slot.slotRef" :title="`#${index + 1} · ${slot.label}`">
              <template #append>
                <v-icon v-if="slot.hasImage" color="success" icon="mdi-check-circle" />
                <span v-else-if="slot.waived" class="text-caption text-success">allowed by a manager</span>
                <v-icon v-else color="grey" icon="mdi-timer-sand" />
              </template>
            </v-list-item>
          </v-list>
          <div class="text-body-1 text-medium-emphasis">{{ doneCount }} of {{ slots.length }} ready</div>
        </div>
      </div>

      <v-card-actions class="px-0 pb-0 mt-3">
        <v-btn :disabled="helpAsked || !clientRef" size="large" prepend-icon="mdi-bell-ring-outline" variant="tonal" @click="askForHelp">
          {{ helpAsked ? "Manager asked" : "Ask a manager" }}
        </v-btn>
        <v-spacer />
        <v-btn size="large" variant="text" @click="cancel">Cancel</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { RequestUnknownItemHelp } from "../../wailsjs/go/main/App";
import UnknownPhonePairing from "@/components/UnknownPhonePairing.vue";

// Held between "pay" and the payment itself while a unit of the cart still
// has no picture. The paired phone has usually taken them all by then; this
// shows which are missing (the sale screen keeps them live) and the phone's
// QR code, for a phone that isn't connected. The sale screen closes it the
// moment every unit has a picture -- or a kashi manager lets it through.

type Slot = { slotRef: string; label: string; hasImage: boolean; waived: boolean; done: boolean };

const props = defineProps<{ modelValue: boolean; slots: Slot[]; clientRef: string; error: string }>();
const emit = defineEmits<{
  "update:modelValue": [open: boolean];
  cancel: [];
  retry: [];
}>();

const helpAsked = ref(false);
const doneCount = computed(() => props.slots.filter((slot) => slot.done).length);

async function askForHelp() {
  helpAsked.value = true;
  try {
    await RequestUnknownItemHelp(props.clientRef);
  } catch {
    helpAsked.value = false;
  }
}

function cancel() {
  emit("cancel");
  emit("update:modelValue", false);
}
</script>
