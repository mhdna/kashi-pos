export const INCOME_CHANGED_EVENT = "kashi:income-changed";

export function notifyIncomeChanged() {
  window.dispatchEvent(new Event(INCOME_CHANGED_EVENT));
}
