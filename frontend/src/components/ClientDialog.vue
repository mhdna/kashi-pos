<template>
  <!-- Reset only once the close animation is over: resetting on close put
       the phone step back on screen while the dialog was still fading. -->
  <v-dialog v-model="dialog" :max-width="step === 'phone' ? 640 : 760" @after-leave="reset">
    <v-card v-if="step === 'phone'" class="client-phone-card">
      <v-card-title class="font-weight-bold">Enter Phone Number</v-card-title>
      <v-card-text class="client-phone-layout">
        <div class="client-phone-form">
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
              variant="tonal"
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
              variant="tonal"
              @click="backspacePhone"
            >
              ⌫
            </v-btn>
          </div>
          </div>
          <div
            class="client-phone-error text-error text-caption text-center"
            :class="{ 'client-phone-error--hidden': !phoneError }"
          >
            {{ phoneError || " " }}
          </div>
          <v-btn
            class="client-phone-action-button w-100"
            variant="elevated"
            color="primary"
            :disabled="!phone.trim()"
            @click="lookupPhone"
          >
            Continue
          </v-btn>
          <v-btn
            class="client-phone-action-button w-100 mt-5"
            variant="tonal"
            @click="selectAnonymous"
          >
            Prefer not to say
          </v-btn>
        </div>
        <div class="client-phone-countries">
          <button
            type="button"
            class="client-phone-country-trigger mb-3"
            aria-haspopup="dialog"
            :aria-expanded="countryPickerOpen"
            @click="countryPickerOpen = true"
          >
            <span
              v-if="isLebanon(selectedCountry)"
              class="country-flag country-flag-lebanon"
              aria-label="Lebanon"
            />
            <span v-else class="country-flag">{{ selectedFlag }}</span>
            <span class="client-phone-country-trigger-name">{{ selectedCountryName }} (+{{ selectedDialCode }})</span>
            <span class="client-phone-country-trigger-more">All countries</span>
          </button>
          <div class="client-phone-country-cards" role="listbox" aria-label="Common countries">
            <button
              v-for="country in frequentCountries"
              :key="country.code"
              type="button"
              role="option"
              class="client-phone-country-card"
              :class="{ 'client-phone-country-card--active': country.code === selectedCountry }"
              :aria-selected="country.code === selectedCountry"
              @click="selectedCountry = country.code"
            >
              <span
                v-if="isLebanon(country.code)"
                class="country-flag country-flag-lebanon"
                aria-label="Lebanon"
              />
              <span v-else class="country-flag">{{ country.flag }}</span>
              <span class="client-phone-country-card-name">{{ country.name }}</span>
              <span class="client-phone-country-card-code">+{{ country.dialCode }}</span>
            </button>
          </div>
        </div>
      </v-card-text>

      <!-- Every country as a card, filling the whole dialog; tapping one
           picks it and closes this. -->
      <div
        v-if="countryPickerOpen"
        class="client-phone-picker"
        role="dialog"
        aria-label="Select country"
      >
        <div class="client-phone-picker-header">
          <span class="text-h6 font-weight-bold">Select country</span>
          <v-btn variant="tonal" size="large" @click="countryPickerOpen = false">Close</v-btn>
        </div>
        <div class="client-phone-picker-grid">
          <button
            v-for="country in pickerCountries"
            :key="country.code"
            type="button"
            class="client-phone-country-card"
            :class="{ 'client-phone-country-card--active': country.code === selectedCountry }"
            @click="pickCountry(country.code)"
          >
            <span
              v-if="isLebanon(country.code)"
              class="country-flag country-flag-lebanon"
              aria-label="Lebanon"
            />
            <span v-else class="country-flag">{{ country.flag }}</span>
            <span class="client-phone-country-card-name">{{ country.name }}</span>
            <span class="client-phone-country-card-code">+{{ country.dialCode }}</span>
          </button>
        </div>
      </div>
    </v-card>

    <v-card v-else-if="step === 'new'">
      <v-card-title>New Client</v-card-title>
      <v-card-text>
        <PhoneField
          ref="newPhoneField"
          v-model="newClient.phone"
          class="mb-2"
          :error-messages="newPhoneError"
          @focus="newPhoneActive = true"
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
        </PhoneField>
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
          @focus="newPhoneActive = false"
        />
        <PhonePad v-if="newPhoneActive" class="mt-3" @press="newPhoneField?.press($event)" />
        <InlineKeyboard v-else v-model="newClient.name" class="client-name-keyboard mt-3" />
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
  validatePhoneNumberLength,
  type CountryCode,
} from "libphonenumber-js/mobile";
import { GetClientByPhone, CreateClient } from "../../wailsjs/go/main/App";
import {
  anonymousClient,
  preventClientNameNumber,
  preventClientNameNumberInput,
  toClient,
  type Client,
} from "@/lib/client";
import { normalizeLebanesePhone } from "@/lib/phone";
import { playClickSound, playDtmfSound } from "@/lib/sound";
import InlineKeyboard from "@/components/InlineKeyboard.vue";
import PhoneField from "@/components/PhoneField.vue";
import PhonePad from "@/components/PhonePad.vue";

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
// newClient.phone is the number after the country code, as edited on the
// new-client step; createClient puts the country code back.
const newClient = reactive({ name: "", phone: "" });
const newPhoneField = ref<InstanceType<typeof PhoneField> | null>(null);
const newPhoneActive = ref(false);
const newPhoneError = ref("");
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
// Countries our clients' numbers actually come from (production, by client
// count), most common first; +1 is shared by Canada and the US, so both are
// listed. Anything else is still in the dropdown.
const frequentCountryCodes: CountryCode[] = [
  "LB", "IQ", "SY", "CI", "DE", "FR", "CG", "ID", "GA", "SL", "SA", "CA", "US",
  "ML", "GM", "AE", "GH", "CF", "SE", "MZ", "CH", "KW", "GB", "TG", "OM", "AF",
];
const frequentCountries = frequentCountryCodes
  .map((code) => countries.find((country) => country.code === code))
  .filter((country): country is (typeof countries)[number] => !!country);
