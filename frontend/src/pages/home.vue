<template>
  <div
    class="home-page d-flex flex-column h-100 position-relative"
    :style="{ width: '100%', overflow: 'hidden', '--home-search-keyboard-height': `${searchKeyboardHeight}px` }"
  >
    <div class="home-panes d-flex">
      <div class="home-left-pane d-flex flex-column" style="flex: 5 5 0; min-width: 0; min-height: 0">
        <div>
          <v-card-title
            class="d-flex justify-space-between"
            style="font-size: var(--kashi-font-size-2xl)"
          >
            <div class="d-flex flex-column">
              <div class="font-weight-bold">{{ pageModeTitle }}</div>
              <v-card-subtitle v-if="pageMode === 'sale' || pageMode === 'exchange'" class="pa-0">{{ invoiceCode }}</v-card-subtitle>
            </div>
            <div class="d-flex flex-column align-end">
              <div class="d-flex align-center ga-2">
                <div
                  class="required-assignees d-flex align-center ga-2"
                  :class="{ 'required-assignees--missing': missingAssignmentFlash }"
                >
                  <SalespersonsDialog v-if="pageMode !== 'settlement'" />
                  <ClientButton
                    v-if="pageMode === 'sale'"
                    :always-anonymous-available="alwaysAnonymousClientEnabled"
                  />
                </div>
                <NoteButton :note="invoiceNotes" @openDialog="openNoteDialog" />
                <v-dialog v-model="noteDialog" max-width="760">
                  <v-card>
                    <v-card-title>Add a note</v-card-title>
                    <v-card-text>
                      <v-textarea
                        ref="noteInput"
                        v-model="noteDraft"
                        auto-grow
                        autofocus
                        hide-details
                        placeholder="Add a note"
                        rows="3"
                        variant="outlined"
                        @keydown.ctrl.enter="saveNote"
                      />
                      <InlineKeyboard v-model="noteDraft" :max-length="500" class="mt-3" />
                    </v-card-text>
                    <v-card-actions class="pa-4">
                      <v-btn
                        v-if="invoiceNotes"
                        variant="text"
                        color="error"
                        @click="removeNote"
                      >
                        Remove
                      </v-btn>
                      <v-spacer />
                      <v-btn variant="text" @click="noteDialog = false">Cancel</v-btn>
                      <v-btn color="primary" variant="elevated" @click="saveNote">Add</v-btn>
                    </v-card-actions>
                  </v-card>
                </v-dialog>
              </div>
            </div>
          </v-card-title>
        </div>
        <div class="d-flex align-center ms-4 mt-4" :class="pageMode === 'settlement' ? 'me-4' : 'me-0'">
          <v-text-field
            ref="barcodeInput"
            v-model="barcodeValue"
            class="flex-grow-1"
            :class="barcodeFeedback ? `barcode-flash-${barcodeFeedback}` : ''"
            density="compact"
            hide-details="auto"
            :placeholder="scanByCodeEnabled ? 'Enter a barcode or a code' : 'Scan or enter barcode'"
            :disabled="pageMode === 'settlement'"
            variant="outlined"
            @blur="onBlur"
            @keydown.enter="onBarcodeEnter"
          />
          <v-btn
            v-if="pageMode !== 'settlement'"
            icon
            class="ms-1"
            variant="text"
            aria-label="Invoices on hold"
            @click="onHoldMenu?.open()"
          >
            <v-icon icon="mdi-pause" size="26" />
          </v-btn>
          <v-btn
            v-if="pageMode !== 'settlement'"
            icon
            class="ms-1"
            variant="text"
            aria-label="Exchange invoice"
            @click="returnInvoiceMenu?.open()"
          >
            <v-icon icon="mdi-autorenew" size="26" />
          </v-btn>
          <v-btn
            v-if="pageMode !== 'settlement'"
            icon
            class="ms-1"
            variant="text"
            aria-label="Add an unknown item"
            @click="unknownPrice.show = true"
          >
            <v-icon icon="mdi-help-box-outline" size="26" />
          </v-btn>
          <v-btn
            v-if="pageMode !== 'settlement' && productSearchEnabled"
            icon
            class="ms-1"
            variant="text"
            aria-label="Search products"
            @click="openProductSearch"
          >
            <v-icon icon="mdi-magnify" size="26" />
          </v-btn>
          <OnHoldMenu ref="onHoldMenu" hide-activator @restore="onRestoreHold" />
          <ReturnInvoiceMenu
            ref="returnInvoiceMenu"
            hide-activator
            :rounding-mode="roundingMode"
            @exchange="onExchange"
          />
          <UnknownPriceDialog v-model="unknownPrice.show" @add="onUnknownAdd" />
          <UnknownPicturesDialog
            v-if="unknownPictures.show"
            v-model="unknownPictures.show"
            :client-ref="unknownPicturesAtCheckout ? '' : unknownSession.clientRef"
            :units="unknownPictures.units"
            @cancel="answerUnknownPictures(false)"
            @ready="answerUnknownPictures(true)"
          />
          <ProductSearchDialog
            ref="productSearchDialog"
            v-if="pageMode !== 'settlement'"
            :show-activator="false"
            :code-only="productSearchCodeOnly"
            :hidden-fields="productSearchHiddenFields"
            @select="onProductSelect"
          />
        </div>
        <InvoiceItems
          ref="invoiceItems"
          :rounding-mode="roundingMode"
          :readonly="pageMode === 'settlement'"
          :custom-discounts-enabled="customItemDiscountsEnabled"
          :custom-prices-enabled="customItemPricesEnabled"
          :unknown-barcode="unknownProductBarcode"
          :price-edit-restricted="priceEditRestricted"
          :price-edit-codes="priceEditCodes"
          :per-unit-prices-enabled="perUnitItemPricesEnabled"
          :price-change-mode="priceChangeMode"
          :keyboard-mode="invoiceKeyboardModeEnabled"
          :deal-lines="quote?.lines ?? []"
          :discount-toggle-enabled="discountToggleEnabled && pageMode === 'sale'"
          :deal-toggle-enabled="dealToggleEnabled && pageMode === 'sale'"
          :discount-default-off="discountToggleDefaultOff"
          style="flex: 1 1 0; min-height: 0"
          @discount-error="showDiscountError"
        />
        <UnknownPicturesCorner
          v-if="showUnknownCorner"
          :client-ref="unknownSession.clientRef"
          :paused="unknownPictures.show"
          :units="unknownCartUnits"
          @ready="unknownPicturesReadyFor = $event"
          @session="unknownSession.clientRef = $event"
        />
        <!-- Taller than the search keyboard, so hidden behind it while
             searching in place, keeping its room so nothing moves. The
             wrapper takes no room of its own; the panel inherits it. -->
        <div class="home-quick-access" :class="{ 'home-under-keyboard': searchingInPlace }">
          <QuickAccessPanel
            v-if="pageMode !== 'settlement' && quickAccessRows > 0"
            @add="
              (item) => {
                playClickSound();
                addProductToCart(item);
              }
            "
          />
        </div>
      </div>
      <div class="home-right-pane d-flex flex-column position-relative" style="flex: 5 5 0; min-width: 0">
        <!-- Searching in place: the results lie over the calculator, down to the
             keyboard; everything under them stays as it was, a payment being
             keyed included, and is back once the search ends. -->
        <ProductSearchResults
          v-if="searchingInPlace"
          class="home-search-results"
          :search="inPlaceSearch"
          active
          :grid-mode="productGridMode"
          :code-only="productSearchCodeOnly"
          :hidden-fields="productSearchHiddenFields"
          @select="onInPlaceSelect"
        />
        <div
          v-if="!hasItems"
          class="position-absolute d-flex align-center justify-center"
          style="
            top: 0;
            left: 0;
            right: 0;
            bottom: 0;
            background: rgba(0, 0, 0, 0.1);
            backdrop-filter: blur(4px);
            z-index: 10;
          "
        >
          <v-card
            class="empty-cart-card pa-0 text-center d-flex flex-column align-center justify-center"
            rounded="lg"
            width="180"
            height="110"
            elevation="4"
          >
            <v-icon icon="mdi-barcode-scan" size="42" class="mb-2" />
            <div class="empty-cart-label text-subtitle-2">
              Scan a barcode<br />{{ scanByCodeEnabled ? "or type a code" : "to add a product" }}
            </div>
          </v-card>
        </div>
        <!--
        <div class="invoice-total-summary d-flex flex-column align-center mx-2 mt-1 px-1">
          <span class="invoice-total-summary__label text-medium-emphasis">Total</span>
          <div class="d-flex align-baseline justify-center ga-3">
            <span class="invoice-total-summary__usd">{{ calculator?.totalSummary?.first ?? "$0.00" }}</span>
            <span
              v-if="calculator?.totalSummary?.second"
              class="invoice-total-summary__lbp text-medium-emphasis"
            >
              ≈ {{ calculator.totalSummary.second }}
            </span>
          </div>
        </div>
        -->
        <div class="mb-3 mt-1" style="flex: 1 1 0; min-height: 0">
          <Calculator
            ref="calculator"
            :total-cents="quote?.grandTotalCents ?? 0"
            :unrounded-cents="quote?.unroundedCents"
            :invoice-discount-cents="quote?.invoiceDiscountCents ?? 0"
            :invoice-discount-name="quote?.invoiceDiscountName ?? ''"
            :exchange-shortfall-cents="quote?.exchangeShortfallCents ?? 0"
            :quote-error="quoteError"
            :items-json="cartItemsJson"
            :client-id="client.id"
            :rounding-mode="roundingMode"
            :rounding-currency="roundingCurrency"
            :split-primary-currency="splitPrimaryCurrency"
            :split-primary-first="splitPrimaryFirst"
            :split-primary-step="splitPrimaryStep"
            :has-items="hasItems"
            :exchange-mode="pageMode === 'exchange'"
            :exchange-original-invoice-id="exchangeState?.originalInvoiceId"
            :settlement-invoice-id="settlementState?.invoiceId"
            :hide-payment-method-currency="hidePaymentMethodCurrency"
            :auto-append-lbp-zeros="autoAppendLbpZeros"
            :overchange-confirmation-enabled="overchangeConfirmationEnabled"
            @account="accountDialog = true"
            @coupon="calculator?.openCouponDialog()"
          />
        </div>
        <ActionBar
          ref="actionBar"
          :total="invoiceTotal"
          :total-summary="calculator?.totalSummary ?? { first: '$0.00', second: '' }"
          :can-checkout="canCheckout"
          :has-items="hasItems"
          :mode="pageMode"
          @checkout="onCheckout"
          @hold="onHold"
          @void="onVoid"
          @cancelMode="cancelMode"
        />
      </div>
    </div>
    <!-- Over the bottom of the sale screen, not pushing it up: what's
         there (Check Out, the quick-access cards) waits behind it. -->
    <ProductSearchKeyboard
      v-if="searchingInPlace"
      ref="searchKeyboard"
      v-model="inPlaceSearch"
      v-model:grid-mode="productGridMode"
      class="home-search-keyboard"
      :code-only="productSearchCodeOnly"
      @close="searchingInPlace = false"
    />
  </div>

  <AccountDialog v-model="accountDialog" @select="onAccountClientSelect" />

  <v-overlay v-model="printing" class="align-center justify-center" persistent>
    <v-card class="pa-8 text-center" width="300">
      <template v-if="printPhase === 'success'">
        <div class="print-face" aria-hidden="true">:)</div>
        <div class="text-h6 mt-2">Done!</div>
      </template>
      <template v-else>
        <div v-if="printCashCents > 0" class="text-subtitle-1 mb-2">
          {{ formatUsd(printCashCents / 100) }} into cash
        </div>
        <div class="print-spinner mb-4" aria-hidden="true"></div>
        <div class="text-h6">Printing receipt...</div>
      </template>
    </v-card>
  </v-overlay>

  <v-snackbar
    v-model="snackbar.show"
    :class="[
      'app-snackbar',
      {
        'app-snackbar--error': snackbar.color === 'red-darken-4',
      },
    ]"
    :timeout="snackbar.icon ? 1500 : 3500"
    :location="snackbar.icon ? 'center' : 'top'"
    variant="flat"
  >
    <div class="d-flex align-center ga-3">
      <v-icon v-if="snackbar.icon" :icon="snackbar.icon" size="44" />
      <span :class="{ 'checkout-blocked-text': snackbar.icon }">{{ snackbar.text }}</span>
    </div>
  </v-snackbar>

  <!-- "warn" oversell mode: the branch would go below zero on these
       products. The cashier decides; kashi flags it either way. -->
  <v-dialog v-model="oversellDialog.show" max-width="480" persistent>
    <v-card>
      <v-card-title>Not enough stock</v-card-title>
      <v-card-text>
        <p class="mb-3">The branch doesn't have enough of:</p>
        <div v-for="line in oversellDialog.lines" :key="line.code" class="mb-1">
          <strong>{{ line.name || line.code }}</strong> — {{ Math.max(line.available, 0) }} in stock,
          {{ line.requested }} in this sale
        </div>
        <p class="mt-3 mb-0">Sell anyway? kashi will flag the stock to be corrected.</p>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn text="Cancel" @click="answerOversell(false)" />
        <v-btn color="primary" text="Sell anyway" variant="flat" @click="answerOversell(true)" />
      </v-card-actions>
    </v-card>
  </v-dialog>

  <!-- A typed price far from the list price is almost always a slip of the
       keypad ($3 keyed as $3000); asked once at checkout, not per keystroke. -->
  <v-dialog v-model="priceCheckDialog.show" max-width="480" persistent>
    <v-card>
      <v-card-title>Check these prices</v-card-title>
      <v-card-text>
        <div v-for="line in priceCheckDialog.lines" :key="line.code" class="mb-1">
          <strong>{{ line.name || line.code }}</strong> — selling at ${{ line.price.toFixed(2) }},
          list price ${{ line.listPrice.toFixed(2) }}
        </div>
        <p class="mt-3 mb-0">Is that right?</p>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn text="Fix prices" @click="answerPriceCheck(false)" />
        <v-btn color="primary" text="Prices are right" variant="flat" @click="answerPriceCheck(true)" />
      </v-card-actions>
    </v-card>
  </v-dialog>

  <!-- A scanned/typed product code shared by several variants: pick one. -->
  <v-dialog v-model="variantPicker.show" max-width="560" scrollable>
    <v-card>
      <v-card-title>Choose a variant of {{ variantPicker.code }}</v-card-title>
      <v-card-text class="pa-0">
        <v-list>
          <v-list-item
            v-for="variant in variantPicker.variants"
            :key="variant.barcode"
            link
            @click="pickVariant(variant)"
          >
            <v-list-item-title class="font-weight-medium">
              {{ [variant.color, variant.size].filter(Boolean).join(" · ") || variant.name }}
            </v-list-item-title>
            <v-list-item-subtitle>{{ variant.name }} · {{ variant.barcode }}</v-list-item-subtitle>
            <template #append>
              <span class="font-weight-medium">${{ Number(variant.price || 0).toFixed(2) }}</span>
            </template>
          </v-list-item>
        </v-list>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn text="Cancel" @click="variantPicker.show = false" />
      </v-card-actions>
    </v-card>
  </v-dialog>

  <!-- A product that asks a sale option: which one is being sold. -->
  <v-dialog v-model="saleOptionPicker.show" max-width="420">
    <v-card v-if="saleOptionPicker.product">
      <v-card-title class="text-h5 pb-0">{{ saleOptionPicker.product.name }}</v-card-title>
      <v-card-subtitle class="text-subtitle-1">
        {{ saleOptionPicker.product.code }}
        <template v-if="saleOptionPicker.product.color || saleOptionPicker.product.size">
          · {{ [saleOptionPicker.product.color, saleOptionPicker.product.size].filter(Boolean).join(" / ") }}
        </template>
      </v-card-subtitle>
      <v-card-text class="sale-option-grid pt-3 pb-1">
        <v-btn
          v-for="(option, index) in saleOptionPicker.product.saleOptions"
          :key="option.name"
          :autofocus="index === 0"
          class="sale-option-tile"
          :color="saleOptionLook(option.name).color"
          height="124"
          variant="flat"
          @click="pickSaleOption(option)"
        >
          <div class="d-flex flex-column align-center ga-1">
            <v-icon :icon="saleOptionLook(option.name).icon" size="32" />
            <span class="sale-option-name">{{ option.name }}</span>
            <span v-if="option.discount > 0" class="sale-option-discount">-{{ option.discount }}%</span>
          </div>
        </v-btn>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn size="large" text="Cancel" @click="saleOptionPicker.show = false" />
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script lang="ts" setup>
import { computed, nextTick, onBeforeUnmount, onMounted, provide, reactive, ref, watch } from "vue";
import { saleOptionLook } from "@/lib/saleOptions";
import {
  FindProductsForScan,
  HoldInvoice,
  VoidInvoice,
  CheckoutInvoice,
  CheckoutExchangeInvoice,
  CheckOversell,
  SettleInvoicePayments,
  GetNextInvoiceCode,
  GetAppSettings,
  DeleteHoldInvoice,
  GetOpenShift,
  OpenShift,
  WriteToScreen,
  QuoteInvoiceTotal,
  QuoteExchangeTotal,
  GetUnknownProduct,
  RefreshCartPricing,
} from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import ActionBar from "@/components/ActionBar.vue";
import AccountDialog from "@/components/AccountDialog.vue";
import Calculator from "@/components/Calculator.vue";
import ClientButton from "@/components/ClientButton.vue";
import InvoiceItems from "@/components/InvoiceItems.vue";
import type { DealLine } from "@/lib/deals";
import InlineKeyboard from "@/components/InlineKeyboard.vue";
import NoteButton from "@/components/NoteButton.vue";
import OnHoldMenu from "@/components/OnHoldMenu.vue";
import ProductSearchDialog from "@/components/ProductSearchDialog.vue";
import ProductSearchKeyboard from "@/components/ProductSearchKeyboard.vue";
import ProductSearchResults from "@/components/ProductSearchResults.vue";
import { useProductGridMode } from "@/composables/useProductGridMode";
import UnknownPicturesDialog from "@/components/UnknownPicturesDialog.vue";
import UnknownPicturesCorner from "@/components/UnknownPicturesCorner.vue";
import UnknownPriceDialog from "@/components/UnknownPriceDialog.vue";
import QuickAccessPanel from "@/components/QuickAccessPanel.vue";
import { productSearchInPlace, quickAccessRows } from "@/lib/display";
import ReturnInvoiceMenu from "@/components/ReturnInvoiceMenu.vue";
import SalespersonsDialog from "@/components/SalespersonsDialog.vue";
import type { ExchangeSelection } from "@/lib/exchange";
import { formatUsd } from "@/lib/format";
import { notifyIncomeChanged } from "@/lib/incomeEvents";
import { normalizeRoundingMode, type RoundingMode } from "@/lib/money";
import { playClickSound, playErrorSound, playSuccessSound, playWarningSound } from "@/lib/sound";
import type { Client } from "@/lib/client";
import { useClient } from "@/composables/useClient";
import { useSalesperson } from "@/composables/useSalesperson";
import {
  pendingExchangeSelection,
  pendingHoldInvoice,
  pendingSettlementInvoice,
} from "@/composables/useNavActions";
const INVOICE_DRAFT_KEY = "kashi-pos.invoice-draft";
type InvoiceDraft = {
  items: any[];
  client: Client;
  notes?: string;
  calculator?: {
    currentInput?: string;
    activeMethod?: string;
    entries?: any[];
  };
  // Set only while an exchange is in progress, so an unfinished exchange
  // (returned lines locked in, replacements maybe scanned) survives an app
  // restart or logout/login instead of silently reverting to a normal sale.
  exchangeOriginalInvoiceId?: number;
};
const { client, clientSelected, setClient, resetClient, pulseMissingClient, setClientRequired } = useClient();
const { selectedSalesperson, pulseSalesperson, salespersonPerInvoiceRequired, setSalespersonPerInvoiceRequired } =
  useSalesperson();
