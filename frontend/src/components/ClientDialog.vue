<template>
  <!-- Reset only once the close animation is over: resetting on close put
       the form back to empty while the dialog was still fading. -->
  <k-dialog v-model="dialog" :max-width="dialogWidth" @after-leave="reset">
    <k-card class="client-phone-card">
      <k-card-title v-if="!countryPickerOpen" class="font-weight-bold">{{ t("Client") }}</k-card-title>
      <!-- One form: the phone number on the left; on the right, the client
           the number found, or the name and where they heard about us. -->
      <k-card-text v-if="!countryPickerOpen" class="client-form">
        <div class="client-form-phone">
          <button
            type="button"
            class="client-phone-country-trigger mt-2"
            :aria-label="t('Country: choose or search')"
            aria-haspopup="dialog"
            :aria-expanded="countryPickerOpen"
            @click="countryPickerOpen = true"
          >
            <span class="client-phone-country-trigger-label">{{ t("Country") }}</span>
            <span class="country-flag">{{ selectedFlag }}</span>
            <span class="client-phone-country-trigger-name">{{ selectedCountryName }} (+{{ selectedDialCode }})</span>
            <span class="client-phone-country-trigger-search" aria-hidden="true">
              <k-icon icon="mdi-magnify" size="22" />
              {{ t("Search") }}
            </span>
          </button>
          <div
            class="client-phone-error text-error text-caption text-center"
            :class="{ 'client-phone-error--hidden': !phoneError && !phoneLengthHint }"
          >
            {{ phoneError || phoneLengthHint || " " }}
          </div>
          <div class="client-phone-numpad">
            <div class="client-phone-numpad-display">
              <span class="client-phone-prefix" :aria-label="t('Country code')">+{{ selectedDialCode }}</span>
              <input
                ref="phoneInput"
                class="client-phone-numpad-display-input"
                :value="phone"
                placeholder="0"
                inputmode="tel"
                maxlength="15"
                autofocus
                :aria-label="t('Phone number')"
                @keydown.enter="enterOnPhone"
                @keydown="preventPhoneLength"
                @beforeinput="preventPhoneLengthInput"
                @keydown.backspace="removePhoneAtCursor"
                @keydown.delete="removePhoneAtCursor($event, 'forward')"
                @input="handlePhoneInput"
              />
            </div>
            <PhonePad class="client-phone-pad" :show-arrows="false" @press="pressPhoneKey" />
          </div>
        </div>

        <div class="client-form-details">
          <div v-if="foundClient" class="client-found" data-test="client-found">
            <div class="text-body-2 text-medium-emphasis">{{ t("Existing client") }}</div>
            <div class="client-found-name">{{ foundClient.name }}</div>
          </div>
          <template v-else>
            <k-text-field
              ref="nameField"
              v-model="newClient.name"
              :placeholder="t('Name')"
              variant="outlined"
              density="compact"
              maxlength="80"
              hide-details
              @keydown="preventClientNameNumber"
              @beforeinput="preventClientNameNumberInput"
              @keydown.enter="submit"
            />
            <InlineKeyboard v-model="newClient.name" class="client-name-keyboard mt-3" />
            <div
              v-if="sources.length > 0"
              class="client-source mt-4"
              role="radiogroup"
              :aria-label="t('Where did they hear about us?')"
            >
              <div class="client-source-label text-body-2 font-weight-medium mb-2">{{ t("Where did they hear about us?") }}</div>
              <div class="client-source-options">
                <k-btn
                  v-for="option in sources"
                  :key="option.uid"
                  class="client-source-option"
                  :class="{ 'client-source-option--picked': option.uid === pickedSource }"
                  :color="sourceLook(option).color"
                  :style="{ '--source-ring': sourceLook(option).color }"
                  data-test="client-source-option"
                  role="radio"
                  :aria-checked="option.uid === pickedSource"
                  :append-icon="option.uid === pickedSource ? 'mdi-check' : undefined"
                  variant="flat"
                  @click="pickedSource = toggleSource(pickedSource, option.uid)"
                >
                  <k-icon
                    class="client-source-icon"
                    data-test="client-source-icon"
                    :icon="sourceLook(option).icon"
                    size="22"
                  />
                  {{ t(option.name) }}
                </k-btn>
              </div>
            </div>
          </template>
          <div class="client-form-actions">
            <k-btn class="client-phone-action-button" variant="tonal" @click="selectAnonymous">
              {{ t("Prefer not to say") }}
            </k-btn>
            <k-btn
              class="client-phone-action-button"
              variant="elevated"
              color="primary"
              :disabled="!canSubmitClientForm(lookup, newClient.name, sources, pickedSource)"
              data-test="client-save"
              @click="submit"
            >
              {{ t(clientFormButton(lookup)) }}
            </k-btn>
          </div>
        </div>
      </k-card-text>

      <!-- Every country as a card, replacing the form; tapping one
           picks it and goes back. Search is typed on the on-screen keyboard
           below, by country name or dial code. -->
      <div v-else class="client-phone-picker" role="dialog" :aria-label="t('Select country')">
        <div class="client-phone-picker-header">
          <span class="text-h6 font-weight-bold">{{ t("Select country") }}</span>
          <k-btn variant="tonal" size="large" @click="closeCountryPicker">{{ t("Close") }}</k-btn>
        </div>
        <k-text-field
          :model-value="countrySearch"
          class="client-phone-picker-search"
          :placeholder="t('Search country or code')"
          variant="outlined"
          density="comfortable"
          prepend-inner-icon="mdi-magnify"
          hide-details
          readonly
          clearable
          @click:clear="countrySearch = ''"
        />
        <div class="client-phone-picker-grid">
          <button
            v-for="country in pickerCountries"
            :key="country.code"
            type="button"
            class="client-phone-country-card"
            :class="{ 'client-phone-country-card--active': country.code === selectedCountry }"
            @click="pickCountry(country.code)"
          >
            <span class="country-flag">{{ country.flag }}</span>
            <span class="client-phone-country-card-name">{{ country.name }}</span>
            <span class="client-phone-country-card-code">+{{ country.dialCode }}</span>
          </button>
          <div v-if="pickerCountries.length === 0" class="client-phone-picker-empty text-medium-emphasis">
            {{ t('No country matches "{query}"', { query: countrySearch }) }}
          </div>
        </div>
        <InlineKeyboard v-model="countrySearch" class="client-phone-picker-keyboard" layout="code" :max-length="30" />
      </div>
    </k-card>
  </k-dialog>
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
import { GetClientByPhone, CreateClient, GetClientSources } from "../../wailsjs/go/main/App";
import {
  canSaveNewClient,
  sourceLook,
  sourceOptions,
  sourceToSend,
  toggleSource,
  type ClientSourceOption,
} from "@/lib/clientSource";
import { canSubmitClientForm, clientFormButton, type ClientLookup } from "@/lib/clientForm";
import { t } from "@/i18n";
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
import PhonePad from "@/components/PhonePad.vue";

