export interface Shift {
  id: number;
  total: number;
  priceDeduction?: number;
  createdAt?: string;
  closedAt?: string;
  isClosed: boolean;
  countedUsd?: number;
  countedLbp?: number;
  countedVisa?: number;
  countedWhish?: number;
  expectedUsd?: number;
  expectedLbp?: number;
  expectedVisa?: number;
  expectedWhish?: number;
  openingFloatUsd?: number;
  openingFloatLbp?: number;
  closedBySalesperson?: string;
}

export interface Balance {
  accountId: number;
  accountName: string;
  balance: number;
  currencyCode?: string;
  color?: string;
  isOnAccount?: boolean;
  isOnCoupon?: boolean;
  isCollected?: boolean;
  isAutoCounted?: boolean;
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
  color?: string;
  isOnAccount?: boolean;
  isCollected?: boolean;
}

export interface DayRow {
  day: string;
  total: number;
  shifts: Shift[];
}
