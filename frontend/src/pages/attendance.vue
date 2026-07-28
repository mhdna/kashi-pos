<template>
  <div class="attendance-page pa-4">
    <v-alert v-if="!status.reachable" type="error" variant="tonal" class="mb-4"
        ><template #title>{{ status.message || "Attendance machine is unreachable" }}</template
      >Check the Akuvox network cable and IP address. The app will retry every 60 seconds.</v-alert
    >
    <v-alert v-else type="success" variant="tonal" density="compact" class="mb-4"
      >Akuvox connected · syncing every 60 seconds</v-alert
    >
    <div class="d-flex align-center mb-4 ga-3">
      <v-text-field
        v-if="auth.isAdmin"
        v-model="selectedDate"
        type="date"
        label="Day"
        variant="outlined"
        density="compact"
        hide-details
        style="max-width: 190px"
        @change="refresh"
      /><v-text-field
        v-if="auth.isAdmin"
        v-model="selectedMonth"
        type="month"
        label="History month"
        variant="outlined"
        density="compact"
        hide-details
        style="max-width: 190px"
        @change="loadHistory"
      /><v-spacer /><v-btn
        v-if="auth.isAdmin"
        variant="tonal"
        class="me-2"
        prepend-icon="mdi-download"
        @click="exportMonth"
        >Export month</v-btn
      ><v-btn icon="mdi-refresh" variant="text" :loading="loading" @click="refresh" />
    </div>
    <v-card>
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
          class="attendance-item mb-2"
          rounded="lg"
        >
          <template #prepend>
            <v-avatar color="primary" variant="tonal" class="me-3">
              {{ item.employeeName?.charAt(0)?.toUpperCase() || "?" }}
            </v-avatar>
          </template>
          <v-list-item-title class="font-weight-bold">{{ item.employeeName }}</v-list-item-title>
          <v-list-item-subtitle>Attendance ID: {{ item.userId || "—" }}</v-list-item-subtitle>
          <div class="attendance-item__body">
            <div class="timeline">
              <span
                v-for="event in item.events"
                :key="event.id"
                class="take"
                :title="event.time"
                :style="{ left: `${minutePosition(event.time)}%` }"
              />
              <span class="text-caption">{{ item.events.map((e: any) => e.time).join(" · ") || "No events" }}</span>
            </div>
            <div class="attendance-times">
              <span><span class="text-caption text-medium-emphasis">First in</span>{{ item.events[0]?.time || "—" }}</span>
              <span><span class="text-caption text-medium-emphasis">Last out</span>{{ item.events.length > 1 ? item.events[item.events.length - 1].time : "—" }}</span>
            </div>
            <v-btn
              size="small"
              variant="tonal"
              prepend-icon="mdi-message-draw"
              @click="openComplaint(item)"
              >Request change</v-btn
            >
          </div>
        </v-list-item>
        <v-list-item v-if="!rows.length && !loading" class="text-medium-emphasis">
          No attendance records for this day.
        </v-list-item>
      </v-list>
    </v-card>
    <v-card v-if="auth.isAdmin && complaints.length" class="mt-4"
      ><v-card-title>Change requests</v-card-title
      ><v-list
        ><v-list-item
          v-for="item in complaints"
          :key="item.id"
          :title="`${item.date} · ${item.requestedTime}`"
          :subtitle="item.note"
          ><template #append
            ><v-checkbox-btn
              :model-value="item.approved"
              color="primary"
              @update:model-value="approve(item, $event)" /></template></v-list-item></v-list
    ></v-card>
    <v-dialog v-model="complaintDialog" max-width="440"
      ><v-card
        ><v-card-title>Request attendance change</v-card-title
        ><v-card-text
          ><v-text-field
            v-model="complaintTime"
            type="time"
            label="Correct time"
            variant="outlined"
            density="compact"
            class="mb-2" /><v-textarea
            v-model="complaintNote"
            label="Note"
            placeholder="Explain what should be corrected"
            variant="outlined"
            rows="3" /></v-card-text
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
import { onMounted, onUnmounted, ref } from "vue";
import {
  ApproveAttendanceComplaint,
  ExportAttendanceMonth,
  ListAttendanceComplaints,
  ListAttendance,
  ListUsers,
  SubmitAttendanceComplaint,
  SyncAttendance,
} from "../../wailsjs/go/main/App";
import { useAuthStore } from "@/stores/auth";
const auth = useAuthStore();
const today = new Date().toISOString().slice(0, 10);
const selectedDate = ref(today);
const selectedMonth = ref(today.slice(0, 7));
const loading = ref(false);
const rows = ref<any[]>([]);
const complaints = ref<any[]>([]);
const complaintDialog = ref(false);
const complaintTime = ref("");
const complaintNote = ref("");
const complaintEmployee = ref<any>(null);
const savingComplaint = ref(false);
const status = ref<any>({ reachable: true, message: "" });
let timer: ReturnType<typeof setInterval>;
async function refresh() {
  loading.value = true;
  try {
    status.value = await SyncAttendance(selectedDate.value);
    const [events, users] = await Promise.all([ListAttendance(selectedDate.value), ListUsers()]);
    const employees = users.filter((u: any) => u.attendanceUserId);
    const byId = new Map(employees.map((u: any) => [u.id, u]));
    const grouped = new Map<number, any>();
    employees.forEach((u: any) =>
      grouped.set(u.id, {
        employeeId: u.id,
        employeeName: u.username,
        userId: u.attendanceUserId,
        events: [],
      }),
    );
    events.forEach((e: any) => {
      const u = byId.get(e.employeeId);
      if (!grouped.has(e.employeeId))
        grouped.set(e.employeeId, {
          employeeId: e.employeeId,
          employeeName: u?.username || e.employeeName,
          userId: u?.attendanceUserId,
          events: [],
        });
      grouped.get(e.employeeId).events.push(e);
    });
    rows.value = [...grouped.values()];
    if (auth.isAdmin) await loadHistory();
  } catch {
    status.value = { reachable: false, message: "Attendance sync failed" };
  } finally {
    loading.value = false;
  }
}
async function loadHistory() {
  try {
    complaints.value = await ListAttendanceComplaints(selectedMonth.value);
  } catch {}
}
function minutePosition(t: string) {
  const parts = t.split(":");
  const h = Number(parts[0] || 0);
  const m = Number(parts[1] || 0);
  return ((h * 60 + m) / (24 * 60)) * 100;
}
function openComplaint(row: any) {
  complaintEmployee.value = row;
  complaintTime.value = row.events[0]?.time || "";
  complaintNote.value = "";
  complaintDialog.value = true;
}
async function submitComplaint() {
  if (!complaintNote.value.trim()) return;
  savingComplaint.value = true;
  try {
    await SubmitAttendanceComplaint(
      complaintEmployee.value.employeeId,
      selectedDate.value,
      complaintTime.value,
      complaintNote.value,
    );
    complaintDialog.value = false;
    await loadHistory();
  } finally {
    savingComplaint.value = false;
  }
}
async function approve(item: any, value: boolean) {
  await ApproveAttendanceComplaint(item.id, value);
  item.approved = value;
}
async function exportMonth() {
  const content = await ExportAttendanceMonth(selectedMonth.value);
  const blob = new Blob([content], { type: "application/json" });
  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = `attendance-${selectedMonth.value}.json`;
  a.click();
  URL.revokeObjectURL(a.href);
}
onMounted(() => {
  refresh();
  timer = setInterval(refresh, 60_000);
});
onUnmounted(() => clearInterval(timer));
</script>
<style scoped>
.attendance-page {
  height: 100%;
  overflow: auto;
}
.timeline {
  position: relative;
  flex: 1 1 240px;
  min-width: 200px;
  height: 28px;
  padding-top: 5px;
}
.attendance-item {
  border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
}
.attendance-item__body {
  display: flex;
  align-items: center;
  gap: 16px;
  width: 100%;
  margin-top: 10px;
}
.attendance-times {
  display: flex;
  gap: 18px;
  flex: 0 0 auto;
}
.attendance-times > span {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 54px;
  font-weight: 600;
}
.take {
  position: absolute;
  top: 0;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #2e7d32;
}
.timeline .text-caption {
  white-space: nowrap;
  margin-left: 8px;
}
@media (max-width: 720px) {
  .attendance-item__body {
    align-items: stretch;
    flex-direction: column;
    gap: 8px;
  }
  .attendance-times {
    justify-content: space-between;
  }
  .attendance-item__body .v-btn {
    align-self: flex-start;
  }
}
</style>