// The full picker lists the common countries first, then everyone else A-Z.
const pickerCountries = [
  ...frequentCountries,
  ...countries.filter((country) => !frequentCountryCodes.includes(country.code)),
];
const countryPickerOpen = ref(false);
const selectedCountryName = computed(
  () => countries.find((country) => country.code === selectedCountry.value)?.name ?? selectedCountry.value,
);
function pickCountry(code: CountryCode) {
  selectedCountry.value = code;
  countryPickerOpen.value = false;
}
const selectedFlag = computed(() => flagForCountry(selectedCountry.value));
const selectedDialCode = computed(() => getCountryCallingCode(selectedCountry.value));
const phoneError = computed(() =>
  phoneSubmitted.value && phone.value.trim() && !normalizePhone(phone.value)
    ? isLebanon(selectedCountry.value)
      ? "Enter a valid Lebanese phone number"
      : "Enter a valid mobile phone number"
    : "",
);

watch([dialog, step], ([open, currentStep]) => {
  if (!open || currentStep !== "phone") return;
  void nextTick(() => phoneInput.value?.focus());
});

watch(() => newClient.phone, () => {
  newPhoneError.value = "";
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
  if (value === "00000000") return value;
  if (isLebanon(selectedCountry.value)) {
    const lebanese = normalizeLebanesePhone(value);
    if (lebanese) return lebanese;
  }
  const parsed = parsePhoneNumberFromString(value, selectedCountry.value);
  return parsed?.isValid() ? parsed.number : value === "00000000" ? value : "";
}

// Blocks typing past the selected country's longest possible number (by its
// numbering plan, so it allows for a typed national prefix like UK's 0).
// Lebanon uses its own fixed 8-digit cap, see normalizeLebanesePhone.
function exceedsPhoneLength(value: string) {
  const digits = value.replace(/\D/g, "");
  if (selectedCountry.value === "LB") return digits.length > 8;
  return digits.length > 15 || validatePhoneNumberLength(digits, selectedCountry.value) === "TOO_LONG";
}

function replaceSelection(value: string, start: number, end: number, inserted: string) {
  return value.slice(0, start) + inserted + value.slice(end);
}

function appendPhoneDigit(digit: string) {
  const input = phoneInput.value;
  const rawPhone = phone.value.replace(/\D/g, "");
  const start = input?.selectionStart ?? rawPhone.length;
  const end = input?.selectionEnd ?? start;
  if (exceedsPhoneLength(replaceSelection(rawPhone, start, end, digit))) return;
  playDtmfSound(digit);
  phone.value = rawPhone.slice(0, start) + digit + rawPhone.slice(end);
  phoneSubmitted.value = false;
  restorePhoneCaret(start + 1);
}

function preventPhoneLength(event: KeyboardEvent) {
  if (!/^\p{N}$/u.test(event.key)) return;
  const input = event.target as HTMLInputElement;
  const start = input.selectionStart ?? input.value.length;
  const end = input.selectionEnd ?? start;
  if (exceedsPhoneLength(replaceSelection(input.value, start, end, event.key))) event.preventDefault();
}

function preventPhoneLengthInput(event: InputEvent) {
  if (!event.data || !/\p{N}/u.test(event.data)) return;
  const input = event.target as HTMLInputElement;
  const start = input.selectionStart ?? input.value.length;
  const end = input.selectionEnd ?? start;
  if (exceedsPhoneLength(replaceSelection(input.value, start, end, event.data))) event.preventDefault();
}

function clearPhone() {
  playClickSound();
  phone.value = "";
  phoneSubmitted.value = false;
}

function backspacePhone() {
  playClickSound();
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
  } catch {
    const dial = `+${selectedDialCode.value}`;
    newClient.phone = p.startsWith(dial) ? p.slice(dial.length) : p;
    newClient.name = "";
    newPhoneError.value = "";
    newPhoneActive.value = false;
    step.value = "new";
  }
}

