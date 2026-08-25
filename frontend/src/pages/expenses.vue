<template>
  <v-card class="pa-4">
    <v-card-title class="d-flex align-center justify-space-between">
      Expenses
      <v-btn color="primary" prepend-icon="mdi-plus" @click="dialog = true">Add expense</v-btn>
    </v-card-title>
    <v-data-table :headers="headers" :items="items" :loading="loading" density="compact" hover>
      <template #item.amount="{ item }">{{ formatCents(item.amount) }} {{ item.currencyCode }}</template>
      <template #item.occurredAt="{ item }">{{ formatServerDateTime(item.occurredAt) }}</template>
    </v-data-table>

    <v-dialog v-model="dialog" max-width="480">
      <v-card>
        <v-card-title>Add expense</v-card-title>
        <v-card-text>
          <v-form ref="formRef" v-model="formValid">
            <v-text-field v-model="form.description" label="Description" :rules="[required]" />
            <v-text-field v-model="form.category" label="Category" />
            <v-text-field v-model.number="form.amount" label="Amount" type="number" :rules="[required]" />
            <v-select
              v-model="form.cashboxAccountId"
              :items="paymentMethods"
              item-title="displayName"
              item-value="id"
              label="Paid from"
              :rules="[required]"
            />
          </v-form>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="dialog = false">Cancel</v-btn>
          <v-btn color="primary" :loading="saving" @click="submit">Save</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </v-card>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { CreateExpense, ListExpenses, ListPaymentMethods } from "../../wailsjs/go/main/App";
import { formatServerDateTime } from "@/lib/datetime";
import { isSpecialPaymentMethod } from "@/lib/paymentMethods";

const headers = [
  { title: "ID", key: "id" },
  { title: "Description", key: "description" },
  { title: "Category", key: "category" },
  { title: "Amount", key: "amount" },
  { title: "Date", key: "occurredAt" },
];
const items = ref<any[]>([]);
const loading = ref(true);
const paymentMethods = ref<any[]>([]);

const dialog = ref(false);
const saving = ref(false);
const formValid = ref(false);
const formRef = ref();
const form = ref({ description: "", category: "", amount: null as number | null, cashboxAccountId: null as number | null });

const required = (v: unknown) => (v !== null && v !== undefined && v !== "") || "Required";

function formatCents(cents: number): string {
  return (Number(cents ?? 0) / 100).toFixed(2);
}

async function loadExpenses() {
  loading.value = true;
  try {
    items.value = await ListExpenses(50, 0);
  } catch {
    items.value = [];
  }
  loading.value = false;
}

async function loadPaymentMethods() {
  try {
    const all = (await ListPaymentMethods()) as any[];
    paymentMethods.value = all.filter((m) => !isSpecialPaymentMethod(m.name));
  } catch {
    paymentMethods.value = [];
  }
}

async function submit() {
  const valid = await formRef.value?.validate();
  if (!valid?.valid) return;

  const account = paymentMethods.value.find((m) => m.id === form.value.cashboxAccountId);
  saving.value = true;
  try {
    await CreateExpense(
      form.value.description,
      form.value.category,
      Math.round((form.value.amount ?? 0) * 100),
      account?.currencyCode ?? "USD",
      form.value.cashboxAccountId as number,
    );
    dialog.value = false;
    form.value = { description: "", category: "", amount: null, cashboxAccountId: null };
    await loadExpenses();
  } finally {
    saving.value = false;
  }
}

onMounted(async () => {
  await Promise.all([loadExpenses(), loadPaymentMethods()]);
});
</script>
