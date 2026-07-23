/**
 * router/index.ts
 *
 * Manual routes for ./src/pages/*.vue
 */
import { createRouter, createWebHashHistory } from "vue-router";

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    {
      path: "/auth",
      component: () => import("@/layouts/AuthLayout.vue"),
      children: [
        { path: "/login", component: () => import("@/pages/login.vue") },
      ],
    },
    {
      path: "/",
      component: () => import("@/layouts/DefaultLayout.vue"),
      children: [
        { path: "home", component: () => import("@/pages/home.vue") },
        { path: "settings", component: () => import("@/pages/settings.vue") },
        {
          path: "attendance",
          component: () => import("@/pages/attendance.vue"),
        },
        {
          path: "invoices-list",
          component: () => import("@/pages/invoices-list.vue"),
        },
        { path: "inventory", component: () => import("@/pages/inventory.vue") },
        { path: "transfers", component: () => import("@/pages/transfers.vue") },
        { path: "clients", component: () => import("@/pages/clients.vue") },
        // { path: 'attendance', component: () => import('@/pages/attendance.vue') },
        { path: "shifts", component: () => import("@/pages/shifts.vue") },
      ],
    },
  ],
});

router.beforeEach(async (to) => {
  const { useAuthStore } = await import("@/stores/auth");
  const auth = useAuthStore();
  const isLoginPage = to.path === "/login";
  const isSettings = to.path === "/settings";

  if (!auth.isLoggedIn && !isLoginPage) {
    return "/login";
  }
  if (auth.isLoggedIn && isLoginPage) {
    return "/home";
  }
  if (isSettings && !auth.isAdmin) {
    return "/home";
  }
});

export default router;