async function createClient() {
  if (!newClient.name.trim()) return;
  const phone = newClient.phone.startsWith("+") ? newClient.phone : normalizePhone(newClient.phone);
  if (!phone) {
    newPhoneError.value = isLebanon(selectedCountry.value)
      ? "Enter a valid Lebanese phone number"
      : "Enter a valid mobile phone number";
    return;
  }
  try {
    const c = await CreateClient(capitalizeName(newClient.name), phone);
    emit("select", toClient(c));
  } catch {
    emit("select", toClient({ id: 0, name: capitalizeName(newClient.name), phone }));
  }
  dialog.value = false;
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
}

function reset() {
  step.value = "phone";
  phone.value = "";
  phoneSubmitted.value = false;
  selectedCountry.value = "LB";
  countryPickerOpen.value = false;
  newClient.name = "";
  newClient.phone = "";
  newPhoneError.value = "";
  newPhoneActive.value = false;
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
  align-items: flex-start;
  display: flex;
  gap: 24px;
  justify-content: center;
}

.client-phone-card {
  position: relative;
}

.client-phone-country-trigger {
  align-items: center;
  background: transparent;
  border: 1px solid rgba(var(--v-border-color), 0.4);
  border-radius: 8px;
  color: inherit;
  cursor: pointer;
  display: grid;
  gap: 0 10px;
  grid-template-columns: auto 1fr;
  min-height: 56px;
  padding: 8px 12px;
  text-align: start;
  width: 100%;
}

.client-phone-country-trigger:hover {
  border-color: currentColor;
}

.client-phone-country-trigger-name {
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.client-phone-country-trigger-more {
  font-size: 0.75rem;
  grid-column: 2;
  opacity: 0.7;
}

.client-phone-picker {
  background: rgb(var(--v-theme-surface));
  display: flex;
  flex-direction: column;
  inset: 0;
  padding: 16px;
  position: absolute;
  z-index: 2;
}

.client-phone-picker-header {
  align-items: center;
  display: flex;
  justify-content: space-between;
  margin-bottom: 12px;
}

.client-phone-picker-grid {
  display: grid;
  gap: 10px;
  grid-template-columns: repeat(auto-fill, minmax(170px, 1fr));
  overflow-y: auto;
  padding: 2px;
}

.client-phone-countries {
  display: flex;
  flex: 1 1 0;
  flex-direction: column;
  min-width: 0;
  /* Same height as the numpad column so the cards scroll instead of
     stretching the dialog. */
  max-height: 560px;
}

.client-phone-country-cards {
  display: grid;
  gap: 8px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  overflow-y: auto;
  padding: 2px;
}

.client-phone-country-card {
  align-items: center;
  background: transparent;
  border: 1px solid rgba(var(--v-border-color), 0.3);
  border-radius: 8px;
  color: inherit;
  cursor: pointer;
  display: grid;
  gap: 0 8px;
  grid-template-columns: auto 1fr;
  min-height: 56px;
  padding: 8px 10px;
  text-align: start;
  transition:
    border-color 120ms ease,
    background-color 120ms ease;
}

.client-phone-country-card:hover {
  border-color: currentColor;
}

.client-phone-country-card--active {
  background: rgba(var(--v-theme-primary), 0.12);
  border-color: rgb(var(--v-theme-primary));
}

.client-phone-country-card-name {
  font-size: 0.85rem;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.client-phone-country-card-code {
  font-size: 0.75rem;
  grid-column: 2;
  opacity: 0.7;
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
  gap: 6px;
  grid-template-columns: repeat(3, 1fr);
  grid-auto-rows: 56px;
}

.client-phone-numpad-key {
  border: 1px solid rgba(var(--v-border-color), 0.18);
  border-radius: 4px;
  font-size: 2rem !important;
  height: 100%;
  min-width: 0;
  padding: 0;
  transition:
    border-color 120ms ease,
    background-color 120ms ease;
}

.client-phone-numpad-key:hover {
  border-color: currentColor;
}

.client-name-keyboard {
  margin-inline: auto;
  max-width: 700px;
}

.client-phone-action-button {
  font-size: 1rem;
  min-height: 52px;
}

.client-phone-error {
  min-height: 18px;
}

.client-phone-error--hidden {
  visibility: hidden;
}

@media (max-width: 600px) {
  .client-phone-layout {
    flex-direction: column;
  }

  .client-phone-numpad {
    justify-self: center;
  }
}
</style>
