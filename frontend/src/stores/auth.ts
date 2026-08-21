import { defineStore } from "pinia";
import { ref, computed } from "vue";
import { VerifyPIN, ChangePassword, GetPINLockoutStatus } from "../../wailsjs/go/main/App";

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

  async function changePassword(oldPassword: string, newPassword: string): Promise<boolean> {
    try {
      await ChangePassword(currentUser.value!.username, oldPassword, newPassword);
      return true;
    } catch {
      return false;
    }
  }

  return { currentUser, isLoggedIn, isAdmin, loginWithPIN, getPINLockoutStatus, logout, changePassword };
});
