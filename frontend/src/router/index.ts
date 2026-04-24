import { createRouter, createWebHistory } from 'vue-router'
import ClustersView from '@/views/ClustersView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: () => import('@/views/LoginView.vue'), meta: { public: true } },
    { path: '/setup', component: () => import('@/views/SetupView.vue'), meta: { public: true } },
    { path: '/unauthorized', component: () => import('@/views/UnauthorizedView.vue'), meta: { public: true } },
    { path: '/', redirect: '/clusters' },
    { path: '/clusters', component: ClustersView },
    {
      path: '/clusters/:id/:tab?',
      component: () => import('@/views/ClusterDetailView.vue'),
    },
    {
      path: '/machineclasses',
      component: () => import('@/views/MachineClassesView.vue'),
    },
    {
      path: '/repos',
      component: () => import('@/views/ReposView.vue'),
    },
    {
      path: '/logs',
      component: () => import('@/views/LogsView.vue'),
    },
    {
      path: '/audit',
      component: () => import('@/views/AuditView.vue'),
    },
    {
      path: '/users',
      component: () => import('@/views/UsersView.vue'),
    },
    {
      path: '/instances',
      component: () => import('@/views/InstancesView.vue'),
    },
  ],
})

export default router
