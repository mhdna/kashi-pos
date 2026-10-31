import type { InjectionKey } from "vue";

export interface BtnDefaults {
  variant?: string;
  color?: string;
  density?: string;
}
// Set by <k-card-actions> and <k-btn-toggle> so child buttons pick up defaults.
export const btnDefaultsKey: InjectionKey<BtnDefaults> = Symbol("k-btn-defaults");

export interface BtnToggle {
  isSelected(value: unknown): boolean;
  select(value: unknown): void;
}
export const btnToggleKey: InjectionKey<BtnToggle> = Symbol("k-btn-toggle");

export interface ListContext {
  nav: boolean;
  density: string;
}
export const listKey: InjectionKey<ListContext> = Symbol("k-list");

export interface TabsContext {
  current(): unknown;
  select(value: unknown): void;
  color?: string;
}
export const tabsKey: InjectionKey<TabsContext> = Symbol("k-tabs");

export interface FormContext {
  register(validate: () => boolean): () => void;
}
export const formKey: InjectionKey<FormContext> = Symbol("k-form");
