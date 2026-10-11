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
      /><v-spacer /><v-btn
        v-if="auth.isLoggedIn"
        variant="tonal"
        prepend-icon="mdi-message-draw"
        class="me-1"
        :disabled="!rows.length"
        @click="openRequestDialog"
        >Request change</v-btn
      ><v-menu v-if="auth.isLoggedIn" :close-on-content-click="false">
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
              <v-list-item-subtitle>{{ requestSummary(item) }}</v-list-item-subtitle>
              <template v-if="managerApproval" #append>
                <v-btn size="small" variant="text" color="error" @click="askManager({ kind: 'review', request: item, approved: false })">Reject</v-btn>
                <v-btn size="small" variant="tonal" color="primary" @click="askManager({ kind: 'review', request: item, approved: true })">Approve</v-btn>
              </template>
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
          >{{ presentRows.length }} employees</span
        >
      </v-card-title>
      <v-progress-linear v-if="loading" indeterminate color="primary" />
      <v-list lines="two" class="attendance-list pa-2">
        <v-list-item
          v-for="item in presentRows"
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
              </div>
              <div class="attendance-summary">
                <span class="text-caption text-medium-emphasis">{{ item.events.length }} event{{ item.events.length === 1 ? "" : "s" }}</span>
              </div>
            </div>
          </div>
        </v-list-item>
        <v-list-item v-if="!presentRows.length && !loading" class="attendance-empty text-medium-emphasis">
          No attendance records for this day.
        </v-list-item>
      </v-list>
    </v-card>
    <v-dialog v-model="complaintDialog" max-width="440"
      ><v-card
        ><v-card-title>Request attendance change</v-card-title
        ><v-card-text
          ><v-select
            v-model="complaintAction"
            :items="complaintActionOptions"
            item-title="title"
            item-value="value"
            label="Action"
            variant="outlined"
            density="compact"
            hide-details
            class="attendance-action-field mb-2"
          /><v-select
            :model-value="complaintEmployee?.employeeId ?? null"
            :items="complaintPeople"
            item-title="employeeName"
            item-value="employeeId"
            label="Person"
            prepend-inner-icon="mdi-account-outline"
            variant="outlined"
            density="compact"
            hide-details
            class="mb-2"
            @update:model-value="selectComplaintPerson"
          /><v-select
            v-if="complaintAction !== 'add'"
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
          /><div class="attendance-correction-row mb-2">
            <div v-if="complaintAction !== 'delete'" class="attendance-time-field">
              <span class="attendance-time-field__label">{{ complaintAction === "add" ? "Time" : "Corrected time" }}</span>
              <div class="attendance-time-field__controls">
                <NumberPadMenu v-model="complaintHour" :max="23" :max-length="2" preserve-string suffix="h" aria-label="Corrected hour" />
                <span class="attendance-time-field__separator">:</span>
                <NumberPadMenu v-model="complaintMinute" :max="59" :max-length="2" preserve-string suffix="m" aria-label="Corrected minute" />
              </div>
            </div><v-select
              v-if="complaintAction !== 'delete'"
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
            v-if="managerApproval && complaintAction === 'add'"
            variant="tonal"
            title="Record it now with the manager password, without waiting for approval"
            :disabled="!complaintEmployee"
            @click="askManager({ kind: 'add' })"
            >Add now (manager)</v-btn
          ><v-btn
            color="primary"
            :loading="savingComplaint"
            :disabled="!complaintEmployee || (complaintAction !== 'add' && !complaintEventId)"
            @click="submitComplaint"
            >Submit request</v-btn
          ></v-card-actions
        ></v-card
      ></v-dialog
    >
    <v-dialog v-model="managerDialog.show" max-width="380">
      <v-card>
        <v-card-title>Manager approval</v-card-title>
        <v-card-text>
          <p class="mb-3">{{ managerDialogText }}</p>
          <v-text-field
            v-model="managerDialog.password"
            type="password"
            label="Manager password"
            variant="outlined"
            density="compact"
            autofocus
            :error-messages="managerDialog.error"
            @keyup.enter="confirmManager"
          />
        </v-card-text>
        <v-card-actions>
          <v-btn variant="text" @click="managerDialog.show = false">Cancel</v-btn><v-spacer />
          <v-btn color="primary" :loading="managerDialog.saving" @click="confirmManager">Confirm</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>
