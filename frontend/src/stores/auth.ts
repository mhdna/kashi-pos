import { defineStore } from "pinia";
import { ref, computed } from "vue";
import {
  VerifyPIN,
  VerifyAdminPassword,
  ChangeAdminPassword,
  GetPINLockoutStatus,
} from "../../wailsjs/go/main/App";

interface User {
  id: number;
  username: string;
  role: string;
}

export const useAuthStore = defineStore("auth", () => {
  const currentUser = ref<User | null>(null);
  const isLoggedIn = computed(() => currentUser.value !== null);
  const isAdmin = computed(() => currentUser.value?.role === "admin");

  async function loginWithPIN(pin: string): Promise<{ ok: boolean; retryAfter: number }> {
    try {
      const result = await VerifyPIN(pin);
      currentUser.value = result as User;
      return { ok: true, retryAfter: 0 };
    } catch (error) {
      const match = String(error).match(/locked:(\d+)/);
      return { ok: false, retryAfter: match ? Number(match[1]) : 0 };
    }
  }

  async function loginWithAdminPassword(
    password: string,
  ): Promise<{ ok: boolean; retryAfter: number }> {
    try {
      const result = await VerifyAdminPassword(password);
      currentUser.value = result as User;
      return { ok: true, retryAfter: 0 };
    } catch (error) {
      const match = String(error).match(/locked:(\d+)/);
      return { ok: false, retryAfter: match ? Number(match[1]) : 0 };
    }
  }

  async function getPINLockoutStatus(): Promise<{ locked: boolean; retryAfter: number }> {
    try {
      const result = (await GetPINLockoutStatus()) as { locked: boolean; retryAfter: number };
      return { locked: !!result.locked, retryAfter: Number(result.retryAfter) || 0 };
    } catch {
      return { locked: false, retryAfter: 0 };
    }
  }

  function logout() {
    currentUser.value = null;
  }

  async function changeAdminPassword(
    oldPassword: string,
    newPassword: string,
  ): Promise<{ ok: boolean; error?: string }> {
    try {
      await ChangeAdminPassword(oldPassword, newPassword);
      return { ok: true };
    } catch (error) {
      return { ok: false, error: error instanceof Error ? error.message : String(error) };
    }
  }

  return {
    currentUser,
    isLoggedIn,
    isAdmin,
    loginWithPIN,
    loginWithAdminPassword,
    getPINLockoutStatus,
    logout,
    changeAdminPassword,
  };
});
