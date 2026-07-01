<template>
  <div class="d-flex ga-4 pa-4" style="width: 100%; overflow: hidden">
    <v-card title="Printing Preferences" class="pa-4" min-height="400" min-width="400">
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
          :append-icon="showApiKey ? 'mdi-eye' : 'mdi-eye-off'"
          :type="showApiKey ? 'text' : 'password'"
          @click:append="showApiKey = !showApiKey"
        />
        <v-text-field
          v-model="form.printerID"
          label="Printer ID"
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
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn color="red-darken-4" variant="flat" :loading="saving" @click="saveSettings">
          <v-icon start icon="mdi-content-save" />
          Save Settings
        </v-btn>
      </v-card-actions>
    </v-card>

    <v-card title="Change Password" class="pa-4" min-height="400" min-width="400">
      <v-card-text>
        <v-text-field
          v-model="passwordForm.current"
          label="Current Password"
          variant="outlined"
          density="compact"
          class="mb-2"
          :type="showCurrent ? 'text' : 'password'"
          :append-icon="showCurrent ? 'mdi-eye' : 'mdi-eye-off'"
          @click:append="showCurrent = !showCurrent"
        />
        <v-text-field
          v-model="passwordForm.newPass"
          label="New Password"
          variant="outlined"
          density="compact"
          class="mb-2"
          :type="showNew ? 'text' : 'password'"
          :append-icon="showNew ? 'mdi-eye' : 'mdi-eye-off'"
          @click:append="showNew = !showNew"
        />
        <v-text-field
          v-model="passwordForm.confirm"
          label="Confirm New Password"
          variant="outlined"
          density="compact"
          class="mb-2"
          :type="showConfirm ? 'text' : 'password'"
          :append-icon="showConfirm ? 'mdi-eye' : 'mdi-eye-off'"
          @click:append="showConfirm = !showConfirm"
        />
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn
          color="red-darken-4"
          variant="flat"
          :loading="changingPassword"
          @click="changePassword"
        >
          <v-icon start icon="mdi-lock-reset" />
          Change Password
        </v-btn>
      </v-card-actions>
    </v-card>
  </div>

  <v-snackbar v-model="snackbar.show" :color="snackbar.color" timeout="3000">
    {{ snackbar.message }}
  </v-snackbar>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from "vue";
import { GetAppSettings, UpdateSettings, ChangePassword } from "../../wailsjs/go/main/App";
import { useAuthStore } from "@/stores/auth";

const auth = useAuthStore();

const form = reactive({
  branchName: "",
  apiKey: "",
  printerID: "",
  printerSize: "80mm",
});

const passwordForm = reactive({
  current: "",
  newPass: "",
  confirm: "",
});

const showApiKey = ref(false);
const showCurrent = ref(false);
const showNew = ref(false);
const showConfirm = ref(false);
const saving = ref(false);
const changingPassword = ref(false);

const snackbar = reactive({
  show: false,
  message: "",
  color: "green",
});

function showMessage(message: string, color = "green") {
  snackbar.message = message;
  snackbar.color = color;
  snackbar.show = true;
}

onMounted(async () => {
  try {
    const settings = await GetAppSettings();
    if (settings) {
      form.branchName = settings.branchName ?? "";
      form.apiKey = settings.apiKey ?? "";
      form.printerID = settings.printerID ?? "";
      form.printerSize = settings.printerSize ?? "80mm";
    }
  } catch {
    showMessage("Failed to load settings", "red");
  }
});

async function saveSettings() {
  saving.value = true;
  try {
    await UpdateSettings(form.branchName, form.apiKey, form.printerID, form.printerSize);
    showMessage("Settings saved");
  } catch {
    showMessage("Failed to save settings", "red");
  } finally {
    saving.value = false;
  }
}

async function changePassword() {
  if (!passwordForm.current || !passwordForm.newPass || !passwordForm.confirm) {
    showMessage("Please fill all password fields", "red");
    return;
  }
  if (passwordForm.newPass !== passwordForm.confirm) {
    showMessage("New passwords do not match", "red");
    return;
  }
  if (!auth.currentUser) {
    showMessage("Not logged in", "red");
    return;
  }

  changingPassword.value = true;
  try {
    await ChangePassword(auth.currentUser.username, passwordForm.current, passwordForm.newPass);
    showMessage("Password changed");
    passwordForm.current = "";
    passwordForm.newPass = "";
    passwordForm.confirm = "";
  } catch (e) {
    showMessage(String(e), "red");
  } finally {
    changingPassword.value = false;
  }
}
</script>
