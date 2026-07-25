<template>
  <div class="settings-page">
    <!-- <div class="settings-heading"> -->
    <!--   <div> -->
    <!--     <div class="text-overline text-primary font-weight-bold">Workspace configuration</div> -->
    <!--     <h1 class="text-h4 font-weight-bold mb-1">Settings</h1> -->
    <!--     <p class="text-body-2 text-medium-emphasis mb-0"> -->
    <!--       Configure receipts, customer displays, security, and user access. -->
    <!--     </p> -->
    <!--   </div> -->
    <!--   <v-icon icon="mdi-cog-outline" size="48" class="settings-heading-icon" /> -->
    <!-- </div> -->

    <!-- <v-alert v-if="settingsLoadError" type="warning" variant="tonal" class="mb-4"> -->
    <!--   Settings could not be loaded from the database. The form is using safe defaults. -->
    <!-- </v-alert> -->

    <v-tabs v-model="tab" class="settings-tabs mb-5" color="primary" grow>
      <v-tab value="general" prepend-icon="mdi-printer-wireless">Printing &amp; Screen</v-tab>
      <v-tab value="admin" prepend-icon="mdi-shield-account-outline">Passwords &amp; Users</v-tab>
    </v-tabs>
    <v-progress-linear v-if="settingsLoading" indeterminate color="primary" class="mb-4" />

    <v-window v-model="tab">
      <v-window-item value="general">
        <div class="settings-grid">
          <v-card class="settings-card">
            <v-card-title class="d-flex align-center ga-3">
              <v-avatar color="primary" variant="tonal" size="36">
                <v-icon icon="mdi-printer-outline" size="20" />
              </v-avatar>
              <div>
                <div class="text-subtitle-1 font-weight-bold">Printing preferences</div>
                <div class="text-caption text-medium-emphasis">
                  Set up your receipt printer and layout.
                </div>
              </div>
            </v-card-title>
            <v-card-text>
              <v-text-field
                v-model="form.branchName"
                label="Branch Name"
                variant="outlined"
                density="compact"
                class="mb-2"
              />
              <v-text-field
                v-model="form.apiKey"
                label="API Key"
                variant="outlined"
                density="compact"
                class="mb-2"
                :append-inner-icon="showApiKey ? 'mdi-eye' : 'mdi-eye-off'"
                :type="showApiKey ? 'text' : 'password'"
                @click:append-inner="showApiKey = !showApiKey"
              />
              <v-select
                v-model="form.receiptPrinter"
                :items="printers"
                label="Receipt Printer"
                variant="outlined"
                density="compact"
                class="mb-2"
              />
              <v-select
                v-model="form.printerSize"
                :items="['58mm', '80mm']"
                label="Printing Size"
                variant="outlined"
                density="compact"
                class="mb-2"
              />
              <v-row dense>
                <v-col cols="6">
                  <v-text-field
                    v-model.number="form.receiptWidth"
                    label="Width (mm)"
                    type="number"
                    variant="outlined"
                    density="compact"
                  />
                </v-col>
                <v-col cols="6">
                  <v-text-field
                    v-model.number="form.receiptHeight"
                    label="Height (mm)"
                    type="number"
                    variant="outlined"
                    density="compact"
                  />
                </v-col>
              </v-row>
              <v-text-field
                v-model.number="form.taxRate"
                label="VAT rate"
                hint="Use 0.11 for 11%, or 0 to disable VAT."
                persistent-hint
                type="number"
                step="0.01"
                min="0"
                max="1"
                variant="outlined"
                density="compact"
              />
              <v-select
                v-model="form.roundingMode"
                :items="roundingModes"
                label="Money rounding"
                hint="Applied to discounts and VAT in cents."
                persistent-hint
                variant="outlined"
                density="compact"
                class="mb-2"
              />
              <v-select
                v-model.number="form.salesInventoryId"
                :items="inventories"
                item-title="name"
                item-value="id"
                label="Sales inventory"
                hint="Stock is deducted from this inventory."
                persistent-hint
                variant="outlined"
                density="compact"
                class="mb-2"
              />
              <v-switch v-model="form.receiptEnabled" label="Print receipts after checkout" color="primary" hide-details />
              <v-switch
                :model-value="animationsEnabled"
                label="Animations"
                color="primary"
                hint="Turn off decorative motion for a simpler, faster display."
                persistent-hint
                hide-details
                @update:model-value="(value) => setAnimationsEnabled(value ?? false)"
              />
            </v-card-text>
            <v-card-actions class="px-4 pb-4">
              <v-spacer />
              <v-btn color="primary" variant="flat" :loading="saving" @click="saveSettings">
                <v-icon start icon="mdi-content-save-outline" /> Save changes
              </v-btn>
            </v-card-actions>
          </v-card>

          <v-card class="settings-card">
            <v-card-title class="d-flex align-center ga-3">
              <v-avatar color="primary" variant="tonal" size="36">
                <v-icon icon="mdi-monitor-dashboard" size="20" />
              </v-avatar>
              <div>
                <div class="text-subtitle-1 font-weight-bold">Customer screen</div>
                <div class="text-caption text-medium-emphasis">
                  Choose the display used at checkout.
                </div>
              </div>
            </v-card-title>
            <v-card-text>
              <v-select
                v-model="form.screenPort"
                :items="serialPorts"
                label="Serial Port"
                hint="Leave empty if no customer display is connected."
                persistent-hint
                variant="outlined"
                density="compact"
              />
            </v-card-text>
            <v-card-actions class="px-4 pb-4">
              <v-spacer />
              <v-btn color="primary" variant="flat" :loading="saving" @click="saveSettings">
                <v-icon start icon="mdi-content-save-outline" /> Save changes
              </v-btn>
            </v-card-actions>
          </v-card>
        </div>
      </v-window-item>

      <v-window-item value="admin">
        <div class="settings-grid settings-grid--admin">
          <v-form @submit.prevent="changePassword">
            <v-card class="settings-card">
              <v-card-title class="d-flex align-center ga-3">
                <v-avatar color="primary" variant="tonal" size="36">
                  <v-icon icon="mdi-lock-reset" size="20" />
                </v-avatar>
                <div>
                  <div class="text-subtitle-1 font-weight-bold">Change password</div>
                  <div class="text-caption text-medium-emphasis">Keep your account secure.</div>
                </div>
              </v-card-title>
              <v-card-text>
                <v-text-field
                  v-model="passwordForm.current"
                  label="Current Password"
                  variant="outlined"
                  density="compact"
                  class="mb-2"
                  :type="showPw ? 'text' : 'password'"
                  :append-inner-icon="showPw ? 'mdi-eye' : 'mdi-eye-off'"
                  @click:append-inner="showPw = !showPw"
                />
                <v-text-field
                  v-model="passwordForm.newPass"
                  label="New Password"
                  variant="outlined"
                  density="compact"
                  class="mb-2"
                  :type="showPw ? 'text' : 'password'"
                />
                <v-text-field
                  v-model="passwordForm.confirm"
                  label="Confirm New Password"
                  variant="outlined"
                  density="compact"
                  class="mb-2"
                  :type="showPw ? 'text' : 'password'"
                  :rules="[(v) => v === passwordForm.newPass || 'Passwords do not match']"
                />
              </v-card-text>
              <v-card-actions>
                <v-spacer />
                <v-btn color="primary" variant="flat" type="submit" :loading="changingPassword">
                  <v-icon start icon="mdi-lock-reset" /> Change Password
                </v-btn>
              </v-card-actions>
            </v-card>
          </v-form>

          <v-card class="settings-card settings-card--wide">
            <v-card-title class="d-flex align-center ga-3">
              <v-avatar color="primary" variant="tonal" size="36">
                <v-icon icon="mdi-account-group-outline" size="20" />
              </v-avatar>
              <div>
                <div class="text-subtitle-1 font-weight-bold">Users</div>
                <div class="text-caption text-medium-emphasis">
                  Manage who can access the point of sale.
                </div>
              </div>
              <v-spacer />
              <v-btn color="primary" size="small" @click="openUserDialog()">
                <v-icon start icon="mdi-plus" /> Add user
              </v-btn>
            </v-card-title>
            <v-data-table
              :headers="userHeaders"
              :items="users"
              :items-per-page="10"
              density="comfortable"
              hover
            >
              <template #item.active="{ item }">
                <v-chip :color="item.active ? 'green' : 'red'" size="small" variant="tonal">
                  {{ item.active ? "Active" : "Disabled" }}
                </v-chip>
              </template>
              <template #item.actions="{ item }">
                <v-btn icon size="small" variant="text" @click="openUserDialog(item)">
                  <v-icon icon="mdi-pencil-outline" size="18" />
                </v-btn>
                <v-btn icon size="small" variant="text" @click="deleteUserConfirm(item)">
                  <v-icon icon="mdi-delete-outline" size="18" color="red" />
                </v-btn>
              </template>
            </v-data-table>
          </v-card>
        </div>
      </v-window-item>
    </v-window>

    <v-dialog v-model="userDialog" max-width="400">
      <v-card>
        <v-card-title>{{ editingUser ? "Edit User" : "New User" }}</v-card-title>
        <v-card-text>
          <v-text-field
            v-model="userForm.username"
            label="Username"
            variant="outlined"
            density="compact"
            class="mb-2"
          />
          <v-text-field
            v-model="userForm.password"
            label="Password"
            variant="outlined"
            density="compact"
            class="mb-2"
            :type="showUpw ? 'text' : 'password'"
            :append-inner-icon="showUpw ? 'mdi-eye' : 'mdi-eye-off'"
            @click:append-inner="showUpw = !showUpw"
            :placeholder="editingUser ? 'Leave blank to keep current' : ''"
          />
          <v-select
            v-model="userForm.roleId"
            :items="roles"
            item-title="name"
            item-value="id"
            label="Role"
            variant="outlined"
            density="compact"
            class="mb-2"
          />
        </v-card-text>
        <v-card-actions>
          <v-btn variant="text" @click="userDialog = false">Cancel</v-btn>
          <v-spacer />
          <v-btn
            color="primary"
            variant="flat"
            :disabled="!userForm.username"
            :loading="savingUser"
            @click="saveUser"
            >Save</v-btn
          >
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog v-model="deleteDialog" max-width="400">
      <v-card>
        <v-card-title>Delete User</v-card-title>
        <v-card-text>Are you sure you want to delete {{ deleteTarget?.username }}?</v-card-text>
        <v-card-actions>
          <v-btn variant="text" @click="deleteDialog = false">Cancel</v-btn>
          <v-spacer />
          <v-btn color="red" variant="flat" @click="deleteUser">Delete</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-snackbar v-model="snackbar.show" :color="snackbar.color" timeout="3000" location="top">
      {{ snackbar.text }}
    </v-snackbar>
  </div>
