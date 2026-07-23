<template>
  <div class="pa-4" style="width: 100%; overflow-x: hidden">
    <v-tabs v-model="tab" class="mb-4">
      <v-tab value="printing">Printing</v-tab>
      <v-tab value="screen">Screen</v-tab>
      <v-tab value="password">Password</v-tab>
      <v-tab value="users">Users</v-tab>
    </v-tabs>

    <v-window v-model="tab">
      <v-window-item value="printing">
        <v-card title="Printing Preferences" class="pa-4" max-width="500">
          <v-card-text>
            <v-text-field v-model="form.branchName" label="Branch Name" variant="outlined" density="compact" class="mb-2" />
            <v-text-field v-model="form.apiKey" label="API Key" variant="outlined" density="compact" class="mb-2"
              :append-inner-icon="showApiKey ? 'mdi-eye' : 'mdi-eye-off'" :type="showApiKey ? 'text' : 'password'"
              @click:append-inner="showApiKey = !showApiKey" />
            <v-select v-model="form.receiptPrinter" :items="printers" label="Receipt Printer" variant="outlined" density="compact" class="mb-2" />
            <v-select v-model="form.printerSize" :items="['58mm', '80mm']" label="Printing Size" variant="outlined" density="compact" class="mb-2" />
            <v-text-field v-model.number="form.receiptWidth" label="Receipt Width (mm)" type="number" variant="outlined" density="compact" class="mb-2" />
            <v-text-field v-model.number="form.receiptHeight" label="Receipt Height (mm)" type="number" variant="outlined" density="compact" class="mb-2" />
          </v-card-text>
          <v-card-actions>
            <v-spacer />
            <v-btn color="primary" variant="flat" :loading="saving" @click="saveSettings">
              <v-icon start icon="mdi-content-save" /> Save
            </v-btn>
          </v-card-actions>
        </v-card>
      </v-window-item>

      <v-window-item value="screen">
        <v-card title="Customer Screen" class="pa-4" max-width="500">
          <v-card-text>
            <v-select v-model="form.screenPort" :items="serialPorts" label="Serial Port" variant="outlined" density="compact" class="mb-2" />
          </v-card-text>
          <v-card-actions>
            <v-spacer />
            <v-btn color="primary" variant="flat" :loading="saving" @click="saveSettings">
              <v-icon start icon="mdi-content-save" /> Save
            </v-btn>
          </v-card-actions>
        </v-card>
      </v-window-item>

      <v-window-item value="password">
        <v-form @submit.prevent="changePassword">
        <v-card title="Change Password" class="pa-4" max-width="500">
          <v-card-text>
            <v-text-field v-model="passwordForm.current" label="Current Password" variant="outlined" density="compact" class="mb-2"
              :type="showPw ? 'text' : 'password'" :append-inner-icon="showPw ? 'mdi-eye' : 'mdi-eye-off'"
              @click:append-inner="showPw = !showPw" />
            <v-text-field v-model="passwordForm.newPass" label="New Password" variant="outlined" density="compact" class="mb-2"
              :type="showPw ? 'text' : 'password'" />
            <v-text-field v-model="passwordForm.confirm" label="Confirm New Password" variant="outlined" density="compact" class="mb-2"
              :type="showPw ? 'text' : 'password'"
              :rules="[v => v === passwordForm.newPass || 'Passwords do not match']" />
          </v-card-text>
          <v-card-actions>
            <v-spacer />
            <v-btn color="primary" variant="flat" type="submit" :loading="changingPassword">
              <v-icon start icon="mdi-lock-reset" /> Change Password
            </v-btn>
          </v-card-actions>
        </v-card>
        </v-form>
      </v-window-item>

      <v-window-item value="users">
        <v-card class="pa-4" max-width="700">
          <v-card-title class="d-flex align-center">
            Users
            <v-spacer />
            <v-btn color="primary" size="small" @click="openUserDialog()">
              <v-icon start icon="mdi-plus" /> Add User
            </v-btn>
          </v-card-title>
          <v-data-table :headers="userHeaders" :items="users" :items-per-page="10" density="compact" hover>
            <template #item.active="{ item }">
              <v-chip :color="item.active ? 'green' : 'red'" size="small" variant="tonal">{{ item.active ? 'Active' : 'Disabled' }}</v-chip>
            </template>
            <template #item.actions="{ item }">
              <v-btn icon size="small" variant="text" @click="openUserDialog(item)">
                <v-icon icon="mdi-pencil" size="18" />
              </v-btn>
              <v-btn icon size="small" variant="text" @click="deleteUserConfirm(item)">
                <v-icon icon="mdi-delete" size="18" color="red" />
              </v-btn>
            </template>
          </v-data-table>
        </v-card>
      </v-window-item>
    </v-window>

    <v-dialog v-model="userDialog" max-width="400">
      <v-card>
        <v-card-title>{{ editingUser ? 'Edit User' : 'New User' }}</v-card-title>
        <v-card-text>
          <v-text-field v-model="userForm.username" label="Username" variant="outlined" density="compact" class="mb-2" />
          <v-text-field v-model="userForm.password" label="Password" variant="outlined" density="compact" class="mb-2"
            :type="showUpw ? 'text' : 'password'" :append-inner-icon="showUpw ? 'mdi-eye' : 'mdi-eye-off'"
            @click:append-inner="showUpw = !showUpw"
            :placeholder="editingUser ? 'Leave blank to keep current' : ''" />
          <v-select v-model="userForm.roleId" :items="roles" item-title="name" item-value="id" label="Role" variant="outlined" density="compact" class="mb-2" />
        </v-card-text>
        <v-card-actions>
          <v-btn variant="text" @click="userDialog = false">Cancel</v-btn>
          <v-spacer />
          <v-btn color="primary" variant="flat" :disabled="!userForm.username" :loading="savingUser" @click="saveUser">Save</v-btn>
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
  GetAppSettings, UpdateSettings, ChangePassword,
  ListUsers, CreateUser, UpdateUser, DeleteUser, ListRoles,
  ListSerialPorts, ListPrinters,
} from "../../wailsjs/go/main/App";
import { useAuthStore } from "@/stores/auth";

