<template>
  <v-card class="pa-4">
    <div class="d-flex align-center ga-3 mb-3">
      <v-card-title class="pa-0">Products</v-card-title>
      <v-spacer />
      <v-text-field
        v-model="search"
        prepend-inner-icon="mdi-magnify"
        placeholder="Search name, code or barcode"
        density="compact"
        variant="outlined"
        hide-details
        clearable
        class="products-search"
      />
      <v-btn v-if="standalone" color="primary" prepend-icon="mdi-plus" @click="openCreate">
        Add product
      </v-btn>
    </div>

    <v-alert v-if="!standalone" type="info" variant="tonal" density="compact" class="mb-3">
      This till is connected to kashi, which manages products. Add, edit or remove them in
      kashi; changes arrive here automatically.
    </v-alert>

    <v-table density="compact" hover>
      <thead>
        <tr>
          <th>Name</th>
          <th>Barcode</th>
          <th>Code</th>
          <th>Color / size</th>
          <th class="text-right">Price</th>
          <th class="text-right">Discount</th>
          <th>Status</th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="product in products"
          :key="product.id"
          :class="{ 'products-row--clickable': standalone }"
          @click="openEdit(product)"
        >
          <td>{{ product.name }}</td>
          <td>{{ product.barcode }}</td>
          <td>{{ product.code }}</td>
          <td>{{ [product.color, product.size].filter(Boolean).join(" / ") }}</td>
          <td class="text-right">{{ formatUsd(product.priceCents / 100) }}</td>
          <td class="text-right">{{ product.discount ? `${product.discount}%` : "" }}</td>
          <td>
            <v-chip v-if="!product.isActive" size="x-small" variant="tonal">Inactive</v-chip>
          </td>
        </tr>
        <tr v-if="!loading && products.length === 0">
          <td colspan="7" class="text-center text-medium-emphasis">No products</td>
        </tr>
      </tbody>
    </v-table>

    <div class="d-flex justify-center mt-3">
      <v-progress-circular v-if="loading" indeterminate color="primary" size="24" width="2" />
      <v-btn v-else-if="hasMore" variant="text" @click="loadMore">Load more</v-btn>
    </div>
  </v-card>

  <v-dialog v-model="editOpen" max-width="480">
    <v-card>
      <v-card-title>{{ editTarget ? "Edit product" : "Add product" }}</v-card-title>
      <v-card-text>
        <v-text-field v-model="form.name" label="Name" variant="outlined" autofocus />
        <v-text-field v-model="form.barcode" label="Barcode" variant="outlined" />
        <v-text-field
          v-model="form.code"
          label="Code"
          hint="Defaults to the barcode"
          persistent-hint
          variant="outlined"
          class="mb-2"
        />
        <div class="d-flex ga-2">
          <v-text-field v-model="form.color" label="Color" variant="outlined" />
          <v-text-field v-model="form.size" label="Size" variant="outlined" />
        </div>
        <div class="d-flex ga-2">
          <v-text-field v-model="form.price" label="Price (USD)" type="number" min="0" variant="outlined" />
          <v-text-field
            v-model="form.discount"
            label="Discount %"
            type="number"
            min="0"
            max="100"
            variant="outlined"
          />
        </div>
        <v-text-field v-model="form.description" label="Category / description" variant="outlined" />
        <v-switch v-model="form.isActive" label="Active (sellable)" color="primary" hide-details />
      </v-card-text>
      <v-card-actions>
        <v-btn
          v-if="editTarget && auth.isAdmin"
          color="error"
          variant="text"
          :loading="deleting"
          @click="deleteOpen = true"
          >Delete</v-btn
        >
        <v-spacer />
        <v-btn variant="text" @click="editOpen = false">Cancel</v-btn>
        <v-btn color="primary" :loading="saving" :disabled="!canSave" @click="save">Save</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-dialog v-model="deleteOpen" max-width="420">
    <v-card>
      <v-card-title>Delete product?</v-card-title>
      <v-card-text>
        This will delete {{ editTarget?.name || "this product" }}. A product that was sold,
        transferred, held or still has stock is deactivated instead, so its history stays intact.
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="deleteOpen = false">Cancel</v-btn>
        <v-btn color="error" :loading="deleting" @click="deleteSelected">Delete</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-snackbar v-model="snackbar.show" timeout="3500">{{ snackbar.text }}</v-snackbar>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue";
import {
  CreateProduct,
  DeleteProduct,
  IsStandaloneMode,
  ListManagedProducts,
  UpdateProduct,
} from "../../wailsjs/go/main/App";
import { main } from "../../wailsjs/go/models";
import { useAuthStore } from "@/stores/auth";
import { formatUsd } from "@/lib/format";

