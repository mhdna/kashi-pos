import { nextTick, ref } from "vue";
import { GetAppSettings, GetExchangeInvoice } from "../../wailsjs/go/main/App";
import {
  effectiveExchangePrice,
  mergeDealUnits,
  normalizeExchangeCode,
  type ExchangeInvoice,
  type ExchangeItem,
  type ExchangeSelection,
} from "@/lib/exchange";

// Selecting returned items is the dialog's only job - replacements are
// scanned/searched on the home screen itself after "Start exchange", with
// full discount/custom-price editing (see home.vue's onExchange).
export function useExchangeDialog(roundingMode: () => string) {
  const loading = ref(false);
  const invoiceCode = ref("");
  const scanValue = ref("");
  const invoice = ref<ExchangeInvoice | null>(null);
  const returned = ref<ExchangeItem[]>([]);
  const invoiceError = ref("");
  const scanError = ref("");
  const invoiceInput = ref<InstanceType<typeof import("@/ui").KTextField> | null>(
    null,
  );
  const scanInput = ref<InstanceType<typeof import("@/ui").KTextField> | null>(null);

  function focusInvoiceInput() {
    nextTick(() => invoiceInput.value?.focus());
  }

  function focusScanInput() {
    nextTick(() => scanInput.value?.focus());
  }

  async function loadInvoice() {
    const code = invoiceCode.value.trim();
    if (!code || loading.value) return;
    loading.value = true;
    invoiceError.value = "";
    try {
      const loaded = (await GetExchangeInvoice(await fullInvoiceCode(code))) as ExchangeInvoice;
      invoiceCode.value = loaded.invoiceCode;
      invoice.value = {
        ...loaded,
        items: mergeDealUnits((loaded.items ?? []).map((item) => ({
          ...item,
          price: Number(item.price),
          discount: Number(item.discount),
          available: Number(item.available),
        }))),
      };
      returned.value = [];
      scanValue.value = "";
      focusScanInput();
    } catch (error: any) {
      invoice.value = null;
      invoiceError.value = error?.toString() || "Sales invoice not found.";
      focusInvoiceInput();
    } finally {
      loading.value = false;
    }
  }

  function selectReturned(item: ExchangeItem) {
    const existing = returned.value.find(
      (candidate) => normalizeExchangeCode(candidate.code) === normalizeExchangeCode(item.code)
        && candidate.soldUnitPrice === Number(item.price) && candidate.soldUnitDiscount === Number(item.discount),
    );
    if (existing) {
      if (Number(existing.qty || 1) >= Number(item.available || 1)) {
        scanError.value = "All available units of this item are already selected.";
        return;
      }
      existing.qty = Number(existing.qty || 1) + 1;
    } else {
      returned.value.push({
        ...item,
        soldUnitPrice: Number(item.price),
        soldUnitDiscount: Number(item.discount),
        price: effectiveExchangePrice(item, roundingMode()),
        discount: 0,
        qty: 1,
      });
    }
    scanError.value = "";
    scanValue.value = "";
    focusScanInput();
  }

  function scanItem() {
    const value = scanValue.value.trim();
    if (!value) return;
    scanError.value = "";

    const scanned = normalizeExchangeCode(value);
    const item = invoice.value?.items.find(
      (candidate) =>
        normalizeExchangeCode(candidate.code) === scanned ||
        normalizeExchangeCode(candidate.barcode || "") === scanned,
    );
    if (!item) {
      scanError.value = "This product is not available to return on the loaded invoice.";
      scanValue.value = "";
      focusScanInput();
      return;
    }
    selectReturned(item);
  }

  function removeReturned(code: string, price: number, discount: number) {
    returned.value = returned.value.filter(
      (item) => !(normalizeExchangeCode(item.code) === normalizeExchangeCode(code)
        && item.soldUnitPrice === price && item.soldUnitDiscount === discount),
    );
    focusScanInput();
  }

  function changeInvoice() {
    invoice.value = null;
    returned.value = [];
    scanValue.value = "";
    scanError.value = "";
    invoiceError.value = "";
    focusInvoiceInput();
  }

  function createSelection(): ExchangeSelection | null {
    if (!invoice.value || returned.value.length === 0) return null;
    return {
      originalInvoiceId: invoice.value.id,
      client: { ...invoice.value.client },
      returned: returned.value.map((item) => ({ ...item })),
    };
  }

  function clearForm() {
    invoiceCode.value = "";
    invoice.value = null;
    returned.value = [];
    scanValue.value = "";
    invoiceError.value = "";
    scanError.value = "";
    loading.value = false;
  }

  return {
    loading,
    invoiceCode,
    scanValue,
    invoice,
    returned,
    invoiceError,
    scanError,
    invoiceInput,
    scanInput,
    focusInvoiceInput,
    loadInvoice,
    selectReturned,
    scanItem,
    removeReturned,
    changeInvoice,
    createSelection,
    clearForm,
  };
}

async function fullInvoiceCode(value: string): Promise<string> {
  if (!/^\d{5}$/.test(value)) return value;

  const settings = await GetAppSettings();
  const branchCode = String(settings?.branchCode ?? "").trim().toUpperCase();
  if (!branchCode) throw new Error("Branch code is not configured.");

  return `SA/${branchCode}/${new Date().getFullYear()}/${value}`;
}