const auth = useAuthStore();
const tab = ref("printing");

const form = reactive({
  branchName: "", apiKey: "", printerSize: "80mm",
  receiptPrinter: "", screenPort: "", receiptWidth: 80, receiptHeight: 297,
});
const serialPorts = ref<string[]>([]);
const printers = ref<string[]>([]);
const saving = ref(false);
const showApiKey = ref(false);

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

onMounted(async () => {
  try {
    const s = await GetAppSettings();
    if (s) Object.assign(form, {
      branchName: s.branchName ?? "", apiKey: s.apiKey ?? "",
      printerSize: s.printerSize ?? "80mm",
      receiptPrinter: s.receiptPrinter ?? "", screenPort: s.screenPort ?? "",
      receiptWidth: s.receiptWidth ?? 80, receiptHeight: s.receiptHeight ?? 297,
    });
  } catch {}
  try { serialPorts.value = await ListSerialPorts(); } catch {}
  try { printers.value = await ListPrinters(); } catch {}
  await loadUsers();
  try { roles.value = await ListRoles(); } catch {}
});

async function saveSettings() {
  saving.value = true;
  try {
    await UpdateSettings(
      form.branchName, form.apiKey, "", form.printerSize,
      form.receiptPrinter, form.screenPort, form.receiptWidth, form.receiptHeight,
    );
    notify("Settings saved");
  } catch { notify("Failed to save", "red"); }
  saving.value = false;
}

async function changePassword() {
  if (passwordForm.newPass !== passwordForm.confirm) {
    notify("Passwords do not match", "red"); return;
  }
  changingPassword.value = true;
  try {
    await ChangePassword(auth.currentUser!.username, passwordForm.current, passwordForm.newPass);
    notify("Password changed");
    passwordForm.current = ""; passwordForm.newPass = ""; passwordForm.confirm = "";
  } catch { notify("Failed to change password", "red"); }
  changingPassword.value = false;
}

async function loadUsers() {
  try { users.value = await ListUsers(); } catch {}
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
      await UpdateUser(editingUser.value.id, userForm.username, userForm.password, userForm.roleId, 1);
    } else {
      await CreateUser(userForm.username, userForm.password, userForm.roleId);
    }
    notify(editingUser.value ? "User updated" : "User created");
    userDialog.value = false;
    await loadUsers();
  } catch { notify("Failed to save user", "red"); }
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
  } catch { notify("Failed to delete user", "red"); }
}
</script>