</template>

<script lang="ts" setup>
import { ref, reactive, onMounted } from "vue";
import {
  GetAppSettings,
  UpdateSettings,
  ChangePassword,
  ListUsers,
  CreateUser,
  UpdateUser,
  DeleteUser,
  ListRoles,
  ListSerialPorts,
  ListPrinters,
  ListInventories,
} from "../../wailsjs/go/main/App";
import { useAuthStore } from "@/stores/auth";
import { animationsEnabled, setAnimationsEnabled } from "@/lib/display";

const auth = useAuthStore();
const tab = ref("general");

const form = reactive({
  branchName: "",
  apiKey: "",
  printerID: "",
  printerSize: "80mm",
  receiptPrinter: "",
  screenPort: "",
  receiptWidth: 80,
  receiptHeight: 297,
  taxRate: 0,
  roundingMode: "half_up",
  receiptEnabled: false,
  salesInventoryId: 0,
});
const roundingModes = [
  { title: "Half up (standard)", value: "half_up" },
  { title: "Always down", value: "down" },
  { title: "Always up", value: "up" },
];
const inventories = ref<any[]>([]);
const serialPorts = ref<string[]>([]);
const printers = ref<string[]>([]);
const saving = ref(false);
const showApiKey = ref(false);
const settingsLoading = ref(true);
const settingsLoadError = ref(false);
const passwordForm = reactive({ current: "", newPass: "", confirm: "" });
const showPw = ref(false);
const changingPassword = ref(false);

