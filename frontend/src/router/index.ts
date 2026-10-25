/**
 * router/index.ts
 *
 * Manual routes for ./src/pages/*.vue
 */
import { createRouter, createWebHashHistory } from "vue-router";

const loadHome = () => import("@/pages/home.vue");
const loadInvoicesList = () => import("@/pages/invoices-list.vue");
const loadShifts = () => import("@/pages/shifts.vue");

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
        { path: "home", component: loadHome },
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
          path: "products",
          component: () => import("@/pages/products.vue"),
          meta: { lockable: "products" },
        },
        {
          path: "salespersons",
          component: () => import("@/pages/salespersons.vue"),
          meta: { lockable: "salespersons" },
        },
        // Admins see it outright; anyone else unlocks it with the manager
        // password on the page itself (see GetConversionReport).
        {
          path: "conversion",
          component: () => import("@/pages/conversion.vue"),
        },
        {
          path: "transfers",
          component: () => import("@/pages/transfers.vue"),
          meta: { lockable: "transfers" },
        },
        // Manager only: opened from the shift card's buttons, which ask for
        // the manager password first (see useManagerPass). Admins go straight in.
        {
          path: "expenses",
          component: () => import("@/pages/expenses.vue"),
          meta: { requiresManager: true },
        },
        {
          path: "loans",
          component: () => import("@/pages/loans.vue"),
          meta: { requiresManager: true },
        },
        {
          path: "clients",
          component: () => import("@/pages/clients.vue"),
          meta: { lockable: "clients" },
        },
        // { path: 'attendance', component: () => import('@/pages/attendance.vue') },
        { path: "shifts", component: loadShifts },
      ],
    },
    {
      path: "/close-shift",
      component: () => import("@/pages/close-shift.vue"),
    },
  ],
});

// Fetched while the login screen is up, so signing in doesn't then wait on
// the checkout page, the biggest chunk - nor the first click on Invoices
// or Balance.
router.isReady().then(() => {
  void loadHome().catch(() => undefined);
  void loadInvoicesList().catch(() => undefined);
  void loadShifts().catch(() => undefined);
});

router.beforeEach(async (to, from) => {
  try {
    const { useAuthStore } = await import("@/stores/auth");
    const auth = useAuthStore();
    const isLoginPage = to.path === "/login";
    const requiresAdmin = to.matched.some((route) => route.meta.requiresAdmin);
    const lockable = to.matched.map((route) => route.meta.lockable).find((page) => page);
    const requiresManager = to.matched.some((route) => route.meta.requiresManager);

    if (!auth.isLoggedIn && !isLoginPage) {
      return "/login";
    }
    if (auth.isLoggedIn && isLoginPage) {
      return "/home";
    }
    if (requiresAdmin && !auth.isAdmin) {
      return "/home";
    }
    // Staying on the page (a query change) doesn't need a new pass.
    if (requiresManager && !auth.isAdmin && from.path !== to.path) {
      const { takeManagerPass } = await import("@/composables/useManagerPass");
      if (!takeManagerPass(to.path)) {
        return "/home";
      }
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
