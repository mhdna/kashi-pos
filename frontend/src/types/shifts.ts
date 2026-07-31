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
}

export interface IncomeRow {
  accountName: string;
  total: number;
  currencyCode?: string;
}

export interface DayRow {
  day: string;
  total: number;
  shifts: Shift[];
}
