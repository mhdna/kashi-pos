import { ref } from "vue";
import type { ExchangeSelection } from "@/lib/exchange";

export const pendingNavAction = ref<"onhold" | "exchange" | null>(null);

export interface PendingHoldInvoice {
  id: number;
  items: any[];
}

export const pendingHoldInvoice = ref<PendingHoldInvoice | null>(null);
export const pendingExchangeSelection = ref<ExchangeSelection | null>(null);