const users = ref<any[]>([]);
const roles = ref<any[]>([]);
const userHeaders = [
  { title: "Username", key: "username" },
  { title: "Role", key: "role" },
  { title: "Status", key: "active" },
  { title: "", key: "actions", sortable: false, align: "end" as const },
];

const userDialog = ref(false);
const editingUser = ref<any>(null);
const userForm = reactive({ username: "", password: "", roleId: 1 });
const showUpw = ref(false);
const savingUser = ref(false);

const deleteDialog = ref(false);
const deleteTarget = ref<any>(null);

const snackbar = reactive({ show: false, text: "", color: "success" });

function notify(text: string, color = "success") {
  snackbar.text = text;
  snackbar.color = color;
  snackbar.show = true;
}

async function loadSettings() {
  settingsLoading.value = true;
  settingsLoadError.value = false;
  try {
    const s = await GetAppSettings();
    if (s)
      Object.assign(form, {
        branchName: s.branchName ?? "",
        apiKey: s.apiKey ?? "",
        printerID: s.printerID ?? "",
        printerSize: s.printerSize ?? "80mm",
        receiptPrinter: s.receiptPrinter ?? "",
        screenPort: s.screenPort ?? "",
        receiptWidth: Number(s.receiptWidth ?? 80),
        receiptHeight: Number(s.receiptHeight ?? 297),
        taxRate: Number(s.taxRate ?? 0),
        roundingMode: s.roundingMode ?? "half_up",
        receiptEnabled: Boolean(s.receiptEnabled),
        salesInventoryId: Number(s.salesInventoryId ?? 0),
      });
  } catch {
    settingsLoadError.value = true;
  } finally {
    settingsLoading.value = false;
  }
}

