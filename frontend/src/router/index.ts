/**
 * router/index.ts
 *
 * Manual routes for ./src/pages/*.vue
 */
import { createRouter, createWebHashHistory } from "vue-router";

const loadInvoicesList = () => import("@/pages/invoices-list.vue");

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    {
      path: "/auth",
      component: () => import("@/layouts/AuthLayout.vue"),
      children: [{ path: "/login", component: () => import("@/pages/login.vue") }],
    },
    {
      path: "/",
      component: () => import("@/layouts/DefaultLayout.vue"),
      children: [
        { path: "home", component: () => import("@/pages/home.vue") },
        {
          path: "settings",
          component: () => import("@/pages/settings.vue"),
          meta: { requiresAdmin: true },
        },
        {
          path: "attendance",
          component: () => import("@/pages/attendance.vue"),
          meta: { lockable: "attendance" },
        },
        {
          path: "invoices-list",
          component: loadInvoicesList,
        },
        {
          path: "inventory",
          component: () => import("@/pages/inventory.vue"),
          meta: { lockable: "inventory" },
        },
        {
          path: "salespersons",
          component: () => import("@/pages/salespersons.vue"),
          meta: { lockable: "salespersons" },
        },
        {
          path: "transfers",
          component: () => import("@/pages/transfers.vue"),
          meta: { lockable: "transfers" },
        },
        {
          path: "expenses",
          component: () => import("@/pages/expenses.vue"),
        },
        {
          path: "loans",
          component: () => import("@/pages/loans.vue"),
        },
        {
          path: "clients",
          component: () => import("@/pages/clients.vue"),
          meta: { lockable: "clients" },
        },
        // { path: 'attendance', component: () => import('@/pages/attendance.vue') },
        { path: "shifts", component: () => import("@/pages/shifts.vue") },
      ],
    },
    {
      path: "/close-shift",
      component: () => import("@/pages/close-shift.vue"),
    },
  ],
});

router.isReady().then(() => void loadInvoicesList().catch(() => undefined));

router.beforeEach(async (to) => {
  try {
    const { useAuthStore } = await import("@/stores/auth");
    const auth = useAuthStore();
    const isLoginPage = to.path === "/login";
    const requiresAdmin = to.matched.some((route) => route.meta.requiresAdmin);
    const lockable = to.matched.map((route) => route.meta.lockable).find((page) => page);

    if (!auth.isLoggedIn && !isLoginPage) {
      return "/login";
    }
    if (auth.isLoggedIn && isLoginPage) {
      return "/home";
    }
    if (requiresAdmin && !auth.isAdmin) {
      return "/home";
    }
    if (lockable && !auth.isAdmin) {
      const { usePageUnlocks } = await import("@/composables/usePageUnlocks");
      const { pageUnlocks } = usePageUnlocks();
      if (!pageUnlocks.value[String(lockable)]) {
        return "/home";
      }
    }
  } catch {
    // Allow navigation if auth module can't be loaded (e.g. Wails runtime not ready)
    return true;
  }
});

export default router;