const emit = defineEmits<{ select: [client: Client] }>();

const dialog = defineModel<boolean>({ default: false });
const dialogOpen = inject<import("vue").Ref<boolean>>("dialogOpen", { value: false } as any);
watch(dialog, (v) => {
  dialogOpen.value = v;
});

const phone = ref("");
const phoneInput = ref<HTMLInputElement | null>(null);
const phoneSubmitted = ref(false);
const selectedCountry = ref<CountryCode>("LB");
const newClient = reactive({ name: "" });
const nameField = ref<{ focus?: () => void } | null>(null);
// Where the new customer heard about us; loaded when the dialog opens.
const sources = ref<ClientSourceOption[]>([]);
const pickedSource = ref("");
// The client the number belongs to, once a whole valid number is typed.
const lookup = ref<ClientLookup>("none");
const foundClient = ref<Client | null>(null);
// The form is wide enough for the keypad beside the name keyboard.
const dialogWidth = computed(() => (countryPickerOpen.value ? 760 : 1100));

async function loadSources() {
  try {
    sources.value = sourceOptions(await GetClientSources());
  } catch {
    sources.value = [];
  }
}
const countryNames = new Intl.DisplayNames(["en"], { type: "region" });
// Left out of the country lists altogether.
const hiddenCountryCodes: string[] = ["IL"];
// The browser's own names, except where we use a different one.
const countryNameOverrides: Record<string, string> = { PS: "Palestine" };
const countries = getCountries()
  .filter((code) => !hiddenCountryCodes.includes(code))
  .map((code) => ({
    code,
    name: countryNameOverrides[code] || countryNames.of(code) || code,
    displayName: `${countryNameOverrides[code] || countryNames.of(code) || code} (+${getCountryCallingCode(code)})`,
    dialCode: getCountryCallingCode(code),
    flag: flagForCountry(code),
  }))
  .sort((a, b) => a.name.localeCompare(b.name));
