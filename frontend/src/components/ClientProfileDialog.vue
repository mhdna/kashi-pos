<template>
  <k-dialog v-model="model" max-width="1100" scrollable>
    <k-card>
      <k-card-title class="d-flex align-center px-4 pt-3 pb-0">
        <span>Client Profile</span>
        <k-spacer />
        <k-btn icon="mdi-close" variant="text" density="comfortable" @click="model = false" />
      </k-card-title>

      <k-progress-linear v-if="loading" indeterminate />

      <k-card-text v-if="!loading && profile" class="pa-4">
        <!-- Row 1: header card + stat cards -->
        <div class="profile-top-row">
          <!-- Client header -->
          <k-sheet class="profile-header-card pa-4" rounded border>
            <div class="d-flex align-center ga-4">
              <InitialsAvatar :name="profile.client.name" color="blue-darken-3" :size="72" />
              <div class="flex-1-1">
                <div class="d-flex align-center ga-2 mb-2">
                  <span class="text-h6 font-weight-bold">{{ profile.client.name }}</span>
                  <k-chip
                    :color="tierColor"
                    size="small"
                    variant="flat"
                    prepend-icon="mdi-star"
                  >{{ tier }} Tier</k-chip>
                </div>
                <table class="profile-meta-table">
                  <tbody>
                    <tr v-if="profile.client.uid">
                      <td class="text-medium-emphasis text-caption pr-4">Client ID</td>
                      <td class="text-body-2">{{ profile.client.uid }}</td>
                    </tr>
                    <tr>
                      <td class="text-medium-emphasis text-caption pr-4">Phone</td>
                      <td class="text-body-2">{{ formatLebanesePhone(profile.client.phone) }}</td>
                    </tr>
                    <tr v-if="profile.homeBranch">
                      <td class="text-medium-emphasis text-caption pr-4">Home branch</td>
                      <td class="text-body-2">{{ profile.homeBranch }}</td>
                    </tr>
                    <tr v-if="profile.firstPurchaseAt">
                      <td class="text-medium-emphasis text-caption pr-4">First purchase</td>
                      <td class="text-body-2">{{ formatDateMedium(profile.firstPurchaseAt) }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </k-sheet>

          <!-- Stat cards (2×2) -->
          <div class="profile-stat-grid">
            <k-sheet
              v-for="stat in statCards"
              :key="stat.label"
              class="profile-stat-card pa-3"
              rounded
              border
            >
              <div class="text-caption text-medium-emphasis text-uppercase mb-1">{{ stat.label }}</div>
              <div class="text-h5 font-weight-bold">{{ stat.value }}</div>
            </k-sheet>
          </div>
        </div>

        <!-- Row 2: category treemap + payment mix -->
        <div class="profile-chart-row mt-4" v-if="hasChartData">
          <!-- Treemap -->
          <k-sheet v-if="profile.categorySpend.length" class="pa-3" rounded border style="flex: 2">
            <div class="text-subtitle-2 mb-2">Spend by Category</div>
            <div class="treemap-container">
              <div
                v-for="family in treemapFamilies"
                :key="family.family"
                class="treemap-family"
                :style="{ flex: family.totalCents }"
              >
                <div
                  class="treemap-family-label text-caption font-weight-medium pa-1"
                  :style="{ background: family.color }"
                >{{ family.family }}</div>
                <div class="treemap-kinds">
                  <div
                    v-for="kind in family.kinds"
                    :key="kind.kind"
                    class="treemap-kind text-caption pa-1"
                    :style="{ flex: kind.totalCents, background: family.kindColor, minHeight: '28px' }"
                    :title="`${kind.kind}: ${formatUsd(kind.totalCents / 100)}`"
                  >{{ kind.kind }}</div>
                </div>
              </div>
            </div>
          </k-sheet>

          <!-- Payment mix donut -->
          <k-sheet v-if="profile.paymentMix.length" class="pa-3" rounded border style="flex: 1">
            <div class="text-subtitle-2 mb-2">Payment Mix</div>
            <div class="donut-container">
              <svg viewBox="0 0 120 120" class="donut-svg">
                <g transform="translate(60,60)">
                  <path
                    v-for="(arc, i) in donutArcs"
                    :key="i"
                    :d="arc.d"
                    :fill="arc.color"
                  />
                </g>
                <!-- hole -->
                <circle cx="60" cy="60" r="28" fill="rgba(var(--k-theme-surface))" />
              </svg>
              <div class="donut-legend mt-2">
                <div v-for="(arc, i) in donutArcs" :key="i" class="d-flex align-center ga-1 text-caption mb-1">
                  <span class="donut-legend-dot" :style="{ background: arc.color }" />
                  {{ arc.name }}
                </div>
              </div>
            </div>
          </k-sheet>
        </div>

        <!-- Row 3: recent purchases + loyalty -->
        <div class="profile-chart-row mt-4">
          <!-- Recent purchases -->
          <k-sheet class="pa-3" rounded border style="flex: 2">
            <div class="text-subtitle-2 mb-2">Recent Purchases</div>
            <k-table density="compact">
              <thead>
                <tr>
                  <th>Date</th>
                  <th>Invoice</th>
                  <th>Branch</th>
                  <th class="text-center">Items</th>
                  <th>Payment</th>
                  <th class="text-end">Total</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="inv in profile.recentInvoices" :key="inv.id">
                  <td class="text-caption">{{ formatDateMedium(inv.createdAt) }}</td>
                  <td class="text-caption">{{ inv.invoiceCode }}</td>
                  <td class="text-caption">{{ inv.branchName }}</td>
                  <td class="text-caption text-center">{{ inv.itemCount }}</td>
                  <td>
                    <k-chip
                      v-for="m in inv.paymentMethods"
                      :key="m"
                      size="x-small"
                      class="mr-1"
                      variant="tonal"
                    >{{ m }}</k-chip>
                  </td>
                  <td class="text-caption text-end font-weight-medium">{{ formatUsd(inv.grandTotal / 100) }}</td>
                </tr>
                <tr v-if="!profile.recentInvoices.length">
                  <td colspan="6" class="text-center text-medium-emphasis py-4">No purchases yet.</td>
                </tr>
              </tbody>
            </k-table>
          </k-sheet>

          <!-- Loyalty progress -->
          <k-sheet class="pa-3" rounded border style="flex: 1">
            <div class="text-subtitle-2 mb-3">Loyalty</div>
            <div class="d-flex justify-space-between text-caption text-medium-emphasis mb-1">
              <span>{{ tier }} Tier</span>
              <span>{{ nextTierName }} Tier</span>
            </div>
            <k-progress-linear
              :model-value="tierProgress"
              :color="tierColor"
              rounded
              height="10"
            />
            <div class="text-caption text-medium-emphasis mt-1">
              <span v-if="pointsToNextTier > 0">
                {{ pointsToNextTier.toLocaleString() }} pts to {{ nextTierName }} Tier
              </span>
              <span v-else class="text-green">Max tier reached</span>
            </div>

            <div class="d-flex ga-4 mt-4">
              <div>
                <div class="text-caption text-medium-emphasis">Lifetime spend</div>
                <div class="text-body-1 font-weight-bold">{{ formatUsd(profile.totalSpentCents / 100) }}</div>
              </div>
              <div>
                <div class="text-caption text-medium-emphasis">Points balance</div>
                <div class="text-body-1 font-weight-bold">{{ profile.pointsRemaining.toLocaleString() }}</div>
              </div>
            </div>
            <div class="d-flex ga-4 mt-2">
              <div>
                <div class="text-caption text-medium-emphasis">Items bought</div>
                <div class="text-body-1 font-weight-bold">{{ profile.itemsBought.toLocaleString() }}</div>
              </div>
              <div>
                <div class="text-caption text-medium-emphasis">Points redeemed</div>
                <div class="text-body-1 font-weight-bold">{{ profile.pointsRedeemed.toLocaleString() }}</div>
              </div>
            </div>
          </k-sheet>
        </div>
      </k-card-text>

      <k-card-text v-else-if="!loading && error" class="text-center text-medium-emphasis pa-8">
        {{ error }}
      </k-card-text>
    </k-card>
  </k-dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { GetClientProfile } from "../../wailsjs/go/main/App";
import InitialsAvatar from "@/components/InitialsAvatar.vue";
import { loyaltyTier, loyaltyTierColor } from "@/lib/client";
import { formatUsd } from "@/lib/format";
import { formatLebanesePhone } from "@/lib/phone";

const model = defineModel<boolean>();

const props = defineProps<{ clientId: number | null }>();

const loading = ref(false);
const error = ref("");
const profile = ref<any>(null);

// Tier thresholds matching loyaltyTier() in client.ts
const TIERS = [
  { name: "None",   min: 0 },
  { name: "Bronze", min: 1 },
  { name: "Silver", min: 200 },
  { name: "Gold",   min: 500 },
];

const tier = computed(() => loyaltyTier(Number(profile.value?.client?.totalLoyaltyPoints ?? 0)));
const tierColor = computed(() => loyaltyTierColor(tier.value));

const nextTierName = computed(() => {
  const idx = TIERS.findIndex((t) => t.name === tier.value);
  return TIERS[idx + 1]?.name ?? tier.value;
});

const pointsToNextTier = computed(() => {
  const idx = TIERS.findIndex((t) => t.name === tier.value);
  const next = TIERS[idx + 1];
  if (!next) return 0;
  return next.min - Number(profile.value?.client?.totalLoyaltyPoints ?? 0);
});

const tierProgress = computed(() => {
  const idx = TIERS.findIndex((t) => t.name === tier.value);
  const current = TIERS[idx];
  const next = TIERS[idx + 1];
  if (!next || !current) return 100;
  const pts = Number(profile.value?.client?.totalLoyaltyPoints ?? 0);
  return Math.min(100, Math.round(((pts - current.min) / (next.min - current.min)) * 100));
});

const statCards = computed(() => {
  if (!profile.value) return [];
  return [
    { label: "Money Spent",      value: formatUsd(profile.value.totalSpentCents / 100) },
    { label: "Items Bought",     value: profile.value.itemsBought.toLocaleString() },
    { label: "Points Remaining", value: profile.value.pointsRemaining.toLocaleString() },
    { label: "Points Redeemed",  value: profile.value.pointsRedeemed.toLocaleString() },
  ];
});

// Treemap colours — one palette per family
const FAMILY_COLORS = [
  { bg: "#3949ab", kindBg: "#283593" },
  { bg: "#6ab04c", kindBg: "#4a8a2a" },
  { bg: "#e55039", kindBg: "#b03a28" },
  { bg: "#d4ac0d", kindBg: "#a68c09" },
  { bg: "#8e44ad", kindBg: "#6c3483" },
  { bg: "#16a085", kindBg: "#0e6655" },
  { bg: "#e67e22", kindBg: "#ca6f1e" },
];

const treemapFamilies = computed(() =>
  (profile.value?.categorySpend ?? []).map((f: any, i: number) => {
    const palette = FAMILY_COLORS[i % FAMILY_COLORS.length]!;
    return { ...f, color: palette.bg, kindColor: palette.kindBg };
  }),
);

// Donut chart arcs
const DONUT_COLORS = ["#5c6bc0", "#8bc34a", "#78909c", "#ffa726", "#ec407a", "#26c6da"];

const donutArcs = computed(() => {
  const items: any[] = profile.value?.paymentMix ?? [];
  if (!items.length) return [];
  const total = items.reduce((s: number, r: any) => s + Number(r.totalCents), 0);
  if (!total) return [];
  let angle = -Math.PI / 2;
  return items.map((item: any, i: number) => {
    const slice = (Number(item.totalCents) / total) * 2 * Math.PI;
    const x1 = Math.cos(angle) * 48;
    const y1 = Math.sin(angle) * 48;
    const x2 = Math.cos(angle + slice) * 48;
    const y2 = Math.sin(angle + slice) * 48;
    const large = slice > Math.PI ? 1 : 0;
    const d = `M0,0 L${x1},${y1} A48,48,0,${large},1,${x2},${y2} Z`;
    const color = DONUT_COLORS[i % DONUT_COLORS.length];
    angle += slice;
    return { name: item.name, d, color };
  });
});

const hasChartData = computed(
  () => profile.value?.categorySpend?.length || profile.value?.paymentMix?.length,
);

function formatDateMedium(value?: string): string {
  if (!value) return "";
  const normalized = value.includes("T") ? value : value.replace(" ", "T");
  const hasZ = /(?:Z|[+-]\d{2}:\d{2})$/.test(normalized);
  const d = new Date(hasZ ? normalized : `${normalized}Z`);
  if (isNaN(d.getTime())) return value.slice(0, 10);
  return d.toLocaleDateString(undefined, { day: "2-digit", month: "short", year: "numeric" });
}

async function loadProfile() {
  if (!props.clientId) return;
  loading.value = true;
  error.value = "";
  profile.value = null;
  try {
    profile.value = await GetClientProfile(props.clientId);
  } catch (e: any) {
    error.value = e?.toString() ?? "Could not load client profile";
  } finally {
    loading.value = false;
  }
}

watch(
  () => [model.value, props.clientId] as const,
  ([open]) => { if (open) loadProfile(); },
);
</script>

<style scoped>
.profile-top-row {
  display: flex;
  gap: 12px;
  align-items: stretch;
}

.profile-header-card {
  flex: 3;
  min-width: 0;
}

.profile-meta-table {
  border-collapse: collapse;
  width: 100%;
}

.profile-meta-table td {
  padding-block: 2px;
  vertical-align: middle;
}

.profile-stat-grid {
  flex: 2;
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}

.profile-stat-card {
  display: flex;
  flex-direction: column;
  justify-content: center;
}

.profile-chart-row {
  display: flex;
  gap: 12px;
  align-items: flex-start;
}

/* Treemap */
.treemap-container {
  display: flex;
  height: 220px;
  gap: 2px;
  border-radius: 4px;
  overflow: hidden;
}

.treemap-family {
  display: flex;
  flex-direction: column;
  min-width: 0;
  overflow: hidden;
}

.treemap-family-label {
  flex: 0 0 auto;
  color: #fff;
  font-size: 0.7rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.treemap-kinds {
  display: flex;
  flex-direction: column;
  flex: 1;
  gap: 1px;
}

.treemap-kind {
  color: rgba(255, 255, 255, 0.8);
  font-size: 0.65rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: default;
}

/* Donut */
.donut-container {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.donut-svg {
  width: 140px;
  height: 140px;
}

.donut-legend-dot {
  display: inline-block;
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}
</style>
