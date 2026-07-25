import { defineStore } from "pinia";
import { ref, computed } from "vue";
import { VerifyLogin, ChangePassword } from "../../wailsjs/go/main/App";

interface User {
  id: number;
  username: string;
  role: string;
}

export const useAuthStore = defineStore("auth", () => {
  const currentUser = ref<User | null>(null);
  const isLoggedIn = computed(() => currentUser.value !== null);
  const isAdmin = computed(() => currentUser.value?.role === "admin");

  async function login(username: string, password: string): Promise<boolean> {
    try {
      const result = await VerifyLogin(username, password);
      currentUser.value = result as User;
      return true;
    } catch {
      return false;
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

  return { currentUser, isLoggedIn, isAdmin, login, logout, changePassword };
});