let pageActive = true;

const taxRate = ref(0);
const roundingMode = ref<RoundingMode>("half_up");
const roundingCurrency = ref("");
// The split total's first half; empty leaves the Calculator on its USD
// fallback. splitPrimaryFirst controls which half is shown first.
const splitPrimaryCurrency = ref("");
const splitPrimaryFirst = ref(true);
// 0 = fall back to the primary currency's own cash_rounding_unit.
const splitPrimaryStep = ref(0);
const receiptEnabled = ref(false);
const receiptPrinter = ref("");
const customItemDiscountsEnabled = ref(false);
const customItemPricesEnabled = ref(false);
const perUnitItemPricesEnabled = ref(false);
const priceChangeManualOverrideEnabled = ref(false);
const invoiceKeyboardModeEnabled = ref(false);
const hidePaymentMethodCurrency = ref(false);
const autoAppendLbpZeros = ref(false);
const overchangeConfirmationEnabled = ref(true);
const alwaysAnonymousClientEnabled = ref(false);
// Gates the product-search button in the toolbar below. Toggleable from the
// local Settings page and remotely from kashi; a "settings_changed" event
// (see loadAppSettings) reflects either change here without a reload.
const productSearchEnabled = ref(true);
// Whether the scan field also accepts product codes (see FindProductsForScan);
// only changes the hint text here, the backend enforces it.
const scanByCodeEnabled = ref(true);
const productSearchCodeOnly = ref(false);
const productSearchHiddenFields = ref<string[]>([]);
const priceEditRestricted = ref(false);
// A salesperson can tap a line's discount chip to turn it off/on (kashi's
// discount_toggle), and new lines can start with it off; deal_toggle does
// the same for a line's deal chip.
const discountToggleEnabled = ref(false);
const discountToggleDefaultOff = ref(false);
const dealToggleEnabled = ref(false);
const priceEditCodes = ref<string[]>([]);
const priceChangeMode = computed(() =>
  priceChangeManualOverrideEnabled.value ? "override" : "discount",
);
// Manual price/discount edits at checkout are gated entirely by the custom
// pricing feature flags above (toggled from Settings / remotely from kashi).
// When an admin turns them on they apply to every operator, cashiers included.
const branchName = ref("Store");
const screenPort = ref("");
const invoiceCode = ref("Loading invoice code…");
const invoiceNotes = ref("");
const noteDialog = ref(false);
const noteDraft = ref("");
const exchangeState = ref<{ originalInvoiceId: number } | null>(null);
const settlementState = ref<{
  invoiceId: number;
  invoiceCode: string;
  grandTotalCents: number;
} | null>(null);
// The single source of truth for which of the three home-screen modes is
// active - everything else (locking items/client/salesperson, which
// controls are visible, the checkout button's label/color) branches off
// this instead of checking exchangeState/settlementState directly.
const pageMode = computed<"sale" | "exchange" | "settlement">(() =>
  settlementState.value ? "settlement" : exchangeState.value ? "exchange" : "sale",
);
const pageModeTitle = computed(() =>
  pageMode.value === "settlement"
    ? "Change Settlement"
    : pageMode.value === "exchange"
      ? "Exchange"
      : "Invoice",
);
const invoiceItems = ref<InstanceType<typeof InvoiceItems> | null>(null);
const calculator = ref<InstanceType<typeof Calculator> | null>(null);
const actionBar = ref<InstanceType<typeof ActionBar> | null>(null);
const productSearchDialog = ref<InstanceType<typeof ProductSearchDialog> | null>(null);
const onHoldMenu = ref<InstanceType<typeof OnHoldMenu> | null>(null);
const returnInvoiceMenu = ref<InstanceType<typeof ReturnInvoiceMenu> | null>(null);
const accountDialog = ref(false);

