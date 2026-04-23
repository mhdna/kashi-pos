/**
 * router/index.ts
 *
 * Manual routes for ./src/pages/*.vue
 */
import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/auth',
      component: () => import('@/layouts/AuthLayout.vue'),
      children: [
        { path: '/login', component: () => import('@/pages/login.vue') },
      ],
    },
    {
      path: '/',
      component: () => import('@/layouts/DefaultLayout.vue'),
      children: [
        { path: 'home', component: () => import('@/pages/home.vue') },
        { path: 'inventory', component: () => import('@/pages/inventory.vue') },
        { path: 'transfers', component: () => import('@/pages/transfers.vue') },
        { path: 'clients', component: () => import('@/pages/clients.vue') },
        { path: 'attendance', component: () => import('@/pages/attendance.vue') },
        { path: 'shifts', component: () => import('@/pages/shifts.vue') },
      ],
    },
  ],
})

export default router