<template>
  <v-container
    class="mx-auto pa-4 pb-8 d-flex flex-column"
    flat
    elevation="8"
    max-width="448"
    min-width="400"
    rounded="lg"
  >
    <div
      class="text-center mb-6 text-white d-flex flex-column align-center"
      style="
        flex: 0 0 auto;
        font-size: var(--kashi-font-size-brand);
        font-weight: 900;
        font-family: Belleza, Arial, sans-serif;
      "
    >
      <v-img :src="logo" width="190" :class="{ 'login-logo--dark': currentTheme === 'dark' }" />
      <!-- <div>KBS</div> -->
    </div>
    <!-- <div style="font-family: Belleza">Demo Store</div> -->
    <v-form @submit.prevent="handleLogin">
      <v-text-field
        density="compact"
        placeholder="Username"
        :rules="usernameRules"
        bg-color="white"
        :prepend-inner-icon="'mdi-account'"
        class="mb-2"
        variant="outlined"
        v-model="username"
      >
      </v-text-field>
      <v-text-field
        density="compact"
        placeholder="Enter your password"
        :type="visible ? 'text' : 'password'"
        bg-color="white"
        :rules="passwordRules"
        class="mb-2 text-white"
        variant="outlined"
        v-model="password"
        :prepend-inner-icon="'mdi-lock-outline'"
        :append-inner-icon="visible ? 'mdi-eye-off' : 'mdi-eye'"
        @click:append-inner="visible = !visible"
      >
      </v-text-field>

      <v-btn
        block
        class="mb-8"
        size="large"
        rounded="lg"
        type="submit"
        :loading="loading"
        :disabled="!username.trim() || !password.trim()"
      >
        Log In
      </v-btn>
    </v-form>
    <div class="alert-wrapper">
      <v-alert
        v-if="error"
        color="red-darken-4"
        variant="tonal"
        density="compact"
        closable
        @click:close="error = ''"
      >
        {{ error }}
      </v-alert>
    </div>
  </v-container>
</template>

<script setup lang="ts">
import { ref } from "vue";
import logo from "@/assets/appicon.webp";

import { useRouter } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { useTheme } from "vuetify";

const router = useRouter();
const auth = useAuthStore();
const theme = useTheme();
const currentTheme = theme.name;

const username = ref("");
const password = ref("");
const visible = ref(false);
const error = ref("");
const loading = ref(false);

const usernameRules = [(v: string) => !!v || "Username is required"];
const passwordRules = [(v: string) => !!v || "Password is required"];

async function handleLogin() {
  error.value = "";
  if (!username.value || !password.value) {
    error.value = "Please enter username and password";
    return;
  }

  loading.value = true;
  const start = Date.now();
  try {
    const ok = await auth.login(username.value, password.value);
    if (ok) {
      router.push("/home");
    } else {
      error.value = "Invalid username or password";
    }
  } finally {
    const elapsed = Date.now() - start;
    if (elapsed < 300) await new Promise((r) => setTimeout(r, 300 - elapsed));
    loading.value = false;
  }
}
</script>

<style scoped>
/*.v-btn.v-btn--disabled {
  background-color: rgb(139, 28, 28) !important;
  opacity: 0.5;
}*/
.alert-wrapper {
  min-height: 48px; /* or whatever the alert's natural height is */
}
.login-logo--dark {
  filter: invert(1);
}
</style>
