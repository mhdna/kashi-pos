export interface Shift {
  id: number;
  total: number;
  priceDeduction?: number;
  createdAt?: string;
  closedAt?: string;
  isClosed: boolean;
}

export interface Balance {
  accountId: number;
  accountName: string;
  balance: number;
  currencyCode?: string;
  isOnAccount?: boolean;
  isCollected?: boolean;
}

export interface PaymentMethod {
  id: number;
  name: string;
  displayName?: string;
  currencyCode?: string;
}

export interface IncomeRow {
  accountId?: number;
  accountName: string;
  total: number;
  currencyCode?: string;
  isOnAccount?: boolean;
  isCollected?: boolean;
}

export interface DayRow {
  day: string;
  total: number;
  shifts: Shift[];
}