// The backend quote is the single source of truth for the cart total - see
// refreshQuote()/getFullItemsJSON() below. invoiceTotal/cartItemsJson are
// kept in sync by the same debounced watcher so the calculator's total and
// payment-status checks are always priced from the exact same cart JSON.
interface InvoiceQuote {
  grandTotalCents: number;
  // The total before cash rounding (discounted subtotal plus tax), when the
  // backend says; the calculator shows "Rounded from" when it differs.
  unroundedCents?: number;
  // Only set in exchange mode: how much more replacement value is still
  // needed before the exchange can be checked out. Exchanges can never pay
  // cash back to the customer, so while returned value exceeds replacement
  // value the backend floors the total at 0 and checkout stays blocked -
  // this is what lets the UI explain why instead of just showing $0.00.
  exchangeShortfallCents?: number;
  // The priced lines, for the deals they carry (free units, the saving):
  // InvoiceItems marks them on the cart lines.
  lines?: DealLine[];
  // kashi's whole-sale discount, already off grandTotalCents.
  invoiceDiscountCents?: number;
  invoiceDiscountName?: string;
}
const quote = ref<InvoiceQuote | null>(null);
// A failed quote must never render as $0.00 - with a zero total the
// calculator computes diffCents === 0 and reports "Paid", so a pricing
// error would look like a fully-settled zero-value sale. Whenever this is
// set the calculator shows the reason instead of a total and refuses
// checkout.
const quoteError = ref<string | null>(null);
const cartItemsJson = ref("");
const invoiceTotal = computed(() => (quote.value?.grandTotalCents ?? 0) / 100);
const hasItems = computed(() => (invoiceItems.value?.items?.length ?? 0) > 0);
const salespersonSelected = computed(() => Boolean(selectedSalesperson.value));
const missingAssignmentFlash = ref(false);
let missingAssignmentFlashTimer: ReturnType<typeof setTimeout> | undefined;
const canCheckout = computed(() => {
  // No priced cart, no checkout - settlement/exchange skip the
  // invoiceTotal > 0 test below, so this has to be its own gate.
  if (quoteError.value) return false;
  if (!calculator.value?.checkoutReady) return false;
  // Belt-and-suspenders: an exchange must never hand cash back to the
  // customer, so checkout stays blocked while the returned value hasn't
  // been fully covered by replacement items yet.
  if ((quote.value?.exchangeShortfallCents ?? 0) > 0) return false;
  // Settlement mode's client/salesperson/items are pre-set and frozen from
  // the loaded invoice - only the payment balance gates checkout.
  if (pageMode.value === "settlement") return true;
  return (
    (pageMode.value === "exchange" ? hasItems.value : invoiceTotal.value > 0) &&
    clientSelected.value &&
    salespersonSelected.value
  );
});

