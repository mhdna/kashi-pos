<template>
  <v-dialog v-model="dialog" max-width="760">
    <v-card>
      <v-card-title>Edit Client</v-card-title>
      <v-card-text>
        <v-text-field
          :model-value="formatLebanesePhone(form.phone)"
          placeholder="Phone Number"
          placeholder="Phone Number"
          variant="outlined"
          density="compact"
          class="mb-2"
          inputmode="tel"
          @update:model-value="(v: string) => (form.phone = v.replace(/\D/g, ''))"
        />
        <v-text-field
          v-model="form.name"
          placeholder="Name"
          placeholder="Name"
          variant="outlined"
          density="compact"
          maxlength="80"
          @keydown="preventClientNameNumber"
          @beforeinput="preventClientNameNumberInput"
          @keydown.enter="save"
        />
        <InlineKeyboard v-model="form.name" class="client-edit-keyboard mt-3" />
        <div v-if="error" class="text-error text-caption mt-2">{{ error }}</div>
      </v-card-text>
      <v-card-actions class="pa-4">
        <v-btn variant="text" @click="dialog = false">Cancel</v-btn>
        <v-spacer />
        <v-btn
          variant="elevated"
          color="primary"
          :loading="saving"
          :disabled="!form.name.trim()"
          @click="save"
        >
          Save
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script lang="ts" setup>
import { reactive, ref, watch } from "vue";
import { UpdateClient } from "../../wailsjs/go/main/App";
import {
  preventClientNameNumber,
  preventClientNameNumberInput,
  toClient,
  type Client,
} from "@/lib/client";
import { formatLebanesePhone } from "@/lib/phone";
import InlineKeyboard from "@/components/InlineKeyboard.vue";

const props = defineProps<{ client: Client }>();
const emit = defineEmits<{ saved: [client: Client] }>();

const dialog = defineModel<boolean>({ default: false });
const form = reactive({ name: "", phone: "" });
const saving = ref(false);
const error = ref("");

watch(dialog, (open) => {
  if (!open) return;
  form.name = props.client.name;
  form.phone = props.client.phone.replace(/\D/g, "");
  error.value = "";
});

function capitalizeName(value: string) {
  return value
    .trim()
    .replace(
      /(^|[\s'-])([\p{L}])/gu,
      (_, prefix: string, letter: string) => `${prefix}${letter.toUpperCase()}`,
    );
}

async function save() {
  if (!form.name.trim()) return;
  saving.value = true;
  error.value = "";
  try {
    const updated = await UpdateClient(props.client.id, capitalizeName(form.name), form.phone);
    emit("saved", toClient(updated));
    dialog.value = false;
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err);
  } finally {
    saving.value = false;
  }
}
</script>

<style scoped>
.client-edit-keyboard {
  max-width: 700px;
}
</style>