<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from "vue";
import {
  AddAttendanceWithManagerApproval,
  ApproveAttendanceComplaint,
  ExportAttendanceMonth,
  GetAttendanceManagerApprovalEnabled,
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
import { useAttendanceStatus } from "@/composables/useAttendanceStatus";
import { EventsOn } from "../../wailsjs/runtime/runtime";
const auth = useAuthStore();
const today = new Date().toISOString().slice(0, 10);
const selectedDate = ref(today);
const selectedMonth = ref(today.slice(0, 7));
const loading = ref(false);
const rows = ref<any[]>([]);
// The day's list shows only people who punched; rows keeps everyone with an
// attendance ID so a missed punch can still be requested for them.
const presentRows = computed(() => rows.value.filter((r: any) => r.events.length));
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
// The card combines two sources. The live probe (one TCP check a second)
// catches a pulled cable at once. The last door-log sync catches a device
// that answers but fails the request. Either one being down shows down.
const syncStatus = ref<any>({ reachable: true, message: "" });
const { status: liveStatus } = useAttendanceStatus();
const status = computed(() => (liveStatus.value.reachable ? syncStatus.value : liveStatus.value));
// Once the machine is back, sync again so the card doesn't stay red on a
// failure from while it was down.
watch(
  () => liveStatus.value.reachable,
  (up, wasUp) => {
    if (up && !wasUp && !loading.value) void refresh();
  },
);
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
    if (syncDevice) syncStatus.value = await SyncAttendance(selectedDate.value);
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
    syncStatus.value = { reachable: false, message: "Attendance sync failed" };
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
// The action is picked first and decides who can be picked. "add" is for a
// punch the device never recorded, so anyone with an attendance ID; replace
// and delete act on one of the person's punches, so only people who punched
// that day -- and only offered when someone did.
const complaintActionOptions = computed(() => {
  const options = [{ title: "Add missing punch", value: "add" }];
  if (presentRows.value.length) {
    options.push({ title: "Replace", value: "replace" }, { title: "Delete", value: "delete" });
  }
  return options;
});
const complaintPeople = computed(() => (complaintAction.value === "add" ? rows.value : presentRows.value));
// Switching from add to replace/delete: someone with no punches that day
// can't stay picked; someone with punches gets their first one selected.
watch(complaintAction, (action) => {
  if (action === "add") return;
  if (!complaintEmployee.value?.events?.length) {
    complaintEmployee.value = null;
    complaintEventId.value = null;
  } else if (!complaintEventId.value) {
    selectFirstComplaintEvent();
  }
});
function requestSummary(item: any) {
  if (item.action === "add") return `Add ${item.requestedType} ${item.requestedTime}`;
  if (item.action === "delete") return `Delete ${item.originalTime}`;
  return `Replace ${item.originalTime} with ${item.requestedType} ${item.requestedTime}`;
}
// One "Request change" button by Requests; the action and then the person
// are picked in the dialog, and the punch/time follow whoever is picked.
function openRequestDialog() {
  complaintEmployee.value = null;
  complaintEventId.value = null;
  complaintAction.value = "add";
  complaintType.value = "IN";
  setComplaintTime(new Date().toTimeString().slice(0, 5));
  complaintDialog.value = true;
}
function selectComplaintPerson(employeeId: number | null) {
  complaintEmployee.value = rows.value.find((r: any) => r.employeeId === employeeId) ?? null;
  complaintEventId.value = null;
  if (complaintAction.value !== "add") selectFirstComplaintEvent();
}
// Replace/delete start on the person's first punch of the day, its time
// and IN/OUT filled in.
function selectFirstComplaintEvent() {
  const first = complaintEmployee.value?.events?.[0];
  complaintEventId.value = first?.id ?? null;
  if (!first) return;
  setComplaintTime(first.time);
  complaintType.value = eventKind(first, 0);
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
  if (!complaintEmployee.value) return;
  if (complaintAction.value !== "add" && !complaintEventId.value) return;
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
    syncStatus.value = { reachable: syncStatus.value.reachable, message: error instanceof Error ? error.message : "Attendance export failed" };
  }
}
// Manager approval on the till (on by default, switchable here or from
// kashi): a manager decides requests, or records a missed punch at once,
// with the manager password. Off, requests wait for kashi.
const managerApproval = ref(false);
async function loadManagerApproval() {
  try {
    managerApproval.value = Boolean(await GetAttendanceManagerApprovalEnabled());
  } catch {
    managerApproval.value = false;
  }
}
type ManagerAction = { kind: "review"; request: any; approved: boolean } | { kind: "add" };
const managerDialog = reactive<{ show: boolean; password: string; error: string; saving: boolean; action: ManagerAction | null }>({
  show: false,
  password: "",
  error: "",
  saving: false,
  action: null,
});
const managerDialogText = computed(() => {
  const action = managerDialog.action;
  if (!action) return "";
  if (action.kind === "add") {
    return `Record ${complaintType.value} ${correctedTime()} on ${selectedDate.value} for ${complaintEmployee.value?.employeeName ?? "this person"}.`;
  }
  return `${action.approved ? "Approve" : "Reject"}: ${action.request.employeeName} · ${action.request.date} -- ${requestSummary(action.request)}.`;
});
function askManager(action: ManagerAction) {
  Object.assign(managerDialog, { show: true, password: "", error: "", saving: false, action });
}
async function confirmManager() {
  const action = managerDialog.action;
  if (!action || !managerDialog.password) return;
  managerDialog.saving = true;
  managerDialog.error = "";
  try {
    if (action.kind === "add") {
      if (!complaintEmployee.value) throw new Error("Pick a person first.");
      await AddAttendanceWithManagerApproval(
        complaintEmployee.value.employeeId,
        selectedDate.value,
        correctedTime(),
        complaintType.value,
        managerDialog.password,
      );
      complaintDialog.value = false;
    } else {
      await ApproveAttendanceComplaint(action.request.id, action.approved, managerDialog.password);
    }
    managerDialog.show = false;
    await Promise.all([loadHistory(), refresh(false)]);
  } catch (error) {
    managerDialog.error = error instanceof Error ? error.message : String(error);
  } finally {
    managerDialog.saving = false;
  }
}
let offSettings: (() => void) | undefined;
onMounted(() => {
  loadAttendanceOptions();
  loadManagerApproval();
  offSettings = EventsOn("settings_changed", () => {
    void loadManagerApproval();
  });
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
  offSettings?.();
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
/* Flex rather than v-list-item's own grid: inside the two-column list grid,
   the nested grid sized the row before the punches wrapped, so a day with
   many punches spilled onto the card below. As a flex row the card grows
   with them. */
.attendance-item {
  border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
  min-height: 180px;
  margin: 0;
  display: flex;
  align-items: flex-start;
  height: auto;
}
.attendance-item :deep(.v-list-item__content) {
  flex: 1 1 auto;
  min-width: 0;
  overflow: visible;
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