const barcodeValue = ref("");
const barcodeInput = ref<InstanceType<typeof import("vuetify/components").VTextField> | null>(null);
const noteInput = ref<InstanceType<typeof import("vuetify/components").VTextarea> | null>(null);
const dialogOpen = ref(false);
let overlayObserver: MutationObserver | undefined;
let overlayFocusTimer: ReturnType<typeof setTimeout> | undefined;
let unsubscribeSettingsChanged: (() => void) | undefined;
let unsubscribeListChanges: Array<() => void> = [];
let listRefreshTimer: ReturnType<typeof setTimeout> | null = null;

// kashi changed a price or discount list: lines already in the cart follow it
// (see InvoiceItems.applyListPricing) and the cashier is told, so a total never
// moves silently. Debounced, since one edit can touch several synced tables.
function scheduleCartListRefresh() {
  if (listRefreshTimer) clearTimeout(listRefreshTimer);
  listRefreshTimer = setTimeout(async () => {
    listRefreshTimer = null;
    const barcodes = (invoiceItems.value?.items ?? [])
      .filter((item: any) => item.barcode && !isUnknownItem(item) && item.qty > 0 && !item.locked)
      .map((item: any) => item.barcode);
    if (!barcodes.length || pageMode.value === "settlement") return;
    try {
      const rows = (await RefreshCartPricing(JSON.stringify(barcodes))) ?? [];
      const changed: string[] = invoiceItems.value?.applyListPricing(rows) ?? [];
      if (changed.length) {
        snackbar.text = `Price updated: ${changed.join(", ")}`;
        snackbar.color = "info";
        snackbar.icon = "";
        snackbar.show = true;
        await refreshQuote();
      }
    } catch {
      // The lines keep what they had; the next change tries again.
    }
  }, 400);
}
let restoringDraft = true;

provide("dialogOpen", dialogOpen);

watch(hasItems, (val) => {
  if (!val) calculator.value?.clearAll();
});

watch(dialogOpen, (v) => {
  if (!v) focusBarcodeWhenIdle();
});

watch(noteDialog, (open) => {
  if (!open) return;
  noteDraft.value = invoiceNotes.value;
  void nextTick(() => noteInput.value?.focus());
});

watch(pendingHoldInvoice, () => consumePendingHold());
watch(pendingExchangeSelection, () => void consumePendingExchange());
watch(pendingSettlementInvoice, () => void consumePendingSettlement());

watch(
  [
    () => invoiceItems.value?.items,
    () => client,
    invoiceNotes,
    exchangeState,
    () => calculator.value?.holdState(),
  ],
  ([items, client]) => {
    // Settlement mode's cart is a frozen snapshot of one specific invoice,
    // not a draft sale - it must never be persisted/restored as one.
    if (restoringDraft || !client || settlementState.value) return;
    if (!items?.length && client.id === 0 && !exchangeState.value) {
      localStorage.removeItem(INVOICE_DRAFT_KEY);
      return;
    }
    localStorage.setItem(
      INVOICE_DRAFT_KEY,
      JSON.stringify({
        items: items ?? [],
        client,
        notes: invoiceNotes.value,
        calculator: calculator.value?.holdState(),
        exchangeOriginalInvoiceId: exchangeState.value?.originalInvoiceId,
      } satisfies InvoiceDraft),
    );
  },
  { deep: true },
);

let quoteRequestToken = 0;
let quoteDebounceTimer: ReturnType<typeof setTimeout> | undefined;

async function refreshQuote() {
  const token = ++quoteRequestToken;
  const itemsJson = await getFullItemsJSON();
  if (token !== quoteRequestToken) return; // a newer request already superseded this one
  cartItemsJson.value = itemsJson;
  quoteError.value = null;
  if (settlementState.value) {
    // Settlement mode never re-prices - the total is the invoice's frozen
    // GrandTotal, captured once when it was loaded. Re-running it through
    // QuoteInvoiceTotal could drift from that total if pricing/tax/rounding
    // settings changed since the original sale.
    quote.value = { grandTotalCents: settlementState.value.grandTotalCents };
    return;
  }
  if (itemsJson === "[]") {
    quote.value = null;
    return;
  }
  if (exchangeState.value) {
    // A cart with negative-qty returned lines errors out of
    // QuoteInvoiceTotal outright (it only ever accepts qty > 0), so exchange
    // mode prices through the dedicated exchange quote instead - the same
    // pricing checkoutReturnExchangeInvoice will actually apply.
    try {
      const result = await QuoteExchangeTotal(itemsJson, exchangeState.value.originalInvoiceId);
      if (token !== quoteRequestToken) return;
      quote.value = {
        grandTotalCents: Number(result.grandTotalCents),
        exchangeShortfallCents: result.needsMoreReplacementValue
          ? Number(result.shortfallCents)
          : 0,
      };
    } catch (e) {
      if (token !== quoteRequestToken) return;
      console.error("QuoteExchangeTotal failed:", e);
      quote.value = null;
      quoteError.value = quoteErrorMessage(e);
    }
    return;
  }
  try {
    const result = await QuoteInvoiceTotal(itemsJson);
    if (token !== quoteRequestToken) return;
    quote.value = {
      grandTotalCents: Number(result.grandTotalCents),
      unroundedCents: Number(result.discountedSubtotalCents) + Number(result.taxCents),
      lines: result.lines ?? [],
      invoiceDiscountCents: Number(result.invoiceDiscountCents ?? 0),
      invoiceDiscountName: result.invoiceDiscountName ?? "",
    };
  } catch (e) {
    if (token !== quoteRequestToken) return;
    // Surfaced on screen as well as in the console - the reason is either a
    // real pricing error (an unset sales inventory, a zero-priced product, a
    // broken rounding currency) or QuoteInvoiceTotal not existing on
    // window.go yet because the app needs a rebuild/restart.
    console.error("QuoteInvoiceTotal failed:", e);
    quote.value = null;
    quoteError.value = quoteErrorMessage(e);
  }
}

// Wails rejects with a plain string for a Go error, but a real Error for a
// missing binding or a transport failure - normalise both into something a
// cashier can read out over the phone.
function quoteErrorMessage(e: unknown): string {
  const message = typeof e === "string" ? e : e instanceof Error ? e.message : String(e);
  return message.trim() || "pricing failed";
}

watch(
  [
    () => invoiceItems.value?.items,
    roundingMode,
    roundingCurrency,
    taxRate,
    customItemPricesEnabled,
    perUnitItemPricesEnabled,
    customItemDiscountsEnabled,
  ],
  () => {
    // No wait: a quote reuses the cached prices (pricing_overrides.go) and
    // takes well under a millisecond, so the total follows each scan or key
    // at once. The 0ms timer only folds changes made in the same tick into
    // one request; quoteRequestToken drops any reply a newer one superseded.
    clearTimeout(quoteDebounceTimer);
    quoteDebounceTimer = setTimeout(refreshQuote, 0);
  },
  { deep: true, immediate: true },
);

// An exchange whose returned items have all been taken out of the cart has
// nothing left to exchange: it goes back to a normal sale, keeping any
// replacement items already scanned.
watch(
  () => (invoiceItems.value?.items ?? []).some((item) => item.qty < 0),
  (hasReturns) => {
    if (hasReturns || !exchangeState.value || restoringDraft) return;
    exchangeState.value = null;
    void refreshNextInvoiceCode("sales");
  },
);

async function restoreDraft() {
  try {
    const draft = JSON.parse(
      localStorage.getItem(INVOICE_DRAFT_KEY) || "null",
    ) as InvoiceDraft | null;
    if (!draft) return;
    // Assigned directly (bypassing addItem's scan-one-unit-at-a-time merge,
    // which clamps qty to a minimum of 1) so a locked, negative-qty returned
    // line from an in-progress exchange restores exactly as it was, instead
    // of being corrupted into a positive qty-1 line.
    if (invoiceItems.value) {
      invoiceItems.value.items = (draft.items ?? []).map((item) => ({ ...item }));
    }
    if (draft.client?.id) setClient(draft.client);
    invoiceNotes.value = draft.notes ?? "";
    noteDraft.value = invoiceNotes.value;
    calculator.value?.restoreHoldState(draft.calculator);
    if (draft.exchangeOriginalInvoiceId) {
      exchangeState.value = { originalInvoiceId: draft.exchangeOriginalInvoiceId };
      await refreshNextInvoiceCode("exchange");
    }
  } catch {
    localStorage.removeItem(INVOICE_DRAFT_KEY);
  } finally {
    restoringDraft = false;
  }
}

function clearDraft() {
  localStorage.removeItem(INVOICE_DRAFT_KEY);
}

