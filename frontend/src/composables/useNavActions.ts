import { ref } from "vue";
import type { ExchangeSelection } from "@/lib/exchange";

export const pendingNavAction = ref<"onhold" | "exchange" | null>(null);

export interface PendingHoldInvoice {
  id: number;
  items: any[];
  client?: any;
  salesperson?: any;
  calculator?: any;
  notes?: string;
}

export const pendingHoldInvoice = ref<PendingHoldInvoice | null>(null);
export const pendingExchangeSelection = ref<ExchangeSelection | null>(null);

export interface PendingSettlementInvoice {
  id: number;
  invoiceCode: string;
  items: any[];
  client: any;
  salesperson: any;
  notes: string;
  grandTotalCents: number;
  entries: any[];
}

export const pendingSettlementInvoice = ref<PendingSettlementInvoice | null>(null);
