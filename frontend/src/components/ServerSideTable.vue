<template>
  <v-card class="mx-0" flat>
    <v-card-title class="card-title d-flex justify-space-between">
      Latest Invoices
      <v-text-field
        v-model="search"
        class="mx-4 mb-1 w-100"
        clearable
        density="compact"
        hide-details
        label="Search"
        max-width="380"
        single-line
        variant="filled"
        prepend-inner-icon="mdi-magnify"
      >
      </v-text-field>
    </v-card-title>
    <v-data-table-server
      v-model:items-per-page="itemsPerPage"
      :headers="props.header as any"
      hide-default-footer
      item-value="name"
      :items="serverItems"
      :items-length="totalItems"
      :loading="loading"
      :search="search"
      @update:options="loadItems"
    />
  </v-card>
</template>

<script setup lang="ts">
  import { ref } from 'vue'

  const props = defineProps<{
    apiURL: string
    header: { title: string, align: string, key: string }[]
    rootKey: string
  }>()

  const itemsPerPage = ref(14)
  const search = ref('')
  const serverItems = ref<any[]>([])
  const loading = ref(true)
  const totalItems = ref(0)

  async function fetchProducts ({ page, itemsPerPage, sortBy }: { page: number, itemsPerPage: number, sortBy: { key: string, order: string }[] }) {
    const res = await fetch(props.apiURL)
    const data = await res.json()
    let items = data[props.rootKey]
    if (sortBy.length > 0 && sortBy[0]) {
      const sortKey = sortBy[0].key
      const sortOrder = sortBy[0].order
      items = items.slice().toSorted((a: any, b: any) => {
        const aValue = a[sortKey], bValue = b[sortKey]
        return sortOrder === 'desc' ? (bValue > aValue ? 1 : -1) : (aValue > bValue ? 1 : -1)
      })
    }
    return { items: items.slice((page - 1) * itemsPerPage, (page - 1) * itemsPerPage + itemsPerPage), total: items.length }
  }

  function loadItems ({ page, itemsPerPage, sortBy }: { page: number, itemsPerPage: number, sortBy: { key: string, order: string }[] }) {
    loading.value = true
    fetchProducts({ page, itemsPerPage, sortBy }).then(({ items, total }) => {
      serverItems.value = items
      totalItems.value = total
      loading.value = false
    })
  }
</script>
