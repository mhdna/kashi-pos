<template>
  <v-dialog v-model="dialog" max-width="400">
    <v-card v-if="step === 'phone'">
      <v-card-title>Enter Phone Number</v-card-title>
      <v-card-text>
        <v-autocomplete
          v-model="selectedCountry"
          :items="countries"
          item-title="displayName"
          item-value="code"
          placeholder="Country"
          variant="outlined"
          density="compact"
          class="mb-2"
          hide-details
          autocomplete="off"
        >
          <template #selection="{ item }">
            <span class="country-flag">{{ item.flag }}</span>
            <span>{{ item.displayName }}</span>
          </template>
          <template #item="{ props: itemProps, item }">
            <v-list-item v-bind="itemProps" :subtitle="`+${item.dialCode}`">
              <template #title>
                <span class="country-flag">{{ item.flag }}</span>
                <span>{{ item.name }}</span>
              </template>
            </v-list-item>
          </template>
        </v-autocomplete>
        <div class="client-input-with-toggle">
          <v-text-field
            v-model="phone"
            :placeholder="`Phone Number (+${selectedDialCode})`"
            variant="outlined"
            density="compact"
            autofocus
            :error-messages="phoneError"
            @keydown.enter="lookupPhone"
          />
          <NumberPadMenu
            :model-value="phone"
            :max-length="15"
            :prefix="`+${selectedDialCode}`"
            preserve-string
            :replace-on-open="false"
            aria-label="Open phone number pad"
            @update:model-value="phone = String($event)"
          >
            <template #activator="{ props }">
              <v-btn v-bind="props" class="client-input-toggle" variant="tonal">123</v-btn>
            </template>
          </NumberPadMenu>
        </div>
        <v-btn class="w-100 mb-4" variant="elevated" color="primary" :disabled="Boolean(phoneError) || !phone.trim()" @click="lookupPhone"
          >Continue</v-btn
        >
        <v-btn class="w-100" variant="tonal" @click="selectAnonymous">Prefer not to say</v-btn>
      </v-card-text>
    </v-card>

    <v-card v-else-if="step === 'new'">
      <v-card-title>New Client</v-card-title>
      <v-card-text>
        <v-text-field
          v-model="newClient.phone"
          placeholder="Phone Number"
          variant="outlined"
          density="compact"
          class="mb-2"
          readonly
        >
          <template #prepend>
            <span class="country-flag">{{ selectedFlag }}</span>
            <span>+{{ selectedDialCode }}</span>
          </template>
        </v-text-field>
        <div class="client-input-with-toggle">
          <v-text-field
            v-model="newClient.name"
            placeholder="Name"
            variant="outlined"
            density="compact"
            autofocus
          />
          <PopupKeyboardMenu v-model="newClient.name" aria-label="Open name keyboard">
            <template #activator="{ props }">
              <v-btn v-bind="props" class="client-input-toggle" variant="tonal">ABC</v-btn>
            </template>
          </PopupKeyboardMenu>
        </div>
        <div class="text-caption text-medium-emphasis mt-n1">
          Please don't use rediculous names like "nonameeeeee", or "ali4738432".
        </div>
      </v-card-text>
      <v-card-actions class="pa-4">
        <v-btn variant="text" @click="step = 'phone'">Back</v-btn>
        <v-spacer />
        <v-btn
          variant="elevated"
          color="primary"
          :disabled="!newClient.name.trim()"
          @click="createClient"
          >Save</v-btn
        >
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script lang="ts" setup>
import { ref, reactive, inject, watch, computed } from "vue";
import { getCountries, getCountryCallingCode, parsePhoneNumberFromString, type CountryCode } from "libphonenumber-js/min";
import { GetClientByPhone, CreateClient } from "../../wailsjs/go/main/App";
import { anonymousClient, toClient, type Client } from "@/lib/client";
import NumberPadMenu from "@/components/NumberPadMenu.vue";
import PopupKeyboardMenu from "@/components/PopupKeyboardMenu.vue";

const emit = defineEmits<{ select: [client: Client] }>();

const dialog = defineModel<boolean>({ default: false });
const dialogOpen = inject<import("vue").Ref<boolean>>("dialogOpen", { value: false } as any);
watch(dialog, (v) => {
  dialogOpen.value = v;
});

const step = ref<"phone" | "new">("phone");
const phone = ref("");
const selectedCountry = ref<CountryCode>("LB");
const newClient = reactive({ name: "", phone: "" });
const countryNames = new Intl.DisplayNames(["en"], { type: "region" });
const countries = getCountries().map((code) => ({
  code,
  name: countryNames.of(code) || code,
  displayName: `${countryNames.of(code) || code} (+${getCountryCallingCode(code)})`,
  dialCode: getCountryCallingCode(code),
  flag: flagForCountry(code),
})).sort((a, b) => a.name.localeCompare(b.name));
const selectedFlag = computed(() => flagForCountry(selectedCountry.value));
const selectedDialCode = computed(() => getCountryCallingCode(selectedCountry.value));
const phoneError = computed(() => phone.value.trim() && !normalizePhone(phone.value) ? "Enter a valid phone number" : "");

function flagForCountry(code: string) {
  return code.toUpperCase().replace(/[A-Z]/g, (letter) => String.fromCodePoint(letter.charCodeAt(0) + 127397));
}

function normalizePhone(value: string) {
  const parsed = parsePhoneNumberFromString(value, selectedCountry.value);
  return parsed?.isValid() ? parsed.number : value === "00000000" ? value : "";
}

async function lookupPhone() {
  const p = normalizePhone(phone.value);
  if (!p) return;
  try {
    const c = await GetClientByPhone(p);
    emit("select", toClient(c));
    dialog.value = false;
    reset();
  } catch {
    newClient.phone = p;
    newClient.name = "";
    step.value = "new";
  }
}

async function createClient() {
  try {
    const c = await CreateClient(newClient.name.trim(), newClient.phone);
    emit("select", toClient(c));
  } catch {
    emit("select", toClient({ id: 0, name: newClient.name.trim(), phone: newClient.phone }));
  }
  dialog.value = false;
  reset();
}

function selectAnonymous() {
  emit("select", anonymousClient());
  dialog.value = false;
  reset();
}

function reset() {
  step.value = "phone";
  phone.value = "";
  selectedCountry.value = "LB";
  newClient.name = "";
  newClient.phone = "";
}
</script>

<style scoped>
.country-flag {
  display: inline-block;
  margin-inline-end: 6px;
  line-height: 1;
}

.client-input-with-toggle {
  align-items: flex-start;
  display: flex;
  gap: 6px;
}

.client-input-with-toggle :deep(.v-input) {
  flex: 1 1 auto;
}

.client-input-toggle {
  flex: 0 0 48px;
  font-weight: 700;
  height: 40px;
  min-width: 48px;
}
</style>