function clearInvoice() {
  invoiceItems.value?.clearItems();
  calculator.value?.clearAll();
  // Keep the salesperson selected for the next invoice, unless the branch
  // requires picking one per invoice - then clear it so the cashier must
  // choose again. Restoring an on-hold invoice still replaces this below with
  // the salesperson saved on that hold.
  if (salespersonPerInvoiceRequired.value) {
    selectedSalesperson.value = null;
  }
  invoiceNotes.value = "";
  noteDraft.value = "";
  noteDialog.value = false;
  unknownPicturesReadyFor = "";
  unknownSession.clientRef = "";
  clearDraft();
  resetClient();
}

function saveNote() {
  invoiceNotes.value = noteDraft.value.trim();
  noteDialog.value = false;
  focusBarcode();
}

function removeNote() {
  invoiceNotes.value = "";
  noteDraft.value = "";
  noteDialog.value = false;
  focusBarcode();
}

function openNoteDialog() {
  noteDraft.value = invoiceNotes.value;
  noteDialog.value = true;
}

function focusBarcode() {
  barcodeInput.value?.focus();
}

function hasBlockingUi() {
  return (
    dialogOpen.value ||
    Boolean(
      document.querySelector(
        ".v-overlay--active, .v-menu__content, [role='dialog'], [role='menu']",
      ),
    )
  );
}

// True while a keyboard-mode qty/discount/price field is focused - typing
// into one mutates the DOM (the live value re-renders), which would
// otherwise trigger the MutationObserver below and steal focus back to the
// barcode field mid-keystroke.
function keyboardFieldFocused() {
  const active = document.activeElement;
  return active instanceof HTMLElement && Boolean(active.closest(".invoice-items"));
}

function focusBarcodeWhenIdle(attempt = 0) {
  if (hasBlockingUi() || keyboardFieldFocused()) {
    if (attempt < 20) requestAnimationFrame(() => focusBarcodeWhenIdle(attempt + 1));
    return;
  }
  focusBarcode();
}

function scheduleBarcodeFocus() {
  clearTimeout(overlayFocusTimer);
  overlayFocusTimer = setTimeout(() => focusBarcodeWhenIdle(), 50);
}

function onBlur() {
  setTimeout(() => {
    if (hasBlockingUi()) return;
    const active = document.activeElement;
    if (
      active instanceof HTMLElement &&
      (active.matches("input, textarea, select, [contenteditable='true']") ||
        Boolean(
          active.closest(
            "[role='dialog'], [role='menu'], .v-overlay__content, .invoice-items",
          ),
        ))
    ) {
      return;
    }
    focusBarcode();
  });
}

// The search button: in place (results where the calculator is, keyboard
// along the bottom) unless Settings turned that off for the dialog.
const searchingInPlace = ref(false);
const inPlaceSearch = ref("");
const productGridMode = useProductGridMode(
  computed(() => productSearchCodeOnly.value || productSearchHiddenFields.value.includes("name")),
);

function openProductSearch() {
  if (productSearchInPlace.value) searchingInPlace.value = true;
  else productSearchDialog.value?.openDialog();
}

// Picking a product ends the search; all of it goes and the sale is back.
function onInPlaceSelect(product: Parameters<typeof onProductSelect>[0]) {
  searchingInPlace.value = false;
  onProductSelect(product);
}

// The results end where the keyboard starts.
const searchKeyboard = ref<{ $el?: HTMLElement } | null>(null);
const searchKeyboardHeight = ref(0);
const keyboardResizer = new ResizeObserver(([entry]) => {
  searchKeyboardHeight.value = entry?.borderBoxSize?.[0]?.blockSize ?? entry?.contentRect.height ?? 0;
});
watch(searchKeyboard, (keyboard, old) => {
  if (old?.$el) keyboardResizer.unobserve(old.$el);
  if (keyboard?.$el) keyboardResizer.observe(keyboard.$el);
});
onBeforeUnmount(() => keyboardResizer.disconnect());

watch(searchingInPlace, (open) => {
  // Keeps the barcode field from taking the focus back while searching.
  dialogOpen.value = open;
  if (!open) inPlaceSearch.value = "";
});

watch(pageMode, (mode) => {
  if (mode === "settlement") searchingInPlace.value = false;
});

function onProductSelect(product: {
  name: string;
  code: string;
  category: string;
  price: number;
  discount: number;
  qty: number;
}) {
  playClickSound();
  if (addProductToCart(product)) focusBarcode();
}

function onAccountClientSelect(client: Client) {
  setClient(client);
  nextTick(() => calculator.value?.addOnAccount());
}

const barcodeFeedback = ref<"success" | "error" | null>(null);
let barcodeFeedbackTimer: ReturnType<typeof setTimeout> | null = null;

function flashBarcodeFeedback(kind: "success" | "error") {
  if (barcodeFeedbackTimer) clearTimeout(barcodeFeedbackTimer);
  barcodeFeedback.value = kind;
  barcodeFeedbackTimer = setTimeout(
    () => {
      barcodeFeedback.value = null;
      barcodeFeedbackTimer = null;
    },
    kind === "success" ? 450 : 1400,
  );
}

// The backend tries the value as a barcode first, then (when the branch
// allows it) as a product code. A code shared by several variants comes
// back as all of them, and the cashier picks one in variantPicker.
async function onBarcodeEnter() {
  const value = barcodeValue.value;
  if (!value) return;
  barcodeValue.value = "";
  let matches: any[] = [];
  try {
    matches = (await FindProductsForScan(value)) ?? [];
  } catch {}
  flashBarcodeFeedback(matches.length ? "success" : "error");
  if (matches.length === 1) {
    if (!addProductToCart({ ...matches[0], qty: 1 })) return;
  } else if (matches.length > 1) {
    variantPicker.code = value.trim();
    variantPicker.variants = matches;
    variantPicker.show = true;
    return;
  }
  focusBarcode();
}

// A product that asks a sale option (Summer/Winter, set in kashi) can't go
// into the cart until the cashier says which one is being sold: the same
// barcode covers both, and they're discounted differently. Every way of
// adding a product comes through here. Returns false when it opened the
// picker instead, so the caller leaves focus to the dialog. A product with a
// single option (a jacket that is only ever Winter) has nothing to ask: it
// goes in as that option, with that option's discount.
const saleOptionPicker = reactive<{ show: boolean; product: any }>({ show: false, product: null });

type SaleOption = { name: string; discount: number; discountListName?: string };

function addProductToCart(product: any): boolean {
  if (product?.saleOptions?.length === 1 && !product.saleOption) {
    addAsSaleOption(product, product.saleOptions[0]);
    return true;
  }
  if (product?.saleOptions?.length && !product.saleOption) {
    saleOptionPicker.product = product;
    saleOptionPicker.show = true;
    return false;
  }
  invoiceItems.value?.addItem(product);
  return true;
}

function addAsSaleOption(product: any, option: SaleOption) {
  invoiceItems.value?.addItem({
    ...product,
    saleOption: option.name,
    discount: Number(option.discount || 0),
    originalDiscount: Number(option.discount || 0),
    discountListName: option.discountListName ?? "",
  });
}

function pickSaleOption(option: SaleOption) {
  const product = saleOptionPicker.product;
  saleOptionPicker.show = false;
  saleOptionPicker.product = null;
  if (!product) return;
  addAsSaleOption(product, option);
  scheduleBarcodeFocus();
}

const variantPicker = reactive<{ show: boolean; code: string; variants: any[] }>({
  show: false,
  code: "",
  variants: [],
});

function pickVariant(variant: any) {
  variantPicker.show = false;
  if (addProductToCart({ ...variant, qty: 1 })) scheduleBarcodeFocus();
}

function showDiscountError(message: string) {
  snackbar.text = message || "Could not save discount.";
  snackbar.color = "red-darken-4";
  snackbar.icon = "";
  snackbar.show = true;
}

const printing = ref(false);
const printPhase = ref<"printing" | "success">("printing");
const printCashCents = ref(0);
const snackbar = reactive({ show: false, text: "", color: "success", icon: "" });

let screenQueue = Promise.resolve();
function queueScreenUpdate(message: string) {
  const port = screenPort.value.trim();
  if (!port) return;
  screenQueue = screenQueue
    .then(() => WriteToScreen(port, message))
    .catch((error) => console.warn("Customer screen update failed:", error));
}

function showCustomerTotal() {
  if (!hasItems.value) {
    queueScreenUpdate("");
    return;
  }
  queueScreenUpdate(invoiceTotal.value.toFixed(2));
}

watch([invoiceTotal, hasItems], showCustomerTotal);

async function getItemsJSON(): Promise<string> {
  const items = invoiceItems.value?.items ?? [];
  return JSON.stringify(
    items.map((i: any) => ({ code: i.code, barcode: i.barcode, qty: i.qty, saleOption: i.saleOption || undefined })),
  );
}

