<script setup lang="ts">
import { computed, getCurrentInstance, ref } from "vue";
import KIcon from "./KIcon";
import KProgressLinear from "./KProgressLinear.vue";
import { useScrollWindow } from "./useScrollWindow";
import { toUnit } from "./util";
import { nextSort, sortItems, visibleRange, type SortState } from "./virtual";

export interface Header {
  title?: string;
  key: string;
  sortable?: boolean;
  width?: string | number;
  align?: "start" | "center" | "end";
}

// One table for both k-data-table and k-data-table-virtual. Rows are
// windowed only when `item-height` is given (the virtual flavour).
const props = defineProps<{
  headers: readonly Header[];
  items: readonly any[];
  itemValue?: string;
  itemHeight?: number | string;
  height?: string | number;
  fixedHeader?: boolean;
  loading?: boolean;
  hover?: boolean;
  density?: string;
  noDataText?: string;
  itemsPerPage?: number | string;
  hideDefaultFooter?: boolean;
}>();
const emit = defineEmits<{ "click:row": [event: MouseEvent, scope: { item: any; index: number }] }>();
defineSlots<Record<string, (scope: any) => unknown>>();

// A pointer cursor only when someone listens for row clicks.
const rowsClickable = !!getCurrentInstance()?.vnode.props?.["onClick:row"];

const wrapper = ref<HTMLElement | null>(null);
const { scrollTop, height: viewport } = useScrollWindow(wrapper);
const sort = ref<SortState | null>(null);

const sorted = computed(() => sortItems(props.items, sort.value));
const virtual = computed(() => props.itemHeight !== undefined);
const rowHeight = computed(() => Number(props.itemHeight));
const range = computed(() =>
  virtual.value ? visibleRange(scrollTop.value, Math.max(0, viewport.value - 56), rowHeight.value, sorted.value.length) : { start: 0, end: sorted.value.length, padTop: 0, padBottom: 0 },
);
const rows = computed(() => sorted.value.slice(range.value.start, range.value.end).map((item, i) => ({ item, index: range.value.start + i })));

function isSortable(h: Header) {
  return h.sortable !== false && !!h.title;
}
function onHeaderClick(h: Header) {
  if (isSortable(h)) sort.value = nextSort(sort.value, h.key);
}
function alignClass(h: Header) {
  return h.align === "end" ? "k-align-end" : h.align === "center" ? "k-align-center" : undefined;
}
function rowKey(item: any, index: number) {
  const v = props.itemValue ? item?.[props.itemValue] : undefined;
  return v ?? index;
}
function scrollToIndex(index: number) {
  if (wrapper.value) wrapper.value.scrollTop = Math.max(0, index) * (virtual.value ? rowHeight.value : 52);
}
defineExpose({ scrollToIndex });
</script>

<template>
  <div
    class="k-table"
    :class="[fixedHeader && 'k-table--fixed-header', hover && 'k-table--hover', density && density !== 'default' && `k-table--density-${density}`]"
    :style="{ height: toUnit(height) }"
  >
    <div v-if="loading" class="k-table__loading"><KProgressLinear indeterminate :height="2" /></div>
    <div ref="wrapper" class="k-table__wrapper">
      <table>
        <colgroup>
          <col v-for="h in headers" :key="h.key" :style="{ width: toUnit(h.width) }" />
        </colgroup>
        <thead>
          <tr>
            <th v-for="h in headers" :key="h.key" :class="[alignClass(h), isSortable(h) && 'k-sortable']" @click="onHeaderClick(h)">
              <div class="k-table__th-content">
                <slot :name="`header.${h.key}`" :column="h">
                  <span class="k-table__sort">
                    {{ h.title }}
                    <KIcon v-if="sort?.key === h.key" :icon="sort.direction === 'asc' ? 'mdi-arrow-up' : 'mdi-arrow-down'" />
                  </span>
                </slot>
              </div>
            </th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="range.padTop" aria-hidden="true"><td :colspan="headers.length" :style="{ height: `${range.padTop}px`, padding: 0, border: 0 }" /></tr>
          <tr
            v-for="row in rows"
            :key="rowKey(row.item, row.index)"
            :class="{ 'k-table__row-clickable': rowsClickable }"
            :style="virtual ? { height: `${rowHeight}px` } : undefined"
            @click="emit('click:row', $event, { item: row.item, index: row.index })"
          >
            <td v-for="h in headers" :key="h.key" :class="alignClass(h)">
              <slot :name="`item.${h.key}`" :item="row.item" :value="row.item?.[h.key]" :index="row.index">{{ row.item?.[h.key] }}</slot>
            </td>
          </tr>
          <tr v-if="range.padBottom" aria-hidden="true"><td :colspan="headers.length" :style="{ height: `${range.padBottom}px`, padding: 0, border: 0 }" /></tr>
          <tr v-if="!items.length && !loading" class="k-table__no-data">
            <td :colspan="headers.length"><slot name="no-data">{{ noDataText ?? "No data available" }}</slot></td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