onMounted(async () => {
  await loadSettings();
  try {
    serialPorts.value = await ListSerialPorts();
  } catch {}
  try {
    printers.value = await ListPrinters();
  } catch {}
  try {
    inventories.value = await ListInventories();
  } catch {}
  await loadUsers();
  try {
    roles.value = await ListRoles();
  } catch {}
});

async function saveSettings() {
  saving.value = true;
  try {
    const receiptWidth = Number(form.receiptWidth);
    const receiptHeight = Number(form.receiptHeight);
    const taxRate = Number(form.taxRate);
    await UpdateSettings(
      form.branchName,
      form.apiKey,
      form.printerID,
      form.printerSize,
      form.receiptPrinter,
      form.screenPort,
      Number.isFinite(receiptWidth) && receiptWidth > 0 ? Math.round(receiptWidth) : 80,
      Number.isFinite(receiptHeight) && receiptHeight > 0 ? Math.round(receiptHeight) : 297,
      Number.isFinite(taxRate) ? Math.min(Math.max(taxRate, 0), 1) : 0,
      roundingModes.some((m) => m.value === form.roundingMode) ? form.roundingMode : "half_up",
      Boolean(form.receiptEnabled),
      Number(form.salesInventoryId) > 0 ? Number(form.salesInventoryId) : 0,
    );
    notify("Settings saved");
  } catch {
    notify("Failed to save", "red");
  }
  saving.value = false;
}

async function changePassword() {
  if (passwordForm.newPass !== passwordForm.confirm) {
    notify("Passwords do not match", "red");
    return;
  }
  changingPassword.value = true;
  try {
    await ChangePassword(auth.currentUser!.username, passwordForm.current, passwordForm.newPass);
    notify("Password changed");
    passwordForm.current = "";
    passwordForm.newPass = "";
    passwordForm.confirm = "";
  } catch {
    notify("Failed to change password", "red");
  }
  changingPassword.value = false;
}

async function loadUsers() {
  try {
    users.value = await ListUsers();
  } catch {}
}

function openUserDialog(user?: any) {
  editingUser.value = user ?? null;
  if (user) {
    userForm.username = user.username;
    userForm.password = "";
    userForm.roleId = user.roleId;
  } else {
    userForm.username = "";
    userForm.password = "";
    userForm.roleId = roles.value[0]?.id ?? 1;
  }
  userDialog.value = true;
}

async function saveUser() {
  savingUser.value = true;
  try {
    if (editingUser.value) {
      await UpdateUser(
        editingUser.value.id,
        userForm.username,
        userForm.password,
        userForm.roleId,
        1,
      );
    } else {
      await CreateUser(userForm.username, userForm.password, userForm.roleId);
    }
    notify(editingUser.value ? "User updated" : "User created");
    userDialog.value = false;
    await loadUsers();
  } catch {
    notify("Failed to save user", "red");
  }
  savingUser.value = false;
}

function deleteUserConfirm(user: any) {
  deleteTarget.value = user;
  deleteDialog.value = true;
}

async function deleteUser() {
  try {
    await DeleteUser(deleteTarget.value.id);
    notify("User deleted");
    deleteDialog.value = false;
    await loadUsers();
  } catch {
    notify("Failed to delete user", "red");
  }
}
</script>

<style scoped>
.settings-page {
  box-sizing: border-box;
  height: 100%;
  overflow-y: auto;
  padding: 24px;
}

.settings-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  max-width: 1100px;
  margin: 0 auto 24px;
}

.settings-heading-icon {
  opacity: 0.16;
}

.settings-tabs,
.settings-grid {
  max-width: 1100px;
  margin-inline: auto;
}

.settings-tabs {
  border-bottom: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
}

.settings-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  align-items: start;
  gap: 16px;
}

.settings-grid--admin {
  grid-template-columns: minmax(280px, 0.8fr) minmax(0, 1.6fr);
}

.settings-card {
  min-width: 0;
  overflow: hidden;
  border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
}

.settings-card :deep(.v-card-title) {
  padding: 20px 20px 8px;
}

.settings-card :deep(.v-card-text) {
  padding: 16px 20px 4px;
}

.settings-card :deep(.v-data-table-footer) {
  padding-inline: 20px;
}

@media (max-width: 900px) {
  .settings-grid,
  .settings-grid--admin {
    grid-template-columns: 1fr;
  }

  .settings-page {
    padding: 16px;
  }
}
</style>
