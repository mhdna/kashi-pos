import { computed, nextTick, ref } from "vue";
import {
  GetAppSettings,
  GetExchangeInvoice,
  GetProductByBarcode,
  GetProductByCode,
} from "../../wailsjs/go/main/App";
import {
  effectiveExchangePrice,
  normalizeExchangeCode,
  replacementTotal,
  type ExchangeInvoice,
  type ExchangeItem,
  type ExchangeSelection,
} from "@/lib/exchange";

export function useExchangeDialog(roundingMode: () => string) {
  const loading = ref(false);
  const scanning = ref(false);
  const invoiceCode = ref("");
  const scanValue = ref("");
  const invoice = ref<ExchangeInvoice | null>(null);
  const returned = ref<ExchangeItem | null>(null);
  const replacements = ref<ExchangeItem[]>([]);
  const invoiceError = ref("");
  const scanError = ref("");
  const invoiceInput = ref<InstanceType<typeof import("vuetify/components").VTextField> | null>(
    null,
  );
  const scanInput = ref<InstanceType<typeof import("vuetify/components").VTextField> | null>(null);

  const replacementsTotal = computed(() =>
    replacementTotal(replacements.value, roundingMode()),
  );
  const returnedTotal = computed(() =>
    returned.value ? effectiveExchangePrice(returned.value, roundingMode()) : 0,
  );
  const exchangeDifference = computed(() =>
    Math.max(0, replacementsTotal.value - returnedTotal.value),
  );
  const remainingReplacementValue = computed(() =>
    Math.max(0, returnedTotal.value - replacementsTotal.value),
  );
  const hasEnoughReplacementValue = computed(
    () => Boolean(returned.value) && replacementsTotal.value >= returnedTotal.value,
  );

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
        items: (loaded.items ?? []).map((item) => ({
          ...item,
          price: Number(item.price),
          discount: Number(item.discount),
          available: Number(item.available),
        })),
      };
      returned.value = null;
      replacements.value = [];
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
    returned.value = {
      ...item,
      price: effectiveExchangePrice(item, roundingMode()),
      discount: 0,
      qty: -1,
    };
    replacements.value = [];
    scanError.value = "";
    scanValue.value = "";
    focusScanInput();
  }

  async function scanItem() {
    const value = scanValue.value.trim();
    if (!value || scanning.value) return;
    scanError.value = "";

    if (!returned.value) {
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
      return;
    }

    scanning.value = true;
    try {
      const product = await findProduct(value);
      if (normalizeExchangeCode(product.code) === normalizeExchangeCode(returned.value.code)) {
        throw new Error("Choose a different product as the replacement.");
      }
      const existing = replacements.value.find(
        (item) => normalizeExchangeCode(item.code) === normalizeExchangeCode(product.code),
      );
      if (existing) {
        existing.qty = Number(existing.qty || 1) + 1;
      } else {
        replacements.value.push({
          ...product,
          price: Number(product.price),
          discount: Number(product.discount),
          qty: 1,
        });
      }
      scanValue.value = "";
    } catch (error: any) {
      scanValue.value = "";
      scanError.value = error?.toString() || "Replacement product not found.";
    } finally {
      scanning.value = false;
      focusScanInput();
    }
  }

  function removeReplacement(code: string) {
    replacements.value = replacements.value.filter((item) => item.code !== code);
    focusScanInput();
  }

  function changeInvoice() {
    invoice.value = null;
    returned.value = null;
    replacements.value = [];
    scanValue.value = "";
    scanError.value = "";
    invoiceError.value = "";
    focusInvoiceInput();
  }

  function createSelection(): ExchangeSelection | null {
    if (!invoice.value || !returned.value || !hasEnoughReplacementValue.value) return null;
    return {
      originalInvoiceId: invoice.value.id,
      client: { ...invoice.value.client },
      returned: { ...returned.value },
      replacements: replacements.value.map((item) => ({ ...item })),
    };
  }

  function clearForm() {
    invoiceCode.value = "";
    invoice.value = null;
    returned.value = null;
    replacements.value = [];
    scanValue.value = "";
    invoiceError.value = "";
    scanError.value = "";
    loading.value = false;
    scanning.value = false;
  }

  return {
    loading,
    scanning,
    invoiceCode,
    scanValue,
    invoice,
    returned,
    replacements,
    invoiceError,
    scanError,
    invoiceInput,
    scanInput,
    replacementsTotal,
    exchangeDifference,
    remainingReplacementValue,
    hasEnoughReplacementValue,
    focusInvoiceInput,
    loadInvoice,
    selectReturned,
    scanItem,
    removeReplacement,
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

async function findProduct(value: string): Promise<ExchangeItem> {
  try {
    return (await GetProductByBarcode(value)) as ExchangeItem;
  } catch {
    return (await GetProductByCode(value)) as ExchangeItem;
  }
}