// Countries our clients' numbers actually come from (production, by client
// count), most common first; +1 is shared by Canada and the US, so both are
// listed. Anything else is still in the dropdown.
const frequentCountryCodes: CountryCode[] = [
  "LB", "IQ", "SY", "PS", "CI", "DE", "FR", "CG", "ID", "GA", "SL", "SA", "CA", "US",
  "ML", "GM", "AE", "GH", "CF", "SE", "MZ", "CH", "KW", "GB", "TG", "OM", "AF",
];
const frequentCountries = frequentCountryCodes
  .map((code) => countries.find((country) => country.code === code))
  .filter((country): country is (typeof countries)[number] => !!country);
// The full picker lists the common countries first, then everyone else A-Z.
const allPickerCountries = [
  ...frequentCountries,
  ...countries.filter((country) => !frequentCountryCodes.includes(country.code)),
];
const countryPickerOpen = ref(false);
const countrySearch = ref("");
// Search by name ("uni"), two-letter code ("us") or dial code ("+1" or "1").
// Closest matches first: an exact code, then names starting with the text,
// then dial codes starting with it, then names containing it; ties keep the
// common-countries-first order.
const pickerCountries = computed(() => {
  const raw = countrySearch.value.trim().toLowerCase();
  if (!raw) return allPickerCountries;
  const dialOnly = raw.startsWith("+");
  const query = raw.replace(/^\+/, "");
  if (!query) return allPickerCountries;
  const digitsOnly = /^\d+$/.test(query);
  const rank = (country: (typeof countries)[number]) => {
    const name = country.name.toLowerCase();
    const iso = country.code.toLowerCase();
    if (digitsOnly) {
      if (country.dialCode === query) return 0;
      return country.dialCode.startsWith(query) ? 2 : -1;
    }
    if (dialOnly) return -1;
    if (iso === query) return 0;
    if (name.startsWith(query)) return 1;
    if (query.length <= 2 && iso.startsWith(query)) return 2;
    return name.includes(query) ? 3 : -1;
  };
  return allPickerCountries
    .map((country, index) => ({ country, index, score: rank(country) }))
    .filter((entry) => entry.score >= 0)
    .sort((a, b) => a.score - b.score || a.index - b.index)
    .map((entry) => entry.country);
});
function closeCountryPicker() {
  countryPickerOpen.value = false;
  countrySearch.value = "";
}
function pickCountry(code: CountryCode) {
  selectedCountry.value = code;
  closeCountryPicker();
}
const selectedCountryName = computed(
  () => countries.find((country) => country.code === selectedCountry.value)?.name ?? selectedCountry.value,
);
const selectedFlag = computed(() => flagForCountry(selectedCountry.value));
const selectedDialCode = computed(() => getCountryCallingCode(selectedCountry.value));
const phoneError = computed(() =>
  phoneSubmitted.value && phone.value.trim() && !normalizePhone(phone.value)
    ? isLebanon(selectedCountry.value)
      ? t("Enter a valid Lebanese phone number")
      : t("Enter a valid mobile phone number")
    : "",
);

watch(
  dialog,
  (open) => {
    if (!open) return;
    void loadSources();
    void nextTick(() => phoneInput.value?.focus());
  },
  { immediate: true },
);

