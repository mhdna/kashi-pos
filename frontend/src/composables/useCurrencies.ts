import { ref } from "vue";
import { GetAppSettings, ListCurrencies } from "../../wailsjs/go/main/App";
import { normalizeRoundingMode, type CurrencyRate, type RoundingMode } from "@/lib/money";

const currencies = ref<CurrencyRate[]>([]);
const roundingMode = ref<RoundingMode>("half_up");
let loaded = false;

async function loadCurrencies() {
  try {
    const list = (await ListCurrencies()) as any[];
    currencies.value = list.map((c) => ({
      code: String(c.code),
      symbol: String(c.symbol),
      unitsPerUsd: Number(c.unitsPerUsd),
      cashRoundingUnit: Number(c.cashRoundingUnit),
    }));
  } catch {
    /* keep previous currencies on failure */
  }
}

async function loadRoundingMode() {
  try {
    const settings = await GetAppSettings();
    roundingMode.value = normalizeRoundingMode(String(settings?.roundingMode ?? ""));
  } catch {
    /* keep previous rounding mode on failure */
  }
}

/**
 * Shared, module-cached list of currencies (from ListCurrencies()) and the
 * shop's configured rounding mode. Any component that needs to display a
 * balance in its own currency (formatBalance) or convert a cash amount
 * should use this instead of hardcoding "LBP"/a single global exchange
 * rate - adding a currency in Settings makes it usable everywhere that
 * calls this composable, with no code change.
 */
export function useCurrencies() {
  if (!loaded) {
    loaded = true;
    void loadCurrencies();
    void loadRoundingMode();
  }
  return { currencies, roundingMode, refreshCurrencies: loadCurrencies, refreshRoundingMode: loadRoundingMode };
}

export function findCurrency(currencies: CurrencyRate[], code: string): CurrencyRate | undefined {
  return currencies.find((c) => c.code === code);
}
