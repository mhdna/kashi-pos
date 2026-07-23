<template>
  <v-dialog v-model="dialog" max-width="400">
    <v-card v-if="step === 'phone'">
      <v-card-title>Enter Phone Number</v-card-title>
      <v-card-text>
        <v-text-field
          v-model="phone"
          placeholder="Phone Number"
          variant="outlined"
          density="compact"
          autofocus
          @keydown.enter="lookupPhone"
        />
      </v-card-text>
      <v-card-actions class="flex-column ga-2 pa-4">
        <v-btn class="w-100" variant="elevated" color="primary" @click="lookupPhone">Continue</v-btn>
        <v-btn class="w-100" variant="tonal" @click="selectAnonymous">Prefer not to say</v-btn>
      </v-card-actions>
    </v-card>

    <v-card v-else-if="step === 'new'">
      <v-card-title>New Client</v-card-title>
      <v-card-text>
        <v-text-field v-model="newClient.phone" label="Phone Number" variant="outlined" density="compact" class="mb-2" readonly />
        <v-text-field v-model="newClient.name" label="Name" variant="outlined" density="compact" autofocus />
      </v-card-text>
      <v-card-actions class="pa-4">
        <v-btn variant="text" @click="step = 'phone'">Back</v-btn>
        <v-spacer />
        <v-btn variant="elevated" color="primary" :disabled="!newClient.name.trim()" @click="createClient">Save</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script lang="ts" setup>
import { ref, reactive, inject, watch } from "vue";
import { GetClientByPhone, CreateClient } from "../../wailsjs/go/main/App";

export interface Client {
  id: number;
  name: string;
  phone: string;
  initials: string;
  tier: string;
  points: number;
  totalLoyaltyPoints: number;
  validLoyaltyPoints: number;
}

const emit = defineEmits<{ select: [client: Client] }>();

const dialog = defineModel<boolean>({ default: false });
const dialogOpen = inject<import("vue").Ref<boolean>>("dialogOpen", { value: false } as any);
watch(dialog, (v) => { dialogOpen.value = v; });

const step = ref<"phone" | "new">("phone");
const phone = ref("");
const newClient = reactive({ name: "", phone: "" });

function initials(name: string) {
  return name.split(" ").map((w) => w[0]).join("").toUpperCase().slice(0, 2);
}

function tier(points: number) {
  if (points >= 500) return "Gold";
  if (points >= 200) return "Silver";
  return "Bronze";
}

function toClient(raw: any): Client {
  return {
    id: raw.id,
    name: raw.name,
    phone: raw.phone,
    initials: initials(raw.name),
    tier: tier(raw.totalLoyaltyPoints),
    points: raw.totalLoyaltyPoints,
    totalLoyaltyPoints: raw.totalLoyaltyPoints,
    validLoyaltyPoints: raw.validLoyaltyPoints,
  };
}

async function lookupPhone() {
  const p = phone.value.replace(/\D/g, "");
  if (!p) return;
  try {
    const c = await GetClientByPhone(p);
    emit("select", toClient(c));
    dialog.value = false;
    reset();
  } catch {
    newClient.phone = phone.value;
    newClient.name = "";
    step.value = "new";
  }
}

async function createClient() {
  try {
    const c = await CreateClient(newClient.name.trim(), newClient.phone);
    emit("select", toClient(c));
  } catch {
    emit("select", {
      id: 0, name: newClient.name.trim(), phone: newClient.phone,
      initials: initials(newClient.name.trim()), tier: "Bronze",
      points: 0, totalLoyaltyPoints: 0, validLoyaltyPoints: 0,
    });
  }
  dialog.value = false;
  reset();
}

function selectAnonymous() {
  emit("select", {
    id: 0, name: "Anonymous", phone: "",
    initials: "AN", tier: "None",
    points: 0, totalLoyaltyPoints: 0, validLoyaltyPoints: 0,
  });
  dialog.value = false;
  reset();
}

function reset() {
  step.value = "phone";
  phone.value = "";
  newClient.name = "";
  newClient.phone = "";
}
</script>