type ManagedProduct = {
  id: number;
  uid: string;
  barcode: string;
  code: string;
  name: string;
  description: string;
  priceCents: number;
  discount: number;
  color: string;
  size: string;
  isActive: boolean;
};

const auth = useAuthStore();
const batchSize = 50;
const products = ref<ManagedProduct[]>([]);
const search = ref("");
const cursor = ref(0);
const hasMore = ref(true);
const loading = ref(false);
const standalone = ref(false);
const editOpen = ref(false);
const deleteOpen = ref(false);
const saving = ref(false);
const deleting = ref(false);
const editTarget = ref<ManagedProduct | null>(null);
const snackbar = reactive({ show: false, text: "" });
const form = reactive({
  name: "",
  barcode: "",
  code: "",
  color: "",
  size: "",
  price: "",
  discount: "0",
  description: "",
  isActive: true,
});

const canSave = computed(
  () => form.name.trim() !== "" && form.barcode.trim() !== "" && Number(form.price) > 0,
);

let requestSequence = 0;

async function loadMore() {
  if (loading.value || !hasMore.value) return;
  const sequence = requestSequence;
  loading.value = true;
  try {
    const page = await ListManagedProducts(cursor.value, batchSize, search.value ?? "");
    if (sequence !== requestSequence) return;
    products.value.push(...((page.items ?? []) as ManagedProduct[]));
    cursor.value = page.nextCursor;
    hasMore.value = page.hasMore;
  } catch (e: any) {
    snackbar.text = e?.toString() || "Could not load products";
    snackbar.show = true;
  } finally {
    if (sequence === requestSequence) loading.value = false;
  }
}

function reload() {
  requestSequence++;
  products.value = [];
  cursor.value = 0;
  hasMore.value = true;
  loading.value = false;
  void loadMore();
}

let searchTimer: ReturnType<typeof setTimeout> | undefined;
watch(search, () => {
  clearTimeout(searchTimer);
  searchTimer = setTimeout(reload, 250);
});

function fillForm(product: ManagedProduct | null) {
  form.name = product?.name ?? "";
  form.barcode = product?.barcode ?? "";
  form.code = product?.code ?? "";
  form.color = product?.color ?? "";
  form.size = product?.size ?? "";
  form.price = product ? String(product.priceCents / 100) : "";
  form.discount = String(product?.discount ?? 0);
  form.description = product?.description ?? "";
  form.isActive = product?.isActive ?? true;
}

function openCreate() {
  editTarget.value = null;
  fillForm(null);
  editOpen.value = true;
}

function openEdit(product: ManagedProduct) {
  if (!standalone.value) return;
  editTarget.value = product;
  fillForm(product);
  editOpen.value = true;
}

function formInput(): main.ProductInput {
  return main.ProductInput.createFrom({
    name: form.name,
    barcode: form.barcode,
    code: form.code,
    color: form.color,
    size: form.size,
    description: form.description,
    priceCents: Math.round(Number(form.price) * 100),
    discount: Math.round(Number(form.discount) || 0),
    isActive: form.isActive,
  });
}

async function save() {
  saving.value = true;
  try {
    if (editTarget.value) {
      const updated = (await UpdateProduct(editTarget.value.id, formInput())) as ManagedProduct;
      Object.assign(editTarget.value, updated);
      snackbar.text = "Product updated";
    } else {
      await CreateProduct(formInput());
      reload();
      snackbar.text = "Product added";
    }
    editOpen.value = false;
  } catch (e: any) {
    snackbar.text = e?.toString() || "Could not save product";
  } finally {
    saving.value = false;
    snackbar.show = true;
  }
}

async function deleteSelected() {
  if (!editTarget.value) return;
  deleting.value = true;
  try {
    const outcome = await DeleteProduct(editTarget.value.id);
    const target = editTarget.value;
    if (outcome === "deactivated") {
      target.isActive = false;
      snackbar.text = "Product has history, so it was deactivated instead";
    } else {
      products.value = products.value.filter((p) => p.id !== target.id);
      snackbar.text = "Product deleted";
    }
    deleteOpen.value = false;
    editOpen.value = false;
  } catch (e: any) {
    snackbar.text = e?.toString() || "Could not delete product";
  } finally {
    deleting.value = false;
    snackbar.show = true;
  }
}

onMounted(async () => {
  try {
    standalone.value = await IsStandaloneMode();
  } catch {
    standalone.value = false;
  }
  void loadMore();
});
</script>

<style scoped>
.products-search {
  max-width: 320px;
}
.products-row--clickable {
  cursor: pointer;
}
</style>
