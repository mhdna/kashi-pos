export interface Shift {
  id: number;
  total: number;
  createdAt?: string;
  closedAt?: string;
  isClosed: boolean;
}

export interface Balance {
  accountId: number;
  accountName: string;
  balance: number;
}

export interface IncomeRow {
  accountName: string;
  total: number;
}

export interface DayRow {
  day: string;
  total: number;
  shifts: Shift[];
}