async function getFullItemsJSON(): Promise<string> {
  const items = invoiceItems.value?.items ?? [];
  return JSON.stringify(
    items.map((i: any) => ({
      name: i.name,
      code: i.code,
      // Colour/size variants of one model share a code; the barcode is what
      // tells them apart, here and in the backend's duplicate-line check.
      barcode: i.barcode || undefined,
      category: i.category,
      price: i.price,
      customPriceCents:
        customItemPricesEnabled.value && !isUnknownItem(i) ? Math.round(Number(i.price) * 100) : undefined,
      unitPricesCents:
        (perUnitItemPricesEnabled.value || customItemPricesEnabled.value || isUnknownItem(i)) && i.qty > 0
          ? (i.unitPrices ?? Array(Math.trunc(i.qty)).fill(i.price)).map((price: number) =>
              Math.round(Number(price) * 100),
            )
          : undefined,
      unitDiscounts:
        (perUnitItemPricesEnabled.value || customItemDiscountsEnabled.value || isUnknownItem(i)) && i.qty > 0
          ? (i.unitDiscounts ?? Array(Math.trunc(i.qty)).fill(i.discount)).map((discount: number) =>
              Math.trunc(Number(discount)),
            )
          : undefined,
      unitPriceCents: i.qty < 0 ? Math.round(Number(i.soldUnitPrice ?? i.price) * 100) : undefined,
      unitDiscount: i.qty < 0 ? Math.trunc(Number(i.soldUnitDiscount ?? i.discount)) : undefined,
      discount: i.discount,
      salespersonId: Number(i.salespersonId || 0),
      note: String(i.note || "").trim(),
      saleOption: i.saleOption || undefined,
      noDiscount: discountToggleEnabled.value && i.discountOff && i.qty > 0 ? true : undefined,
      noDeal: dealToggleEnabled.value && i.dealOff && i.qty > 0 ? true : undefined,
      qty: i.qty,
    })),
  );
}

async function onHold() {
  if (invoiceItems.value && !(await invoiceItems.value.flushChanges())) return;
  const items = invoiceItems.value?.items ?? [];
  if (!items.length) return;
  await HoldInvoice(
    JSON.stringify({
      version: 1,
      items,
      client,
      salesperson: selectedSalesperson.value,
      calculator: calculator.value?.holdState(),
      notes: invoiceNotes.value,
    }),
  );
  clearInvoice();
}

async function onVoid() {
  const json = await getFullItemsJSON();
  if (json === "[]") return;
  await VoidInvoice(json, "");
  clearInvoice();
}

async function onSettlementCheckout() {
  if (!settlementState.value) return;
  printing.value = false;
  try {
    const paymentsJSON = calculator.value?.paymentsJSON ?? "[]";
    const result = await SettleInvoicePayments(settlementState.value.invoiceId, paymentsJSON);
    snackbar.text = `Settlement updated — ${result?.invoiceCode ?? settlementState.value.invoiceCode}`;
    snackbar.color = "success";
    snackbar.icon = "";
    snackbar.show = true;
    playSuccessSound();
    clearInvoice();
    settlementState.value = null;
    notifyIncomeChanged();
    await refreshNextInvoiceCode("sales");
  } catch (e: any) {
    snackbar.text = e?.toString() || "Could not update settlement";
    snackbar.color = "red-darken-4";
    snackbar.icon = "";
    snackbar.show = true;
    playErrorSound();
  }
}

type OversellLine = { code: string; name: string; requested: number; available: number };
const oversellDialog = reactive<{ show: boolean; lines: OversellLine[] }>({ show: false, lines: [] });
let oversellAnswer: ((sell: boolean) => void) | null = null;

function answerOversell(sell: boolean) {
  oversellDialog.show = false;
  oversellAnswer?.(sell);
  oversellAnswer = null;
}

// Only "warn" asks here; "block" is refused by checkout itself. A failed
// check never stops a sale.
async function confirmOversellIfNeeded(itemsJSON: string): Promise<boolean> {
  let check;
  try {
    check = await CheckOversell(itemsJSON);
  } catch {
    return true;
  }
  if (check?.mode !== "warn" || !check.lines?.length) return true;
  oversellDialog.lines = check.lines;
  oversellDialog.show = true;
  return new Promise<boolean>((resolve) => {
    oversellAnswer = resolve;
  });
}

// A manual price more than PRICE_CHECK_FACTOR times above or below the list
// price is asked about before the sale exists, so a mistyped price never
// needs a cancel afterwards.
const PRICE_CHECK_FACTOR = 3;
type PriceCheckLine = { code: string; name: string; price: number; listPrice: number };
const priceCheckDialog = reactive<{ show: boolean; lines: PriceCheckLine[] }>({ show: false, lines: [] });
let priceCheckAnswer: ((ok: boolean) => void) | null = null;

function answerPriceCheck(ok: boolean) {
  priceCheckDialog.show = false;
  priceCheckAnswer?.(ok);
  priceCheckAnswer = null;
}

async function confirmPricesIfUnusual(): Promise<boolean> {
  const lines: PriceCheckLine[] = [];
  for (const item of invoiceItems.value?.items ?? []) {
    const listPrice = Number(item.originalPrice ?? 0);
    if (item.qty <= 0 || !(listPrice > 0)) continue;
    const prices: number[] = item.unitPrices?.length ? item.unitPrices : [item.price];
    const odd = prices.map(Number).find((p) => p > listPrice * PRICE_CHECK_FACTOR || p < listPrice / PRICE_CHECK_FACTOR);
    if (odd !== undefined) lines.push({ code: item.code, name: item.name, price: odd, listPrice });
  }
  if (!lines.length) return true;
  priceCheckDialog.lines = lines;
  priceCheckDialog.show = true;
  return new Promise<boolean>((resolve) => {
    priceCheckAnswer = resolve;
  });
}

// --- "Unknown item": something with no code, rung up at a typed price, with a
// picture of every unit taken on a phone before the customer pays. ---
// The product kashi named for unknown items (or the built-in placeholder).
const unknownProductBarcode = ref("UNKNOWN");
const isUnknownItem = (item: any) => item?.barcode === unknownProductBarcode.value;
const unknownPrice = reactive({ show: false });
type UnknownUnit = { label: string; unitPriceCents: number; discount: number; lineRef: string; slotRef: string };
const unknownPictures = reactive<{ show: boolean; units: UnknownUnit[] }>({ show: false, units: [] });
let unknownPicturesAnswer: ((ready: boolean) => void) | null = null;
// The cart (one unit per picture) the last finished session was opened for.
let unknownPicturesReadyFor = "";
// Off (the default): the QR code shows on the sale screen as soon as an
// unknown item is in the cart, so the phone takes pictures while the sale is
// still being rung up. On: only once the cashier pays.
const unknownPicturesAtCheckout = ref(false);
// The session the sale screen's QR code opened for this cart.
const unknownSession = reactive({ clientRef: "" });
const unknownCartUnits = computed(() => unknownUnits());
const showUnknownCorner = computed(
  () => !unknownPicturesAtCheckout.value && pageMode.value !== "settlement" && unknownCartUnits.value.length > 0,
);

