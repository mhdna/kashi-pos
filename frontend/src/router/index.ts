/**
 * router/index.ts
 *
 * Manual routes for ./src/pages/*.vue
 */
import { createRouter, createWebHashHistory } from "vue-router";
import { pageEnabled } from "@/lib/buildConfig";

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
        },
        {
          path: "invoices-list",
          component: loadInvoicesList,
        },
        {
          path: "inventory",
          component: () => import("@/pages/inventory.vue"),
          meta: { buildPage: "inventory", requiresAdmin: true },
        },
        {
          path: "import-data",
          component: () => import("@/pages/import-data.vue"),
          meta: { buildPage: "import-data" },
        },
        { path: "transfers", component: () => import("@/pages/transfers.vue") },
        {
          path: "clients",
          component: () => import("@/pages/clients.vue"),
          meta: { requiresAdmin: true },
        },
        // { path: 'attendance', component: () => import('@/pages/attendance.vue') },
        { path: "shifts", component: () => import("@/pages/shifts.vue") },
      ],
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
    const buildPage = to.matched.find((route) => route.meta.buildPage)?.meta.buildPage;
    if (typeof buildPage === "string" && !pageEnabled(buildPage)) return "/home";

    if (!auth.isLoggedIn && !isLoginPage) {
      return "/login";
    }
    if (auth.isLoggedIn && isLoginPage) {
      return "/home";
    }
    if (requiresAdmin && !auth.isAdmin) {
      return "/home";
    }
  } catch {
    // Allow navigation if auth module can't be loaded (e.g. Wails runtime not ready)
    return true;
  }
});

export default router;
