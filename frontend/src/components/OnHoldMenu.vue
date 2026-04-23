<template>
    <div class="text-center">
        <v-menu v-model="menu" :close-on-content-click="false" location="bottom">
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
            <v-card min-width="500">
                <v-data-table :headers="headers" :items="cashboxAccounts" hide-default-footer />
            </v-card>
        </v-menu>
    </div>
</template>
<script setup>
import { ref } from 'vue'

const menu = ref(false)

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