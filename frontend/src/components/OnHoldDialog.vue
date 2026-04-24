<template>
    <div class="text-center">
        <v-dialog v-model="dialog" max-width="800">
            <template v-slot:activator="{ props }">
                <v-btn variant="tonal" v-bind="props" style="font-size: 17px;">
                    <div>
                        Invoices On Hold
                    </div>
                    <v-avatar size="16" class="mx-2 me-0 pa-3" color="red">
                        4
                    </v-avatar>
                </v-btn>
            </template>
            <v-card max-width="800" height="700" class="px-2 pb-2" title="Invoices On Hold">
                <v-data-table :headers="headers" :items="cashboxAccounts" hide-default-footer />
            </v-card>
        </v-dialog>
    </div>
</template>
<script setup>
import { ref } from 'vue'

const dialog = ref(false)

const headers = [
    {
        title: 'Name',
        align: 'start',
        key: 'name',
    },
    {
        title: 'Amount',
        align: 'start',
        key: 'totalAmount',
    },
    {
        title: 'Currency',
        align: 'start',
        key: 'currencyCode',
    },
    {
        title: 'Last Updated',
        align: 'start',
        key: 'lastUpdate',
    },
]

const cashboxAccounts = ref([
    {
        id: 1,
        name: "Cash USD",
        currencyCode: "USD",
        totalAmount: 0,
        lastUpdate: null
    },
    {
        id: 2,
        name: "PayPal",
        currencyCode: "USD",
        totalAmount: 0,
        lastUpdate: null
    },
    {
        id: 3,
        name: "Test",
        currencyCode: "USD",
        totalAmount: 0,
        lastUpdate: null
    },
    {
        id: 4,
        name: "Test",
        currencyCode: "USD",
        totalAmount: 0,
        lastUpdate: null
    }
])
</script>