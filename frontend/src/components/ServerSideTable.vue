<template>
    <v-card class="mx-0" flat>
        <v-card-title class="card-title d-flex justify-space-between">
            Latest Invoices
            <v-text-field v-model="search" label="Search" prepend-inner-icon="mdi-magnify" variant="filled" clearable
                hide-details class="mx-4 mb-1 w-100" density="compact" single-line max-width="380" />
            <!-- <v-tooltip text="Search"> -->
            <!--     <template v-slot:activator="{ props }"> -->
            <!--         <v-btn v-bind="props" variant="flat" icon="mdi-magnify" @click="searchBar = !searchBar"></v-btn> -->
            <!--     </template> -->
            <!-- </v-tooltip> -->
        </v-card-title>
        <v-data-table-server v-model:items-per-page="itemsPerPage" :headers="headers" :items="serverItems"
            hide-default-footer :items-length="totalItems" :loading="loading" :search="search" item-value="name"
            @update:options="loadItems"></v-data-table-server>
    </v-card>
</template>
<script setup>
import { ref } from 'vue'

const searchBar = ref(false)
const props = defineProps({
    apiURL: {
        type: String,
        required: true,
    },
    headers: {
        type: Array,
        required: true,
    },
    rootKey: {
        type: String,
        required: true,
    }
})

async function fetchProducts({ page, itemsPerPage, sortBy }) {
    const res = await fetch(props.apiURL)
    const data = await res.json()

    let items = data[props.rootKey]

    // sorting
    if (sortBy.length) {
        const sortKey = sortBy[0].key
        const sortOrder = sortBy[0].order
        items = items.slice().sort((a, b) => {
            const aValue = a[sortKey]
            const bValue = b[sortKey]
            return sortOrder === 'desc'
                ? (bValue > aValue ? 1 : -1)
                : (aValue > bValue ? 1 : -1)
        })
    }

    // pagination
    const start = (page - 1) * itemsPerPage
    const end = start + itemsPerPage
    const paginated = items.slice(start, end)

    return {
        items: paginated,
        total: items.length,
    }
}
const itemsPerPage = ref(14);

const search = ref('')
const serverItems = ref([])
const loading = ref(true)
const totalItems = ref(0)
function loadItems({ page, itemsPerPage, sortBy }) {
    loading.value = true
    fetchProducts({ page, itemsPerPage, sortBy }).then(({ items, total }) => {
        serverItems.value = items
        totalItems.value = total
        loading.value = false
    })
}
</script>