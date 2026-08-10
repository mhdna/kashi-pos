<template>
  <v-dialog v-model="dialog" :max-width="step === 'phone' ? 640 : 620">
    <v-card v-if="step === 'phone'">
      <v-card-title class="font-weight-bold">Enter Phone Number</v-card-title>
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
              <span
                v-if="isLebanon(item.code)"
                class="country-flag country-flag-lebanon"
                aria-label="Lebanon"
              />
              <span v-else class="country-flag">{{ item.flag }}</span>
              <span>{{ item.displayName }}</span>
            </template>
            <template #item="{ props: itemProps, item }">
              <v-list-item v-bind="itemProps" :subtitle="`+${item.dialCode}`">
                <template #title>
                  <span
                    v-if="isLebanon(item.code)"
                    class="country-flag country-flag-lebanon"
                    aria-label="Lebanon"
                  />
                  <span v-else class="country-flag">{{ item.flag }}</span>
                  <span>{{ item.name }}</span>
                </template>
              </v-list-item>
            </template>
          </v-autocomplete>
          <v-text-field
            :model-value="formatLebanesePhone(phone)"
            :placeholder="`Phone Number (+${selectedDialCode})`"
            variant="outlined"
            density="compact"
            autofocus
            maxlength="15"
            :error-messages="phoneError"
            @keydown.enter="lookupPhone"
            @update:model-value="phone = $event.replace(/\D/g, '')"
          />
          <v-btn
            class="w-100 mb-4"
            variant="elevated"
            color="primary"
            :disabled="!phone.trim()"
            @click="lookupPhone"
          >
            Continue
          </v-btn>
          <v-btn class="w-100" variant="tonal" @click="selectAnonymous"> Prefer not to say </v-btn>
        </div>

        <div class="client-phone-numpad">
          <div class="client-phone-numpad-display">
            <span class="client-phone-numpad-prefix">+{{ selectedDialCode }}</span>
            <span>{{ formatLebanesePhone(phone) || "0" }}</span>
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
          :model-value="formatLebanesePhone(newClient.phone)"
          placeholder="Phone Number"
          variant="outlined"
          density="compact"
          class="mb-2"
          readonly
          @update:model-value="newClient.phone = $event.replace(/\D/g, '')"
        >
          <template #prepend>
            <span
              v-if="isLebanon(selectedCountry)"
              class="country-flag country-flag-lebanon"
              aria-label="Lebanon"
            />
            <span v-else class="country-flag">{{ selectedFlag }}</span>
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
        <!-- <div class="text-caption text-medium-emphasis mt-n1"> -->
        <!--   Please don't use rediculous names like "nonameeeeee", or "ali4738432". -->
        <!-- </div> -->
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
import {
  getCountries,
  getCountryCallingCode,
  parsePhoneNumberFromString,
  type CountryCode,
} from "libphonenumber-js/min";
import { GetClientByPhone, CreateClient } from "../../wailsjs/go/main/App";
import { anonymousClient, toClient, type Client } from "@/lib/client";
import { formatLebanesePhone } from "@/lib/phone";
import InlineKeyboard from "@/components/InlineKeyboard.vue";

const emit = defineEmits<{ select: [client: Client] }>();

const dialog = defineModel<boolean>({ default: false });
const dialogOpen = inject<import("vue").Ref<boolean>>("dialogOpen", { value: false } as any);
watch(dialog, (v) => {
  dialogOpen.value = v;
});

const step = ref<"phone" | "new">("phone");
const phone = ref("");
const phoneSubmitted = ref(false);
const selectedCountry = ref<CountryCode>("LB");
const newClient = reactive({ name: "", phone: "" });
const phonePadDigits = ["1", "2", "3", "4", "5", "6", "7", "8", "9"];
const countryNames = new Intl.DisplayNames(["en"], { type: "region" });
const countries = getCountries()
  .map((code) => ({
    code,
    name: countryNames.of(code) || code,
    displayName: `${countryNames.of(code) || code} (+${getCountryCallingCode(code)})`,
    dialCode: getCountryCallingCode(code),
    flag: flagForCountry(code),
  }))
  .sort((a, b) => a.name.localeCompare(b.name));
const selectedFlag = computed(() => flagForCountry(selectedCountry.value));
const selectedDialCode = computed(() => getCountryCallingCode(selectedCountry.value));
const phoneError = computed(() =>
  phoneSubmitted.value && phone.value.trim() && !normalizePhone(phone.value)
    ? "Enter a valid phone number"
    : "",
);

watch([phone, selectedCountry], () => {
  phoneSubmitted.value = false;
});

function flagForCountry(code: string) {
  return code
    .toUpperCase()
    .replace(/[A-Z]/g, (letter) => String.fromCodePoint(letter.charCodeAt(0) + 127397));
}

function isLebanon(code: string) {
  return code.toUpperCase() === "LB";
}

function normalizePhone(value: string) {
  const parsed = parsePhoneNumberFromString(value, selectedCountry.value);
  return parsed?.isValid() ? parsed.number : value === "00000000" ? value : "";
}

function appendPhoneDigit(digit: string) {
  if (phone.value.length >= 15) return;
  phone.value += digit;
  phoneSubmitted.value = false;
}

function clearPhone() {
  phone.value = "";
  phoneSubmitted.value = false;
}

function backspacePhone() {
  phone.value = phone.value.slice(0, -1);
  phoneSubmitted.value = false;
}

async function lookupPhone() {
  phoneSubmitted.value = true;
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
    const c = await CreateClient(capitalizeName(newClient.name), newClient.phone);
    emit("select", toClient(c));
  } catch {
    emit(
      "select",
      toClient({ id: 0, name: capitalizeName(newClient.name), phone: newClient.phone }),
    );
  }
  dialog.value = false;
  reset();
}

function capitalizeName(value: string) {
  return value
    .trim()
    .replace(
      /(^|[\s'-])([\p{L}])/gu,
      (_, prefix: string, letter: string) => `${prefix}${letter.toUpperCase()}`,
    );
}

function selectAnonymous() {
  emit("select", anonymousClient());
  dialog.value = false;
  reset();
}

function reset() {
  step.value = "phone";
  phone.value = "";
  phoneSubmitted.value = false;
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

.country-flag-lebanon {
  position: relative;
  width: 24px;
  height: 16px;
  vertical-align: -3px;
  border-radius: 2px;
  background: linear-gradient(to bottom, #ed1c24 0 25%, #fff 25% 75%, #ed1c24 75%);
  box-shadow: 0 0 0 1px rgb(0 0 0 / 12%);
}

.country-flag-lebanon::after {
  content: "";
  position: absolute;
  inset: 3px 9px 3px 9px;
  background: #16863d;
  clip-path: polygon(
    50% 0,
    62% 28%,
    100% 18%,
    72% 48%,
    100% 72%,
    62% 62%,
    50% 100%,
    38% 62%,
    0 72%,
    28% 48%,
    0 18%,
    38% 28%
  );
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
  font-size: var(--kashi-font-size-2xl);
  font-weight: 700;
  justify-content: center;
  min-height: 40px;
  overflow: hidden;
}

.client-phone-numpad-prefix {
  font-size: var(--kashi-font-size-xs);
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
  font-size: var(--kashi-font-size-base);
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