async function onUnknownAdd(price: number) {
  try {
    const product = await GetUnknownProduct();
    // unknownRef names the line to kashi, so its picture stays with it as
    // other lines come and go while the phone is open.
    const unknownRef = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 8)}`;
    invoiceItems.value?.addItem({ ...product, price, qty: 1, originalPrice: price, unknownRef } as any);
    playClickSound();
  } catch (e: any) {
    showDiscountError(e?.toString() || "Couldn't add an unknown item");
  }
  focusBarcode();
}

// One unit per piece sold, tagged with its cart line: kashi takes one picture
// per line, whatever its qty.
function unknownUnits() {
  const units: UnknownUnit[] = [];
  let line = 0;
  for (const item of invoiceItems.value?.items ?? []) {
    if (!isUnknownItem(item) || !(item.qty > 0)) continue;
    line++;
    // A line from before unknownRef (an old draft or hold) goes by position.
    const lineRef = String(item.unknownRef || line);
    for (let i = 0; i < item.qty; i++) {
      const price = Number(item.unitPrices?.[i] ?? item.price);
      units.push({
        label: `Unknown item $${price.toFixed(2)}`,
        unitPriceCents: Math.round(price * 100),
        discount: Math.trunc(Number(item.unitDiscounts?.[i] ?? item.discount ?? 0)),
        lineRef,
        slotRef: `${lineRef}:${i + 1}`,
      });
    }
  }
  return units;
}

// Resolves true once every unit of the cart has a picture (or a manager's
// pass); false if the cashier backs out. Skipped when nothing is unknown, or
// when this exact cart already finished a session.
async function ensureUnknownPictures(): Promise<boolean> {
  const units = unknownUnits();
  if (!units.length) return true;
  const signature = JSON.stringify(units);
  if (signature === unknownPicturesReadyFor) return true;
  unknownPicturesReadyFor = "";
  unknownPictures.units = units;
  unknownPictures.show = true;
  const ready = await new Promise<boolean>((resolve) => {
    unknownPicturesAnswer = resolve;
  });
  if (ready) unknownPicturesReadyFor = signature;
  return ready;
}

function answerUnknownPictures(ready: boolean) {
  const answer = unknownPicturesAnswer;
  unknownPicturesAnswer = null;
  answer?.(ready);
}

function notifyCheckoutBlocked(text: string, icon: string) {
  playWarningSound();
  snackbar.text = text;
  snackbar.color = "warning";
  snackbar.icon = icon;
  snackbar.show = true;
}

// Flashes/pulses the missing client/salesperson pickers and says which one is
// missing. Returns true when something was missing.
function flagMissingAssignment(): boolean {
  const missingClient = !clientSelected.value;
  const missingSalesperson = !salespersonSelected.value;
  if (!missingClient && !missingSalesperson) return false;
  missingAssignmentFlash.value = true;
  clearTimeout(missingAssignmentFlashTimer);
  missingAssignmentFlashTimer = setTimeout(() => {
    missingAssignmentFlash.value = false;
  }, 900);
  if (missingClient) pulseMissingClient();
  if (missingSalesperson) pulseSalesperson();
  notifyCheckoutBlocked(
    [missingSalesperson && "Pick salesperson", missingClient && "Pick client"].filter(Boolean).join(" · "),
    "mdi-account-alert",
  );
  return true;
}

// The checkout button stays clickable while blocked; explain why, in the same
// order canCheckout checks things, with the cashier-fixable items first.
function explainBlockedCheckout() {
  if (quoteError.value) return notifyCheckoutBlocked("Price error", "mdi-alert-circle");
  if (pageMode.value !== "settlement") {
    const hasSale = pageMode.value === "exchange" ? hasItems.value : invoiceTotal.value > 0;
    if (!hasSale) return notifyCheckoutBlocked("Add items", "mdi-cart-outline");
    if (flagMissingAssignment()) return;
  }
  if ((quote.value?.exchangeShortfallCents ?? 0) > 0)
    return notifyCheckoutBlocked("Add more items", "mdi-swap-horizontal");
  notifyCheckoutBlocked("Not fully paid", "mdi-cash-remove");
}

async function onCheckout() {
  if (!canCheckout.value) {
    explainBlockedCheckout();
    return;
  }
  if (pageMode.value === "settlement") {
    await onSettlementCheckout();
    return;
  }
  if (invoiceItems.value && !(await invoiceItems.value.flushChanges())) return;
  const json = await getFullItemsJSON();
  if (json === "[]") return;
  const isExchange = Boolean(exchangeState.value);
  if (flagMissingAssignment()) return;
  if (!(await ensureUnknownPictures())) return;
  if (!(await confirmPricesIfUnusual())) return;
  if (!isExchange && !(await confirmOversellIfNeeded(json))) return;
  printPhase.value = "printing";
  printCashCents.value = calculator.value?.cashTotalCents ?? 0;
  printing.value = receiptEnabled.value;
  try {
    const paymentsJSON = calculator.value?.paymentsJSON ?? "[]";
    const clientID = client.id;
    const salesperson = selectedSalesperson.value;
    const result = exchangeState.value
      ? await CheckoutExchangeInvoice(
          json,
          paymentsJSON,
          exchangeState.value.originalInvoiceId,
          clientID,
          salesperson!.id,
          invoiceNotes.value,
        )
      : await CheckoutInvoice(
          json,
          paymentsJSON,
          clientID,
          branchName.value,
          branchName.value,
          "",
          salesperson!.fullName,
          receiptPrinter.value,
          invoiceNotes.value,
        );
    snackbar.text = result?.receiptError
      ? `Invoice created; receipt failed — ${result.receiptError}`
      : isExchange
        ? `Exchange completed — ${result?.invoiceCode ?? ""}`
        : receiptEnabled.value
          ? `Receipt printed — ${result?.invoiceCode ?? ""}`
          : `Invoice created — ${result?.invoiceCode ?? ""}`;
    snackbar.color = "success";
    snackbar.icon = "";
    snackbar.show = true;
    playSuccessSound();
    clearInvoice();
    exchangeState.value = null;
    notifyIncomeChanged();
    await nextTick();
    queueScreenUpdate("");
    await refreshNextInvoiceCode("sales");
    if (printing.value) {
      printPhase.value = "success";
      await new Promise((resolve) => setTimeout(resolve, 900));
    }
  } catch (e: any) {
    snackbar.text = e?.toString() || "Checkout failed";
    snackbar.color = "red-darken-4";
    snackbar.icon = "";
    snackbar.show = true;
    playErrorSound();
  }
  printing.value = false;
}

async function onExchange(payload: ExchangeSelection) {
  clearInvoice();
  invoiceItems.value?.addExchangeItems(
    payload.returned.map((item) => ({
      ...item,
      category: item.category ?? "",
      qty: Number(item.qty || 1),
    })),
    [],
  );
  setClient(payload.client);
  exchangeState.value = { originalInvoiceId: payload.originalInvoiceId };
  await refreshNextInvoiceCode("exchange");
  calculator.value?.clearAll();
  snackbar.text = "Exchange loaded. Scan or search replacement items, then check out.";
  snackbar.color = "info";
  snackbar.icon = "";
  snackbar.show = true;
}

async function cancelMode() {
  clearInvoice();
  calculator.value?.clearAll();
  exchangeState.value = null;
  settlementState.value = null;
  resetClient();
  await refreshNextInvoiceCode("sales");
}

async function refreshNextInvoiceCode(kind: "sales" | "exchange") {
  try {
    invoiceCode.value = await GetNextInvoiceCode(kind);
  } catch {
    invoiceCode.value = "Invoice code unavailable";
  }
}

function onRestoreHold(hold: any) {
  if (!invoiceItems.value) return;
  invoiceItems.value.clearItems();
  for (const item of hold.items ?? []) {
    invoiceItems.value.addItem({ ...item, qty: item.qty || 1 });
  }
  if (hold.client?.id) setClient(hold.client);
  else resetClient();
  selectedSalesperson.value = hold.salesperson ?? null;
  invoiceNotes.value = hold.notes ?? "";
  noteDraft.value = invoiceNotes.value;
  calculator.value?.restoreHoldState(hold.calculator);
  void DeleteHoldInvoice(hold.id);
}

function consumePendingHold() {
  const pending = pendingHoldInvoice.value;
  if (!pending) return;
  pendingHoldInvoice.value = null;
  onRestoreHold(pending);
}

async function consumePendingExchange() {
  const pending = pendingExchangeSelection.value;
  if (!pending) return;
  pendingExchangeSelection.value = null;
  await onExchange(pending);
}

async function consumePendingSettlement() {
  const pending = pendingSettlementInvoice.value;
  if (!pending || !invoiceItems.value) return;
  pendingSettlementInvoice.value = null;
  clearInvoice();
  // Assigned directly (bypassing addItem's scan-one-unit-at-a-time merge
  // logic) so each priced line from the invoice - including multiple
  // buckets of the same product at different custom prices - loads exactly
  // as it was, instead of being folded into one averaged line.
  invoiceItems.value.items = pending.items.map((item: any) => {
    const qty = Math.max(1, Math.trunc(Number(item.qty || 1)));
    return {
      name: item.name,
      code: item.code,
      barcode: item.barcode,
      category: item.category ?? "",
      color: item.color,
      size: item.size,
      price: Number(item.price),
      discount: Number(item.discount || 0),
      qty,
      salespersonId: Number(item.salespersonId || 0),
      note: item.note ?? "",
      priceListOverridden: Boolean(item.isPriceOverridden),
      priceListName: item.priceListName,
      discountListOverridden: Boolean(item.isDiscountOverridden),
      discountListName: item.discountListName,
      unitPrices: Array(qty).fill(Number(item.price)),
      unitDiscounts: Array(qty).fill(Number(item.discount || 0)),
    };
  });
  if (pending.client?.id) setClient(pending.client);
  selectedSalesperson.value = pending.salesperson ?? null;
  invoiceNotes.value = pending.notes ?? "";
  noteDraft.value = invoiceNotes.value;
  settlementState.value = {
    invoiceId: pending.id,
    invoiceCode: pending.invoiceCode,
    grandTotalCents: Number(pending.grandTotalCents || 0),
  };
  invoiceCode.value = pending.invoiceCode;
  await nextTick();
  calculator.value?.restoreHoldState({ entries: pending.entries ?? [] });
}

async function loadAppSettings() {
  try {
    const settings = await GetAppSettings();
    if (!pageActive) return;
    taxRate.value = Number(settings?.taxRate ?? 0);
    roundingMode.value = normalizeRoundingMode(settings?.roundingMode ?? "");
    roundingCurrency.value = settings?.roundingCurrency ?? "";
    splitPrimaryCurrency.value = settings?.splitPrimaryCurrency ?? "";
    splitPrimaryFirst.value = settings?.splitPrimaryFirst !== false;
    splitPrimaryStep.value = Number(settings?.splitPrimaryStep) || 0;
    receiptEnabled.value = Boolean(settings?.receiptEnabled);
    receiptPrinter.value = settings?.receiptPrinter ?? "";
    customItemDiscountsEnabled.value = Boolean(settings?.customItemDiscountsEnabled);
    customItemPricesEnabled.value = Boolean(settings?.customItemPricesEnabled);
    perUnitItemPricesEnabled.value = Boolean(settings?.perUnitItemPricesEnabled);
    priceChangeManualOverrideEnabled.value = Boolean(settings?.priceChangeManualOverrideEnabled);
    invoiceKeyboardModeEnabled.value = Boolean(settings?.invoiceKeyboardModeEnabled);
    hidePaymentMethodCurrency.value = Boolean(settings?.hidePaymentMethodCurrency);
    autoAppendLbpZeros.value = Boolean(settings?.autoAppendLbpZeros);
    alwaysAnonymousClientEnabled.value = Boolean(settings?.alwaysAnonymousClientEnabled);
    // Keep the shared client/salesperson composables in sync with the saved
    // settings on every load and every settings_changed event - not just when
    // the Settings page toggle runs - so a fresh launch or a remote change
    // takes effect without having to re-toggle it.
    setClientRequired(settings?.clientRequired !== false);
    setSalespersonPerInvoiceRequired(Boolean(settings?.salespersonPerInvoiceRequired));
    // Absent (older backend) means "available" - only an explicit false disables it.
    productSearchEnabled.value = settings?.productSearchEnabled !== false;
    scanByCodeEnabled.value = settings?.scanByCodeEnabled !== false;
    productSearchCodeOnly.value = Boolean(settings?.productSearchCodeOnly);
    productSearchHiddenFields.value = String(settings?.productSearchHiddenFields ?? "")
      .split(",")
      .filter(Boolean);
    priceEditRestricted.value = Boolean(settings?.priceEditRestricted);
    discountToggleEnabled.value = Boolean(settings?.discountToggleEnabled);
    discountToggleDefaultOff.value = Boolean(settings?.discountToggleDefaultOff);
    dealToggleEnabled.value = Boolean(settings?.dealToggleEnabled);
    unknownProductBarcode.value = String(settings?.unknownProductBarcode || "UNKNOWN");
    unknownPicturesAtCheckout.value = Boolean(settings?.unknownPicturesAtCheckout);
    priceEditCodes.value = String(settings?.priceEditCodes ?? "")
      .split(",")
      .map((code) => code.trim().toLowerCase())
      .filter(Boolean);
    overchangeConfirmationEnabled.value = settings?.overchangeConfirmationEnabled !== false;
    screenPort.value = settings?.screenPort ?? "";
    branchName.value = settings?.branchName || "Store";
  } catch {}
}

onMounted(async () => {
  await nextTick();
  if (!pageActive) return;
  unsubscribeListChanges = ["discount_lists", "price_lists", "products"].map((entity) =>
    EventsOn(`sync_applied:${entity}`, scheduleCartListRefresh),
  );
  unsubscribeSettingsChanged = EventsOn("settings_changed", () => {
    void loadAppSettings();
  });
  overlayObserver = new MutationObserver(scheduleBarcodeFocus);
  overlayObserver.observe(document.body, {
    attributes: true,
    attributeFilter: ["class", "style"],
    childList: true,
    subtree: true,
  });
  await restoreDraft();
  consumePendingHold();
  await consumePendingExchange();
  await consumePendingSettlement();
  focusBarcode();
  await loadAppSettings();
  if (!pageActive) return;
  try {
    await GetOpenShift();
  } catch {
    if (!pageActive) return;
    try {
      await OpenShift();
    } catch {}
  }
  if (!pageActive) return;
  showCustomerTotal();
  // A restored exchange draft (or a pending hold/exchange/settlement
  // consumed above) already set the invoice code for its own mode - don't
  // clobber it back to a plain "sales" code.
  if (pageMode.value === "sale") await refreshNextInvoiceCode("sales");
});

onBeforeUnmount(() => {
  pageActive = false;
  unsubscribeSettingsChanged?.();
  unsubscribeListChanges.forEach((unsubscribe) => unsubscribe());
  if (listRefreshTimer) clearTimeout(listRefreshTimer);
  clearTimeout(overlayFocusTimer);
  clearTimeout(missingAssignmentFlashTimer);
  overlayObserver?.disconnect();
});
</script>

<style scoped>
.empty-cart-card {
  background-color: rgb(var(--v-theme-surface)) !important;
  border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
  gap: 0 !important;
  padding: 0 !important;
}

.empty-cart-label {
  letter-spacing: 0.01em;
}

.invoice-total-summary {
  flex: 0 0 auto;
}

.invoice-total-summary__label {
  font-size: 0.8rem;
  text-transform: uppercase;
  letter-spacing: 0.06em;
}

.invoice-total-summary__usd {
  font-size: 1.5rem;
  font-weight: 700;
  line-height: 1.1;
  color: rgb(var(--v-theme-on-surface));
}

.invoice-total-summary__lbp {
  font-size: 0.95rem;
  font-weight: 500;
}

</style>

<style>
.print-spinner {
  display: block;
  flex: 0 0 auto;
  width: 48px;
  height: 48px;
  margin-inline: auto;
  border: 5px solid rgba(var(--v-theme-primary), 0.25);
  border-top-color: rgb(var(--v-theme-primary));
  border-radius: 50%;
  animation: print-spin 0.8s linear infinite !important;
}

@keyframes print-spin {
  to {
    transform: rotate(360deg);
  }
}

/* Same face used on the attendance page's "machine is working" card. */
.print-face {
  margin-inline: auto;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 72px;
  height: 72px;
  border-radius: 50%;
  font-family: "Roboto", sans-serif;
  font-size: 44px;
  font-weight: 300;
  line-height: 1;
  background-color: #9fdada;
  color: #2f8f8f;
}

.home-panes {
  flex: 1 1 0;
  min-height: 0;
}

.home-search-results {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: var(--home-search-keyboard-height, 0px);
  z-index: 11;
  background-color: rgb(var(--v-theme-surface));
}

.home-quick-access {
  display: contents;
}

.home-under-keyboard {
  visibility: hidden;
}

.home-search-keyboard {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 20;
  background-color: rgb(var(--v-theme-surface));
  box-shadow: 0 -4px 16px rgba(0, 0, 0, 0.08);
  border-top: 1px solid rgba(var(--v-border-color), 0.18);
}

.home-right-pane {
  background-color: var(--kashi-app-background);
  border-left: 1px solid rgba(var(--v-border-color), 0.18);
}

.home-left-pane {
  background-color: var(--kashi-app-background);
}

.home-page .bg-surface-light {
  background-color: rgba(var(--v-theme-on-surface), 0.04) !important;
}

.required-assignees {
  border-radius: 12px;
  padding: 2px;
}

.required-assignees--missing {
  animation: required-assignees-flash 900ms ease-in-out;
}

@keyframes required-assignees-flash {
  0%,
  100% {
    background-color: transparent;
  }
  35%,
  70% {
    background-color: rgba(255, 214, 0, 0.42);
  }
}

/* Remove these three rules to disable the barcode scan success/error flash */
.barcode-flash-success .v-field,
.barcode-flash-error .v-field {
  transition: background-color 150ms ease;
  border-radius: 4px;
}
.barcode-flash-success .v-field {
  background-color: rgba(255, 214, 0, 0.35);
}
.barcode-flash-error .v-field {
  background-color: rgba(244, 67, 54, 0.28);
}


.app-snackbar .v-snackbar__wrapper {
  background: rgba(var(--v-theme-surface-variant), 0.72) !important;
  color: rgb(var(--v-theme-on-surface-variant)) !important;
  border: 1px solid rgba(var(--v-theme-on-surface-variant), 0.1);
  box-shadow: 0 4px 18px rgba(0, 0, 0, 0.12) !important;
}
.app-snackbar--error .v-snackbar__wrapper {
  background: rgba(198, 40, 40, 0.85) !important;
  color: #fff !important;
  border-color: rgba(198, 40, 40, 0.4);
}
.checkout-blocked-text {
  font-size: 1.25rem;
  font-weight: 700;
}

.sale-option-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.sale-option-tile {
  text-transform: none;
  letter-spacing: normal;
  border-radius: 12px;
}

.sale-option-discount {
  padding: 1px 10px;
  border-radius: 999px;
  background: #ffffff;
  color: #212121;
  font-size: 0.95rem;
  font-weight: 700;
}

.sale-option-name {
  font-size: 1.25rem;
  font-weight: 800;
  line-height: 1.1;
}
</style>
