<template>
  <v-card class="pa-4">
    <v-card-title class="d-flex align-center justify-space-between">
      Loans
      <v-btn color="primary" prepend-icon="mdi-plus" @click="dialog = true">Add loan</v-btn>
    </v-card-title>
    <v-data-table :headers="headers" :items="items" :loading="loading" density="compact" hover>
      <template #item.amount="{ item }">{{ formatCents(item.amount) }} {{ item.currencyCode }}</template>
      <template #item.occurredAt="{ item }">{{ formatServerDateTime(item.occurredAt) }}</template>
    </v-data-table>

    <v-dialog v-model="dialog" max-width="480">
      <v-card>
        <v-card-title>Add loan</v-card-title>
        <v-card-text>
          <v-form ref="formRef" v-model="formValid">
            <v-text-field v-model="form.description" label="Description" :rules="[required]" />
            <v-combobox
              v-model="form.category"
              :items="loanCategories"
              item-title="name"
              item-value="name"
              :return-object="false"
              label="Category"
              :rules="[required]"
            >
              <template #item="{ props: itemProps, item }">
                <v-list-item v-bind="itemProps" :prepend-icon="item.raw.icon || 'mdi-tag-outline'" />
              </template>
            </v-combobox>
            <v-text-field v-model.number="form.amount" label="Amount" type="number" :rules="[required]" />
            <v-select
              v-model="form.cashboxAccountId"
              :items="paymentMethods"
              item-title="displayName"
              item-value="id"
              label="Deposited to"
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
import { ref, onMounted, onBeforeUnmount } from "vue";
import { CreateLoan, ListLoans, ListLoanCategories, ListPaymentMethods } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";
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
const loanCategories = ref<any[]>([]);

const dialog = ref(false);
const saving = ref(false);
const formValid = ref(false);
const formRef = ref();
const form = ref({ description: "", category: "", amount: null as number | null, cashboxAccountId: null as number | null });

const required = (v: unknown) => (v !== null && v !== undefined && v !== "") || "Required";

function formatCents(cents: number): string {
  return (Number(cents ?? 0) / 100).toFixed(2);
}

async function loadLoans() {
  loading.value = true;
  try {
    items.value = await ListLoans(50, 0);
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

async function loadLoanCategories() {
  try {
    loanCategories.value = (await ListLoanCategories()) as any[];
  } catch {
    loanCategories.value = [];
  }
}

async function submit() {
  const valid = await formRef.value?.validate();
  if (!valid?.valid) return;

  const account = paymentMethods.value.find((m) => m.id === form.value.cashboxAccountId);
  saving.value = true;
  try {
    await CreateLoan(
      form.value.description,
      form.value.category,
      Math.round((form.value.amount ?? 0) * 100),
      account?.currencyCode ?? "USD",
      form.value.cashboxAccountId as number,
    );
    dialog.value = false;
    form.value = { description: "", category: "", amount: null, cashboxAccountId: null };
    await loadLoans();
  } finally {
    saving.value = false;
  }
}

let categorySyncUnsub: (() => void) | null = null;

onMounted(async () => {
  await Promise.all([loadLoans(), loadPaymentMethods(), loadLoanCategories()]);
  categorySyncUnsub = EventsOn("sync_applied:loan_categories", loadLoanCategories);
});

onBeforeUnmount(() => {
  categorySyncUnsub?.();
});
</script>
