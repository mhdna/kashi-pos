export const INCOME_CHANGED_EVENT = "central:income-changed";

export function notifyIncomeChanged() {
  window.dispatchEvent(new Event(INCOME_CHANGED_EVENT));
}
