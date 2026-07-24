<template>
  <v-sheet class="w-100 h-100 position-relative pa-4" rounded="xl">
    <div class="mb-2">
      <v-progress-linear
        bg-color="grey-darken-3"
        :color="diff === 0 ? 'white' : diff > 0 ? 'green-darken-2' : 'red-darken-2'"
        height="28"
        :model-value="Math.min(total > 0 ? (paidUSD / total) * 100 : 0, 100)"
        rounded
      >
        <template #default>
          <span v-if="diff === 0" class="text-body-2 font-weight-bold text-green-darken-2">Exact!</span>
          <span v-else>
            <span v-if="diff > 0">Change: +{{ diff.toFixed(2) }} $ ({{ ((paidUSD / total) * 100).toFixed(0) }}%)</span>
            <span v-else>Due: {{ Math.abs(diff).toFixed(2) }} $ ({{ ((paidUSD / total) * 100).toFixed(0) }}%)</span>
          </span>
        </template>
      </v-progress-linear>
    </div>

    <v-btn-toggle v-model="activeType" class="mb-2 w-100" divided mandatory>
      <v-btn v-for="t in types" :key="t.name" class="flex-grow-1" size="small" :value="t.name">{{ t.name }}</v-btn>
    </v-btn-toggle>

    <v-card class="pa-4 mb-4 text-right font-weight-bold" color="grey-darken-3" style="font-size: 14px">
      {{ currentInput || "0" }}
      <span v-if="currentInput && activeCurrency === 'LBP'">000</span>
      <span class="ml-1">{{ activeType }}</span>
    </v-card>

    <v-row class="font-weight-bold" :no-gutters="true">
      <v-col v-for="n in [1, 2, 3, 4, 5, 6, 7, 8, 9]" :key="n" cols="4">
        <v-btn block class="text-h6" @click="input(n.toString())">{{ n }}</v-btn>
      </v-col>
      <v-col cols="4"><v-btn block class="text-h6 font-weight-bold" @click="input('.')">.</v-btn></v-col>
      <v-col cols="4"><v-btn block class="text-h6" @click="input('0')">0</v-btn></v-col>
      <v-col cols="4"><v-btn block class="text-h6" @click="input('00')">00</v-btn></v-col>
      <v-col cols="6"><v-btn block class="text-h6" color="red-darken-2" @click="clearAll()">CA</v-btn></v-col>
      <v-col cols="6"><v-btn block class="text-h6" @click="input('back')">⌫</v-btn></v-col>
    </v-row>

    <v-btn
      block class="mt-4 text-h6 font-weight-bold"
      :color="isReturning ? 'grey' : 'grey-darken-4'"
      :disabled="currentInput === ''"
      height="44"
      @click="confirm"
    >
      {{ isReturning ? "− Return" : "+ Add" }}
    </v-btn>

    <div v-if="entries.length > 0" class="mt-4">
      <v-divider class="mb-2" />
      <div v-for="(e, i) in entries" :key="i"
        class="d-flex align-center justify-space-between py-2 px-1"
        :class="e.amount >= 0 ? 'text-green-darken-3' : 'text-red-darken-4'"
      >
        <span class="text-body-1 font-weight-medium">{{ e.type }}</span>
        <span class="text-body-1 font-weight-bold">{{ e.amount >= 0 ? "+" : "" }}{{ e.amount }} {{ getCurrencySymbol(e.type) }}</span>
        <v-btn icon size="small" variant="text" @click="removeEntry(i)">
          <v-icon icon="mdi-delete" size="18" />
        </v-btn>
      </div>
    </div>

  </v-sheet>
</template>

<script setup lang="ts">
import { computed, ref, onMounted } from "vue";
import { ListPaymentMethods } from "../../wailsjs/go/main/App";

const props = defineProps<{ total: number }>();

interface PaymentMethod { name: string; currencyCode: string; }

const types = ref<PaymentMethod[]>([]);
const activeType = ref("");
const entries = ref<{ type: string; amount: number }[]>([]);
const currentInput = ref("");

const activeCurrency = computed(() =>
  types.value.find((t) => t.name === activeType.value)?.currencyCode ?? ""
);

function toUSD(amount: number, currencyCode: string): number {
  return amount;
}

const paidUSD = computed(() =>
  entries.value.reduce((sum, e) => {
    const method = types.value.find((t) => t.name === e.type);
    return sum + toUSD(e.amount, method?.currencyCode ?? "USD");
  }, 0),
);

const diff = computed(() => paidUSD.value - props.total);
const isReturning = computed(() => diff.value >= 0);

function getCurrencySymbol(type: string): string {
  return "$";
}

onMounted(async () => {
  try {
    const methods = await ListPaymentMethods();
    types.value = methods.map((m: any) => ({ name: m.name, currencyCode: m.currencyCode }));
    if (types.value.length) activeType.value = types.value[0]!.name;
  } catch {}
});

function input(val: string) {
  if (val === "back") { currentInput.value = currentInput.value.slice(0, -1); return; }
  if (val === "clear") { currentInput.value = ""; return; }
  if (val === "." && currentInput.value.includes(".")) return;
  currentInput.value += val;
}

function confirm() {
  let amount = Number(currentInput.value);
  if (!amount) return;
  const method = types.value.find((t) => t.name === activeType.value);
  if (method?.currencyCode === "LBP") amount *= 1000;
  entries.value.push({ type: activeType.value, amount: isReturning.value ? -amount : amount });
  currentInput.value = "";
}

function removeEntry(i: number) { entries.value.splice(i, 1); }

function clearAll() { currentInput.value = ""; entries.value = []; }
</script>
