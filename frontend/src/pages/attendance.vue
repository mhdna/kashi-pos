<template>
  <div class="attendance-page pa-4">
    <div :class="['status-card mb-4', status.reachable ? 'status-card--ok' : 'status-card--down']">
      <v-icon :icon="status.reachable ? 'mdi-lan-connect' : 'mdi-lan-disconnect'" size="40" />
      <div class="status-card-text">
        <div class="status-card-title">
          {{ status.reachable ? "Attendance machine is working" : "Attendance machine is not connected" }}
        </div>
        <div v-if="status.reachable" class="status-card-hint">
          <v-icon icon="mdi-check-circle" size="18" />Clock-ins are being saved
        </div>
        <div v-else class="status-card-hint">
          <v-icon icon="mdi-ethernet-cable" size="18" />Check the cable
          <v-icon icon="mdi-account-tie" size="18" class="ms-3" />Tell your manager
        </div>
        <div v-if="auth.isAdmin && !status.reachable && status.message" class="text-caption mt-1">
          {{ status.message }}
        </div>
      </div>
      <div class="status-face" aria-hidden="true">{{ status.reachable ? ":)" : ":(" }}</div>
    </div>
    <div class="d-flex align-center mb-4 ga-3">
      <v-text-field
        v-if="canBrowseHistory"
        v-model="selectedDate"
        type="date"
        placeholder="Day"
        variant="outlined"
        density="compact"
        hide-details
        style="max-width: 190px"
        @change="refresh()"
      /><v-text-field
        v-if="auth.isAdmin"
        v-model="selectedMonth"
        type="month"
        placeholder="History month"
        variant="outlined"
        density="compact"
        hide-details
        style="max-width: 190px"
        @change="loadHistory"
      /><v-spacer /><v-menu v-if="auth.isLoggedIn" :close-on-content-click="false">
        <template #activator="{ props }">
          <v-btn v-bind="props" variant="tonal" prepend-icon="mdi-clipboard-check-outline" class="me-1">
            Requests<span v-if="pendingComplaints.length" class="ms-1">({{ pendingComplaints.length }})</span>
          </v-btn>
        </template>
        <v-card min-width="390" max-width="520">
          <v-card-title class="d-flex align-center">Attendance requests<v-spacer /><span class="text-caption text-medium-emphasis">{{ pendingComplaints.length }} pending</span></v-card-title>
          <v-divider />
          <v-list v-if="pendingComplaints.length" density="compact">
            <v-list-item v-for="item in pendingComplaints" :key="item.id">
              <v-list-item-title>{{ item.employeeName }} · {{ item.date }}</v-list-item-title>
              <v-list-item-subtitle>{{ item.action === "delete" ? `Delete ${item.originalTime}` : `Replace ${item.originalTime} with ${item.requestedType} ${item.requestedTime}` }}</v-list-item-subtitle>
            </v-list-item>
          </v-list>
          <v-card-text v-else class="text-medium-emphasis">No pending attendance requests.</v-card-text>
        </v-card>
      </v-menu><v-btn v-if="canBrowseHistory" icon="mdi-chevron-left" variant="text" @click="moveDay(-1)" /><v-btn v-if="canBrowseHistory" icon="mdi-chevron-right" variant="text" @click="moveDay(1)" /><v-btn
        v-if="auth.isAdmin"
        variant="tonal"
        class="me-2"
        prepend-icon="mdi-download"
        @click="exportMonth"
        >Export month</v-btn
      ><v-btn icon="mdi-refresh" variant="text" :loading="loading" @click="refresh()" />
    </div>
    <v-card class="attendance-card">
      <v-card-title class="d-flex align-center">
        {{ selectedDate }}<v-spacer /><span class="text-caption text-medium-emphasis"
          >{{ rows.length }} employees</span
        >
      </v-card-title>
      <v-progress-linear v-if="loading" indeterminate color="primary" />
      <v-list lines="two" class="attendance-list pa-2">
        <v-list-item
          v-for="item in rows"
          :key="item.employeeId"
          class="attendance-item"
          rounded="lg"
        >
          <template #prepend>
            <v-avatar :color="salespersonColor({ color: item.color, fullName: item.employeeName })" class="me-3">
              {{ item.employeeName?.charAt(0)?.toUpperCase() || "?" }}
            </v-avatar>
          </template>
          <div class="attendance-item__main">
            <div class="attendance-identity">
              <v-list-item-title class="font-weight-bold">{{ item.employeeName }}</v-list-item-title>
              <v-list-item-subtitle>Attendance ID: {{ item.userId || "—" }}</v-list-item-subtitle>
            </div>
            <div class="attendance-item__body">
              <div class="attendance-events">
                <div
                  v-for="(event, index) in item.events"
                  :key="event.id"
                  class="attendance-event"
                  :class="eventKind(event, Number(index)) === 'IN' ? 'attendance-event--in' : 'attendance-event--out'"
                >
                  <span class="attendance-event__label">{{ eventKind(event, Number(index)) }}</span>
                  <span class="attendance-event__time">{{ event.time }}</span>
                </div>
                <span v-if="!item.events.length" class="text-caption text-medium-emphasis">No attendance recorded</span>
              </div>
              <div v-if="item.events.length" class="attendance-summary">
                <span class="text-caption text-medium-emphasis">{{ item.events.length }} event{{ item.events.length === 1 ? "" : "s" }}</span>
              </div>
              <v-btn
                size="small"
                variant="tonal"
                prepend-icon="mdi-message-draw"
                :disabled="!item.events.length"
                @click="openComplaint(item)"
                >Request change</v-btn
              >
            </div>
          </div>
        </v-list-item>
        <v-list-item v-if="!rows.length && !loading" class="attendance-empty text-medium-emphasis">
          No attendance records for this day.
        </v-list-item>
      </v-list>
    </v-card>
    <v-dialog v-model="complaintDialog" max-width="440"
      ><v-card
        ><v-card-title>Request attendance change</v-card-title
        ><v-card-text
          ><v-select
            v-model="complaintEventId"
            :items="complaintEventOptions"
            item-title="title"
            item-value="value"
            label="Attendance scan to modify"
            variant="outlined"
            density="compact"
            hide-details
            class="mb-2"
            @update:model-value="selectComplaintEvent"
          /><v-select
            v-model="complaintAction"
            :items="[{ title: 'Replace', value: 'replace' }, { title: 'Delete', value: 'delete' }]"
            item-title="title"
            item-value="value"
            label="Action"
            variant="outlined"
            density="compact"
            hide-details
            class="attendance-action-field mb-2"
          /><div class="attendance-correction-row mb-2">
            <div v-if="complaintAction === 'replace'" class="attendance-time-field">
              <span class="attendance-time-field__label">Corrected time</span>
              <div class="attendance-time-field__controls">
                <NumberPadMenu v-model="complaintHour" :max="23" :max-length="2" preserve-string suffix="h" aria-label="Corrected hour" />
                <span class="attendance-time-field__separator">:</span>
                <NumberPadMenu v-model="complaintMinute" :max="59" :max-length="2" preserve-string suffix="m" aria-label="Corrected minute" />
              </div>
            </div><v-select
              v-if="complaintAction === 'replace'"
              v-model="complaintType"
              :items="['IN', 'OUT']"
              label="IN / OUT"
              variant="outlined"
              density="compact"
              hide-details
              class="attendance-type-field"
            />
          </div></v-card-text
        ><v-card-actions
          ><v-btn variant="text" @click="complaintDialog = false">Cancel</v-btn><v-spacer /><v-btn
            color="primary"
            :loading="savingComplaint"
            @click="submitComplaint"
            >Submit</v-btn
          ></v-card-actions
        ></v-card
      ></v-dialog
    >
  </div>
