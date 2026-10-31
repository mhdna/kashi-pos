<template>
  <k-dialog v-model="dialog" max-width="760">
    <k-card>
      <k-card-title>Edit Client</k-card-title>
      <k-card-text>
        <PhoneField ref="phoneField" v-model="form.phone" class="mb-2" @focus="phoneActive = true" />
        <k-text-field
          v-model="form.name"
          placeholder="Name"
          variant="outlined"
          density="compact"
          maxlength="80"
          @keydown="preventClientNameNumber"
          @beforeinput="preventClientNameNumberInput"
          @keydown.enter="save"
          @focus="phoneActive = false"
        />
        <!-- The number pad while the phone is being edited, letters otherwise. -->
        <PhonePad v-if="phoneActive" class="mt-3" @press="phoneField?.press($event)" />
        <InlineKeyboard v-else v-model="form.name" class="client-edit-keyboard mt-3" />
        <div v-if="error" class="text-error text-caption mt-2">{{ error }}</div>
      </k-card-text>
      <k-card-actions class="pa-4">
        <k-btn variant="text" @click="dialog = false">Cancel</k-btn>
        <k-spacer />
        <k-btn
          variant="elevated"
          color="primary"
          :loading="saving"
          :disabled="!form.name.trim()"
          @click="save"
        >
          Save
        </k-btn>
      </k-card-actions>
    </k-card>
  </k-dialog>
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
import InlineKeyboard from "@/components/InlineKeyboard.vue";
import PhoneField from "@/components/PhoneField.vue";
import PhonePad from "@/components/PhonePad.vue";

const props = defineProps<{ client: Client }>();
const emit = defineEmits<{ saved: [client: Client] }>();

const dialog = defineModel<boolean>({ default: false });
const form = reactive({ name: "", phone: "" });
const saving = ref(false);
const error = ref("");
const phoneField = ref<InstanceType<typeof PhoneField> | null>(null);
const phoneActive = ref(false);

watch(dialog, (open) => {
  if (!open) return;
  form.name = props.client.name;
  // A Lebanese number is edited as the local number ("78928747"); the
  // server turns that back into +961. Anything else keeps its "+".
  const phone = props.client.phone.trim();
  form.phone = phone.startsWith("+961") ? phone.slice(4) : phone;
  error.value = "";
  phoneActive.value = false;
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