// Each change of number or country looks the number up again once it is a
// whole valid one; an answer for an older number is dropped.
let lookupSeq = 0;
let pendingLookup: Promise<void> | null = null;
watch([phone, selectedCountry], () => {
  phoneSubmitted.value = false;
  phoneLengthHint.value = "";
  foundClient.value = null;
  const seq = ++lookupSeq;
  const p = normalizePhone(phone.value);
  if (!p) {
    lookup.value = "none";
    pendingLookup = null;
    return;
  }
  lookup.value = "checking";
  pendingLookup = (async () => {
    let found: Client | null = null;
    try {
      found = toClient(await GetClientByPhone(p));
    } catch {
      found = null;
    }
    if (seq !== lookupSeq) return;
    foundClient.value = found;
    lookup.value = found ? "found" : "new";
  })();
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
// Lebanon is 8 digits, plus the leading 0, 961 or 00961 that
// normalizeLebanesePhone also accepts.
function exceedsPhoneLength(value: string) {
  const digits = value.replace(/\D/g, "");
  if (isLebanon(selectedCountry.value)) {
    if (digits.length <= 8) return false;
    if (digits.startsWith("00961")) return digits.length > 13;
    if (digits.startsWith("961")) return digits.length > 11;
    if (digits.startsWith("0")) return digits.length > 9;
    return true;
  }
  return digits.length > 15 || validatePhoneNumberLength(digits, selectedCountry.value) === "TOO_LONG";
}

// Typing stops at the country's longest number without any other sign, which
// looks like a broken keypad -- most of all when a foreign number is typed
// while Lebanon is still selected. So say why it stopped.
const phoneLengthHint = ref("");
function blocksPhoneLength(value: string) {
  if (!exceedsPhoneLength(value)) return false;
  phoneLengthHint.value = isLebanon(selectedCountry.value)
    ? t("Lebanese numbers have 8 digits. For another country, tap Search.")
    : t("That's the longest number for {country}.", { country: selectedCountryName.value });
  return true;
}

function replaceSelection(value: string, start: number, end: number, inserted: string) {
  return value.slice(0, start) + inserted + value.slice(end);
}

function appendPhoneDigit(digit: string, silent = false) {
  const input = phoneInput.value;
  const rawPhone = phone.value.replace(/\D/g, "");
  const start = input?.selectionStart ?? rawPhone.length;
  const end = input?.selectionEnd ?? start;
  if (blocksPhoneLength(replaceSelection(rawPhone, start, end, digit))) return;
  if (!silent) playDtmfSound(digit);
  phone.value = rawPhone.slice(0, start) + digit + rawPhone.slice(end);
  phoneSubmitted.value = false;
  restorePhoneCaret(start + 1);
}

function preventPhoneLength(event: KeyboardEvent) {
  if (!/^\p{N}$/u.test(event.key)) return;
  const input = event.target as HTMLInputElement;
  const start = input.selectionStart ?? input.value.length;
  const end = input.selectionEnd ?? start;
  if (blocksPhoneLength(replaceSelection(input.value, start, end, event.key))) event.preventDefault();
}

function preventPhoneLengthInput(event: InputEvent) {
  if (!event.data || !/\p{N}/u.test(event.data)) return;
  const input = event.target as HTMLInputElement;
  const start = input.selectionStart ?? input.value.length;
  const end = input.selectionEnd ?? start;
  if (blocksPhoneLength(replaceSelection(input.value, start, end, event.data))) event.preventDefault();
}

function clearPhone(silent = false) {
  if (!silent) playClickSound();
  phone.value = "";
  phoneSubmitted.value = false;
}

function backspacePhone(silent = false) {
  if (!silent) playClickSound();
  const input = phoneInput.value;
  if (!input) return;
  removePhoneAtCursor({
    target: input,
    preventDefault() {},
    stopPropagation() {},
  } as unknown as KeyboardEvent);
}

// Keys from the shared PhonePad, which plays the key sound itself.
function pressPhoneKey(key: string) {
  if (/^\d$/.test(key)) appendPhoneDigit(key, true);
  else if (key === "clear") clearPhone(true);
  else if (key === "back") backspacePhone(true);
  else if (key === "left") movePhoneCaret(-1);
  else if (key === "right") movePhoneCaret(1);
}

function movePhoneCaret(step: number) {
  const input = phoneInput.value;
  if (!input) return;
  const hasSelection = input.selectionStart !== input.selectionEnd;
  const from = (step < 0 ? input.selectionStart : input.selectionEnd) ?? input.value.length;
  const next = hasSelection ? from : Math.min(Math.max(from + step, 0), input.value.length);
  input.focus();
  input.setSelectionRange(next, next);
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

// Enter on the number: a known client is picked, otherwise on to the name.
async function enterOnPhone() {
  phoneSubmitted.value = true;
  if (!normalizePhone(phone.value)) return;
  if (pendingLookup) await pendingLookup;
  if (lookup.value === "found") await submit();
  else nameField.value?.focus?.();
}

async function submit() {
  phoneSubmitted.value = true;
  const p = normalizePhone(phone.value);
  if (!p) return;
  if (pendingLookup) await pendingLookup;
  if (lookup.value === "found" && foundClient.value) {
    emit("select", foundClient.value);
    dialog.value = false;
    return;
  }
  if (!canSubmitClientForm(lookup.value, newClient.name, sources.value, pickedSource.value)) return;
  const name = capitalizeName(newClient.name);
  try {
    const c = await CreateClient(name, p, sourceToSend(sources.value, pickedSource.value));
    emit("select", toClient(c));
  } catch {
    emit("select", toClient({ id: 0, name, phone: p }));
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
  phone.value = "";
  phoneSubmitted.value = false;
  selectedCountry.value = "LB";
  closeCountryPicker();
  newClient.name = "";
  pickedSource.value = "";
}
</script>

<style scoped>
/* The keypad column on the left; the found client, or the name, keyboard
   and options, on the right, with the buttons at its foot. */
.client-form {
  align-items: stretch;
  display: flex;
  gap: 28px;
}
.client-form-phone {
  flex: 0 0 340px;
  min-width: 0;
}
.client-form-details {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-width: 0;
}
.client-found {
  padding-top: 8px;
}
.client-found-name {
  font-size: var(--kashi-font-size-2xl);
  font-weight: 700;
  overflow-wrap: anywhere;
}
.client-form-actions {
  display: grid;
  gap: 12px;
  grid-template-columns: 1fr 2fr;
  margin-top: auto;
  padding-top: 16px;
}
@media (max-width: 900px) {
  .client-form {
    flex-direction: column;
  }
  .client-form-phone {
    flex-basis: auto;
  }
}
/* The options side by side, wrapping when there are many. */
.client-source-options {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.client-source-option {
  flex: 1 1 0;
  min-height: 52px;
  min-width: 150px;
  font-size: 1rem;
}
.client-source-icon {
  margin-inline-end: 8px;
}
/* Each option is filled in its colour; the picked one gets a ring in the
   same colour around it, and a check. */
.client-source-option--picked {
  outline: 3px solid var(--source-ring);
  outline-offset: 2px;
}
.country-flag {
  display: inline-block;
  margin-inline-end: 6px;
  line-height: 1;
  /* WebKit (the macOS webview) lets a colour-font flag leak its colour into the
     text drawn after it; painting the flag in its own box stops that. */
  contain: paint;
}

.client-phone-card {
  position: relative;
}

.client-phone-country-trigger {
  align-items: center;
  background: transparent;
  border: 1px solid rgba(var(--k-border-color), 0.55);
  border-radius: 0;
  color: inherit;
  cursor: pointer;
  display: grid;
  gap: 0 10px;
  grid-template-columns: auto 1fr auto;
  min-height: 56px;
  padding: 0 8px 0 12px;
  position: relative;
  text-align: start;
  width: 100%;
}

.client-phone-country-trigger:hover {
  border-color: currentColor;
}

/* Sits on the top border like an outlined field's label. */
.client-phone-country-trigger-label {
  background: rgb(var(--k-theme-surface));
  font-size: 0.75rem;
  left: 10px;
  line-height: 1;
  opacity: 0.75;
  padding: 0 4px;
  position: absolute;
  top: -6px;
}

.client-phone-country-trigger-name {
  font-size: 0.95rem;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.client-phone-country-trigger-search {
  align-items: center;
  /* Text only, no fill, so it isn't mistaken for Continue. */
  background: transparent;
  border-radius: 0;
  color: rgb(var(--k-theme-on-surface));
  display: flex;
  font-size: 0.95rem;
  font-weight: 500;
  gap: 6px;
  height: 40px;
  padding: 0 14px;
}

.client-phone-picker {
  display: flex;
  flex-direction: column;
  height: min(92vh, 760px);
  padding: 16px;
}

.client-phone-picker-header {
  align-items: center;
  display: flex;
  justify-content: space-between;
  margin-bottom: 12px;
}

.client-phone-picker-search {
  flex: 0 0 auto;
  margin-bottom: 10px;
}

.client-phone-picker-keyboard {
  flex: 0 0 auto;
  margin-inline: auto;
  margin-top: 10px;
  max-width: 700px;
  width: 100%;
}

.client-phone-picker-empty {
  grid-column: 1 / -1;
  padding: 24px;
  text-align: center;
}

.client-phone-picker-grid {
  display: grid;
  gap: 6px;
  flex: 1 1 0;
  grid-template-columns: repeat(auto-fill, minmax(170px, 1fr));
  align-content: start;
  min-height: 0;
  overflow-y: auto;
  padding: 2px;
}

.client-phone-country-card {
  align-items: center;
  background: transparent;
  border: 1px solid rgba(var(--k-border-color), 0.18);
  border-radius: 0;
  color: inherit;
  cursor: pointer;
  display: grid;
  gap: 0 8px;
  grid-template-columns: auto 1fr auto;
  min-height: 48px;
  padding: 0 10px;
  text-align: start;
}

.client-phone-country-card .country-flag,
.client-phone-country-trigger .country-flag {
  font-size: 1.9rem;
  margin-inline-end: 0;
}

.client-phone-country-card:hover {
  border-color: currentColor;
}

.client-phone-country-card--active {
  background: rgb(var(--k-theme-primary));
  border-color: rgb(var(--k-theme-primary));
  color: rgb(var(--k-theme-on-primary));
}

.client-phone-country-card-name {
  font-size: 0.9rem;
  font-weight: 500;
  line-height: 1.2;
  overflow-wrap: anywhere;
}

.client-phone-country-card-code {
  font-size: 0.85rem;
  font-variant-numeric: tabular-nums;
  opacity: 0.65;
}

.client-phone-numpad {
  margin-top: 8px;
  width: 100%;
}

.client-phone-prefix {
  color: rgba(var(--k-theme-on-surface), 0.6);
  font-size: var(--kashi-font-size-xl);
  justify-self: start;
  margin-inline-start: 12px;
}

/* Prefix | number | empty column of the same width, so the number sits on
   the keypad's centre line rather than in the space the prefix leaves. */
.client-phone-numpad-display {
  align-items: baseline;
  display: grid;
  font-size: var(--kashi-font-size-2xl);
  font-weight: 700;
  grid-template-columns: 1fr auto 1fr;
  margin-bottom: 12px;
  min-height: 40px;
  overflow: hidden;
}

.client-phone-numpad-display-input {
  background: transparent;
  border: 0;
  color: inherit;
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

.client-name-keyboard {
  margin-inline: auto;
  max-width: 700px;
}

.client-phone-action-button {
  font-size: 1rem;
  min-height: 52px;
}

.client-phone-error {
  /* Two lines reserved, so a message never moves the number or the keypad. */
  align-items: flex-end;
  display: flex;
  justify-content: center;
  min-height: 2.6em;
  line-height: 1.3;
}

.client-phone-error--hidden {
  visibility: hidden;
}

</style>
