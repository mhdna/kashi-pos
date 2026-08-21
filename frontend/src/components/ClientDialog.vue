<template>
  <v-dialog v-model="dialog" :max-width="step === 'phone' ? 320 : 620">
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
          <div class="client-phone-numpad">
          <div class="client-phone-numpad-display">
            <input
              ref="phoneInput"
              class="client-phone-numpad-display-input"
              :value="phone"
              placeholder="0"
              inputmode="tel"
              maxlength="15"
              autofocus
              aria-label="Phone number"
              @keydown.enter="lookupPhone"
              @keydown="preventPhoneLength"
              @beforeinput="preventPhoneLengthInput"
              @keydown.backspace="removePhoneAtCursor"
              @keydown.delete="removePhoneAtCursor($event, 'forward')"
              @input="handlePhoneInput"
            />
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
          <div v-if="phoneError" class="text-error text-caption text-center mt-2">{{ phoneError }}</div>
          <v-btn
            class="w-100 mt-4"
            variant="elevated"
            color="primary"
            :disabled="!phone.trim()"
            @click="lookupPhone"
          >
            Continue
          </v-btn>
          <v-btn class="w-100 mt-2" variant="tonal" @click="selectAnonymous"> Prefer not to say </v-btn>
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
          @keydown="preventClientNameNumber"
          @beforeinput="preventClientNameNumberInput"
          @keydown.enter="createClient"
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
import { ref, reactive, inject, watch, computed, nextTick } from "vue";
import {
  getCountries,
  getCountryCallingCode,
  parsePhoneNumberFromString,
  type CountryCode,
} from "libphonenumber-js/min";
import { GetClientByPhone, CreateClient } from "../../wailsjs/go/main/App";
import {
  anonymousClient,
  preventClientNameNumber,
  preventClientNameNumberInput,
  toClient,
  type Client,
} from "@/lib/client";
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
const phoneInput = ref<HTMLInputElement | null>(null);
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
const phoneDigitLimit = computed(() => (selectedCountry.value === "LB" ? 8 : 15));
const phoneError = computed(() =>
  phoneSubmitted.value && phone.value.trim() && !normalizePhone(phone.value)
    ? "Enter a valid phone number"
    : "",
);

watch([dialog, step], ([open, currentStep]) => {
  if (!open || currentStep !== "phone") return;
  void nextTick(() => phoneInput.value?.focus());
});

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
  const input = phoneInput.value;
  const rawPhone = phone.value.replace(/\D/g, "");
  const start = input?.selectionStart ?? rawPhone.length;
  const end = input?.selectionEnd ?? start;
  if (rawPhone.length - (end - start) >= phoneDigitLimit.value) return;
  phone.value = rawPhone.slice(0, start) + digit + rawPhone.slice(end);
  phoneSubmitted.value = false;
  restorePhoneCaret(start + 1);
}

function preventPhoneLength(event: KeyboardEvent) {
  if (!/^\p{N}$/u.test(event.key)) return;
  const input = event.target as HTMLInputElement;
  const start = input.selectionStart ?? input.value.length;
  const end = input.selectionEnd ?? start;
  const selectedDigits = (input.value.slice(start, end).match(/\d/g) || []).length;
  const currentDigits = phone.value.replace(/\D/g, "").length;
  if (currentDigits - selectedDigits >= phoneDigitLimit.value) event.preventDefault();
}

function preventPhoneLengthInput(event: InputEvent) {
  if (!event.data || !/\p{N}/u.test(event.data)) return;
  const input = event.target as HTMLInputElement;
  const start = input.selectionStart ?? input.value.length;
  const end = input.selectionEnd ?? start;
  const selectedDigits = (input.value.slice(start, end).match(/\d/g) || []).length;
  const insertedDigits = (event.data.match(/\d/g) || []).length;
  const currentDigits = phone.value.replace(/\D/g, "").length;
  if (currentDigits - selectedDigits + insertedDigits > phoneDigitLimit.value) {
    event.preventDefault();
  }
}

function clearPhone() {
  phone.value = "";
  phoneSubmitted.value = false;
}

function backspacePhone() {
  const input = phoneInput.value;
  if (!input) return;
  removePhoneAtCursor({
    target: input,
    preventDefault() {},
    stopPropagation() {},
  } as unknown as KeyboardEvent);
}

function handlePhoneInput(event: Event) {
  const input = event.target as HTMLInputElement;
  const caret = input.selectionStart ?? input.value.length;
  const digitsBeforeCaret = (input.value.slice(0, caret).match(/\d/g) || []).length;
  phone.value = input.value.replace(/\D/g, "");
  phoneSubmitted.value = false;
  restorePhoneCaret(digitsBeforeCaret);
}

function removePhoneAtCursor(
  event: KeyboardEvent,
  direction: "backward" | "forward" = "backward",
) {
  event.preventDefault();
  event.stopPropagation();
  const input = event.target as HTMLInputElement;
  const value = input.value;
  const start = input.selectionStart ?? value.length;
  const end = input.selectionEnd ?? start;
  const digitsBeforeStart = (value.slice(0, start).match(/\d/g) || []).length;
  const digitsBeforeEnd = (value.slice(0, end).match(/\d/g) || []).length;
  const rawPhone = phone.value.replace(/\D/g, "");
  let removeStart = digitsBeforeStart;
  let removeEnd = digitsBeforeEnd;
  let caretDigitIndex = digitsBeforeStart;

  if (start === end) {
    if (direction === "backward") {
      if (digitsBeforeStart === 0) return;
      removeStart = digitsBeforeStart - 1;
      removeEnd = digitsBeforeStart;
      caretDigitIndex = removeStart;
    } else {
      if (digitsBeforeStart >= rawPhone.length) return;
      removeEnd = digitsBeforeStart + 1;
    }
  }

  phone.value = rawPhone.slice(0, removeStart) + rawPhone.slice(removeEnd);
  phoneSubmitted.value = false;
  restorePhoneCaret(caretDigitIndex);
}

function restorePhoneCaret(digitIndex: number) {
  void nextTick(() => {
    const input = phoneInput.value;
    if (!input) return;
    input.focus();
    const formatted = phone.value;
    let position = 0;
    let digitsSeen = 0;
    while (position < formatted.length && digitsSeen < digitIndex) {
      if (/\d/.test(formatted[position] ?? "")) digitsSeen++;
      position++;
    }
    input.setSelectionRange(position, position);
  });
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
  if (!newClient.name.trim()) return;
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
  display: flex;
  justify-content: center;
}

.client-phone-form {
  width: 100%;
  min-width: 0;
  max-width: 280px;
}

.client-phone-numpad {
  margin-top: 24px;
  width: 100%;
}

.client-phone-numpad-display {
  align-items: baseline;
  display: flex;
  font-size: var(--kashi-font-size-2xl);
  font-weight: 700;
  justify-content: center;
  margin-bottom: 12px;
  min-height: 40px;
  overflow: hidden;
}

.client-phone-numpad-display-input {
  background: transparent;
  border: 0;
  color: inherit;
  flex: 0 1 auto;
  font: inherit;
  min-width: 0;
  outline: 0;
  padding: 0;
  text-align: center;
  width: 150px;
}

.client-phone-numpad-display-input::placeholder {
  color: inherit;
  opacity: 0.55;
}

.client-phone-numpad-grid {
  display: grid;
  gap: 4px;
  grid-template-columns: repeat(3, 1fr);
}

.client-phone-numpad-key {
  font-size: var(--kashi-font-size-xl);
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
