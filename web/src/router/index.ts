import { createRouter, createWebHistory } from 'vue-router'
import { authRequired, getToken } from '@/api'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/Login.vue'),
    },
    {
      path: '/',
      name: 'dashboard',
      component: () => import('@/views/Dashboard.vue'),
    },
    {
      path: '/processes',
      name: 'processes',
      component: () => import('@/views/Processes.vue'),
    },
    {
      path: '/schedules',
      name: 'schedules',
      component: () => import('@/views/Schedules.vue'),
    },
    {
      path: '/logs',
      name: 'logs',
      component: () => import('@/views/Logs.vue'),
    },
    {
      path: '/settings',
      name: 'settings',
      component: () => import('@/views/Settings.vue'),
    },
  ],
})

router.beforeEach(async (to) => {
  const required = await authRequired()
  if (to.name === 'login') {
    if (!required) return { name: 'dashboard' }
    if (getToken()) return { name: 'dashboard' }
    return true
  }
  if (required && !getToken()) return { name: 'login' }
  return true
})

export default router
