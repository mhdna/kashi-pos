<template>
  <v-dialog v-model="dialog" :max-width="step === 'phone' ? 640 : 620">
    <v-card v-if="step === 'phone'">
      <v-card-title>Enter Phone Number</v-card-title>
      <v-card-text class="client-phone-layout">
        <div class="client-phone-form">
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
          <v-text-field
            v-model="phone"
            :placeholder="`Phone Number (+${selectedDialCode})`"
            variant="outlined"
            density="compact"
            autofocus
            maxlength="15"
            :error-messages="phoneError"
            @keydown.enter="lookupPhone"
          />
          <v-btn
            class="w-100 mb-4"
            variant="elevated"
            color="primary"
            :disabled="Boolean(phoneError) || !phone.trim()"
            @click="lookupPhone"
          >
            Continue
          </v-btn>
          <v-btn class="w-100" variant="tonal" @click="selectAnonymous">
            Prefer not to say
          </v-btn>
        </div>

        <div class="client-phone-numpad">
          <div class="client-phone-numpad-display">
            <span class="client-phone-numpad-prefix">+{{ selectedDialCode }}</span>
            <span>{{ phone || "0" }}</span>
          </div>
          <div class="client-phone-numpad-grid">
            <v-btn
              v-for="digit in phonePadDigits"
              :key="digit"
              class="client-phone-numpad-key"
              size="small"
              variant="tonal"
              @click="appendPhoneDigit(digit)"
            >
              {{ digit }}
            </v-btn>
            <v-btn
              aria-label="Clear phone number"
              class="client-phone-numpad-key"
              size="small"
              variant="text"
              @click="clearPhone"
            >
              C
            </v-btn>
            <v-btn
              class="client-phone-numpad-key"
              size="small"
              variant="tonal"
              @click="appendPhoneDigit('0')"
            >
              0
            </v-btn>
            <v-btn
              aria-label="Delete last phone digit"
              class="client-phone-numpad-key"
              size="small"
              variant="text"
              @click="backspacePhone"
            >
              ⌫
            </v-btn>
          </div>
        </div>
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
        <v-text-field
          v-model="newClient.name"
          placeholder="Name"
          variant="outlined"
          density="compact"
          maxlength="80"
          autofocus
        />
        <div class="text-caption text-medium-emphasis mt-n1">
          Please don't use rediculous names like "nonameeeeee", or "ali4738432".
        </div>
        <InlineKeyboard v-model="newClient.name" class="client-name-keyboard mt-3" />
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
import InlineKeyboard from "@/components/InlineKeyboard.vue";

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
const phonePadDigits = ["1", "2", "3", "4", "5", "6", "7", "8", "9"];
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

function appendPhoneDigit(digit: string) {
  if (phone.value.length >= 15) return;
  phone.value += digit;
}

function clearPhone() {
  phone.value = "";
}

function backspacePhone() {
  phone.value = phone.value.slice(0, -1);
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

.client-phone-layout {
  display: grid;
  gap: 20px;
  grid-template-columns: minmax(0, 1fr) 176px;
}

.client-phone-form {
  min-width: 0;
}

.client-phone-numpad {
  align-self: start;
  width: 176px;
}

.client-phone-numpad-display {
  align-items: baseline;
  display: flex;
  font-size: 1.4rem;
  font-weight: 700;
  justify-content: center;
  min-height: 40px;
  overflow: hidden;
}

.client-phone-numpad-prefix {
  font-size: 0.72em;
  font-weight: 600;
  margin-inline-end: 4px;
  opacity: 0.7;
}

.client-phone-numpad-grid {
  display: grid;
  gap: 4px;
  grid-template-columns: repeat(3, 1fr);
}

.client-phone-numpad-key {
  font-size: 1rem;
  height: 36px;
  min-width: 0;
  padding: 0;
}

.client-name-keyboard {
  margin-inline: auto;
  max-width: 540px;
}

@media (max-width: 600px) {
  .client-phone-layout {
    grid-template-columns: 1fr;
  }

  .client-phone-numpad {
    justify-self: center;
  }
}
</style>