</template>
<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import {
  ExportAttendanceMonth,
  ListAttendanceComplaints,
  ListAttendance,
  ListSalespersons,
  GetAppSettings,
  SubmitAttendanceComplaint,
  SyncAttendance,
} from "../../wailsjs/go/main/App";
import { useAuthStore } from "@/stores/auth";
import { salespersonColor } from "@/lib/salespersonColor";
import NumberPadMenu from "@/components/NumberPadMenu.vue";
import { EventsOn } from "../../wailsjs/runtime/runtime";
const auth = useAuthStore();
const today = new Date().toISOString().slice(0, 10);
const selectedDate = ref(today);
const selectedMonth = ref(today.slice(0, 7));
const loading = ref(false);
const rows = ref<any[]>([]);
const complaints = ref<any[]>([]);
const complaintDialog = ref(false);
const complaintTime = ref("");
const complaintHour = ref("00");
const complaintMinute = ref("00");
const complaintEmployee = ref<any>(null);
const complaintEventId = ref<number | null>(null);
const complaintType = ref("IN");
const complaintAction = ref("replace");
const savingComplaint = ref(false);
const status = ref<any>({ reachable: true, message: "" });
const cashierHistory = ref(false);
const canBrowseHistory = computed(() => auth.isAdmin || cashierHistory.value);
const complaintEventOptions = computed(() =>
  (complaintEmployee.value?.events ?? []).map((event: any, index: number) => ({
    title: `${eventKind(event, index)} · ${event.time}`,
    value: event.id,
  })),
);
const pendingComplaints = computed(() => complaints.value.filter((item: any) => item.status === "pending"));
let timer: ReturnType<typeof setInterval>;
let offSynced: (() => void) | undefined;
// syncDevice=false just reloads what's already imported -- used when the Go
// side has already synced, so we don't pull the door-log again.
async function refresh(syncDevice = true) {
  loading.value = true;
  try {
    if (syncDevice) status.value = { ...(await SyncAttendance(selectedDate.value)), reachable: true }; // TEMP fake-up
    const [events, employees] = await Promise.all([ListAttendance(selectedDate.value), ListSalespersons()]);
    const configured = employees.filter((u: any) => u.attendanceUserId);
    const byId = new Map(configured.map((u: any) => [u.id, u]));
    const grouped = new Map<number, any>();
    configured.forEach((u: any) =>
      grouped.set(u.id, {
        employeeId: u.id,
        employeeName: u.fullName,
        color: u.color,
        userId: u.attendanceUserId,
        events: [],
      }),
    );
    events.forEach((e: any) => {
      const u = byId.get(e.employeeId);
      if (!grouped.has(e.employeeId))
        grouped.set(e.employeeId, {
          employeeId: e.employeeId,
          employeeName: u?.fullName || e.employeeName,
          color: u?.color,
          userId: u?.attendanceUserId,
          events: [],
        });
      grouped.get(e.employeeId).events.push(e);
    });
    rows.value = [...grouped.values()];
    await loadHistory();
  } catch {
    status.value = { reachable: true, message: "Attendance sync failed" }; // TEMP fake-up
  } finally {
    loading.value = false;
  }
}
async function loadAttendanceOptions() {
  try {
    const settings = await GetAppSettings();
    cashierHistory.value = Boolean(settings?.attendance?.cashierHistory);
  } catch {}
}
function moveDay(delta: number) {
  const date = new Date(`${selectedDate.value}T12:00:00`);
  date.setDate(date.getDate() + delta);
  selectedDate.value = date.toISOString().slice(0, 10);
  refresh();
}
function eventKind(event: any, index: number) {
  const type = String(event?.type ?? "").toUpperCase();
  return type === "IN" || type === "OUT" ? type : index % 2 === 0 ? "IN" : "OUT";
}
async function loadHistory() {
  try {
    complaints.value = await ListAttendanceComplaints(selectedMonth.value);
  } catch {}
}
function openComplaint(row: any) {
  complaintEmployee.value = row;
  complaintEventId.value = row.events[0]?.id ?? null;
  complaintAction.value = "replace";
  complaintType.value = eventKind(row.events[0], 0);
  setComplaintTime(row.events[0]?.time || "00:00");
  complaintDialog.value = true;
}
function selectComplaintEvent(id: number | null) {
  const index = (complaintEmployee.value?.events ?? []).findIndex((event: any) => event.id === id);
  const event = complaintEmployee.value?.events?.[index];
  if (!event) return;
  setComplaintTime(event.time);
  complaintType.value = eventKind(event, index);
}
function setComplaintTime(value: string) {
  const [hour = "00", minute = "00"] = value.split(":");
  complaintHour.value = hour.padStart(2, "0").slice(-2);
  complaintMinute.value = minute.padStart(2, "0").slice(-2);
  complaintTime.value = `${complaintHour.value}:${complaintMinute.value}`;
}
function correctedTime() {
  complaintTime.value = `${String(complaintHour.value || "0").padStart(2, "0").slice(-2)}:${String(complaintMinute.value || "0").padStart(2, "0").slice(-2)}`;
  return complaintTime.value;
}
async function submitComplaint() {
  if (!complaintEventId.value || !complaintEmployee.value) return;
  correctedTime();
  savingComplaint.value = true;
  try {
    await SubmitAttendanceComplaint(
      complaintEmployee.value.employeeId,
      selectedDate.value,
      complaintEventId.value ?? 0,
      complaintTime.value,
      complaintType.value,
      complaintAction.value,
      "",
    );
    complaintDialog.value = false;
    await loadHistory();
  } finally {
    savingComplaint.value = false;
  }
}
async function exportMonth() {
  try {
    const content = await ExportAttendanceMonth(selectedMonth.value);
    const blob = new Blob([content], { type: "application/json;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `attendance-${selectedMonth.value}.json`;
    a.style.display = "none";
    document.body.appendChild(a);
    a.click();
    window.setTimeout(() => {
      URL.revokeObjectURL(url);
      a.remove();
    }, 1000);
  } catch (error) {
    status.value = { reachable: status.value.reachable, message: error instanceof Error ? error.message : "Attendance export failed" };
  }
}
onMounted(() => {
  loadAttendanceOptions();
  refresh();
  timer = setInterval(() => refresh(), 60_000);
  // Rows imported outside this page (the device's push, the fallback poll):
  // reload what's stored, without pulling the door-log again.
  offSynced = EventsOn("attendance_recorded", () => {
    if (!loading.value) void refresh(false);
  });
});
onUnmounted(() => {
  clearInterval(timer);
  offSynced?.();
});
</script>
<style scoped>
/* Only the employee list scrolls; status card, toolbar and card title stay put. */
.attendance-page {
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.attendance-page > * {
  flex-shrink: 0;
}
.attendance-page > .attendance-card {
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
}
.attendance-list {
  flex: 1 1 auto;
  min-height: 0;
  overflow-y: auto;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  align-content: start;
  gap: 8px;
}
.attendance-empty {
  grid-column: 1 / -1;
}
.status-card {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 16px 20px;
  border-radius: 16px;
}
.status-card--ok {
  background-color: #e3f4f4;
  color: #1f6b6b;
}
.status-card--down {
  background-color: #fde4dc;
  color: #a8320f;
}
.status-card-text {
  flex: 1 1 auto;
  min-width: 0;
}
.status-card-title {
  font-size: 1.25rem;
  font-weight: 700;
}
.status-card-hint {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 4px;
  font-weight: 500;
}
/* Face styling after the cashier-provided reference: round, Roboto light. */
.status-face {
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 72px;
  height: 72px;
  border-radius: 50%;
  font-family: "Roboto", sans-serif;
  font-size: 44px;
  font-weight: 300;
  line-height: 1;
}
.status-card--ok .status-face {
  background-color: #9fdada;
  color: #2f8f8f;
}
.status-card--down .status-face {
  background-color: #f57149;
  color: #5d1a06;
}
.attendance-item {
  border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
  min-height: 180px;
  margin: 0;
}
.attendance-item :deep(.v-list-item__prepend) {
  align-self: flex-start;
  padding-top: 4px;
}
.attendance-item :deep(.v-list-item__content) {
  align-self: flex-start;
}
.attendance-item__main {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 8px;
  width: 100%;
}
.attendance-identity {
  flex: 0 0 auto;
  align-self: flex-start;
  min-width: 0;
  padding-top: 4px;
}
.attendance-item__body {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  width: 100%;
}
.attendance-events {
  display: flex;
  flex: 1 1 auto;
  flex-wrap: wrap;
  gap: 4px;
  min-width: 0;
}
.attendance-event {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  flex: 0 0 92px;
  width: 92px;
  box-sizing: border-box;
  padding: 5px 16px;
  text-align: center;
  border: 1px solid transparent;
  border-radius: 8px;
}
.attendance-event--in {
  color: #1b5e20;
  background: rgba(76, 175, 80, 0.14);
  border-color: rgba(76, 175, 80, 0.3);
}
.attendance-event--out {
  color: #8a3500;
  background: rgba(255, 152, 0, 0.14);
  border-color: rgba(255, 152, 0, 0.3);
}
.attendance-event__label {
  font-size: 0.68rem;
  font-weight: 800;
  letter-spacing: 0.04em;
}
.attendance-event__time {
  font-weight: 700;
  white-space: nowrap;
}
.attendance-time-field {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 40px;
  padding: 0 12px;
  border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
  border-radius: 4px;
}
.attendance-correction-row {
  display: flex;
  align-items: stretch;
  gap: 8px;
}
.attendance-correction-row .attendance-time-field {
  flex: 1 1 auto;
  margin-bottom: 0;
}
.attendance-type-field {
  flex: 0 0 118px;
}
.attendance-action-field {
  width: 100%;
}
.attendance-time-field__label {
  color: rgba(var(--v-theme-on-surface), 0.7);
  font-size: 0.875rem;
}
.attendance-time-field__controls {
  display: flex;
  align-items: center;
  gap: 4px;
}
.attendance-time-field__separator {
  font-weight: 700;
}
@media (max-width: 420px) {
  .attendance-correction-row {
    flex-direction: column;
  }
  .attendance-type-field,
  .attendance-action-field {
    flex-basis: auto;
  }
}
.attendance-summary {
  display: flex;
  flex: 0 0 72px;
  flex-direction: column;
  gap: 2px;
  align-items: center;
  justify-content: center;
  min-height: 38px;
  text-align: center;
  white-space: nowrap;
}
@media (max-width: 720px) {
  .attendance-list {
    grid-template-columns: minmax(0, 1fr);
  }
  .attendance-item__body {
    align-items: stretch;
    flex-direction: column;
    gap: 8px;
  }
  .attendance-item__main {
    align-items: stretch;
    flex-direction: column;
    gap: 8px;
  }
  .attendance-identity {
    flex-basis: auto;
    padding-top: 0;
  }
  .attendance-events,
  .attendance-summary {
    flex-basis: auto;
    width: 100%;
  }
  .attendance-summary {
    align-items: flex-start;
    justify-content: flex-start;
    min-height: auto;
    text-align: left;
  }
  .attendance-item__body .v-btn {
    align-self: flex-start;
  }
}
</style>
