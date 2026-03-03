import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/LoginView.vue'),
      meta: { guest: true },
    },
    {
      path: '/register',
      name: 'register',
      component: () => import('@/views/RegisterView.vue'),
      meta: { guest: true },
    },
    {
      path: '/share/:shareId',
      name: 'share-view',
      component: () => import('@/views/SharePublicView.vue'),
      meta: { guest: true },
    },
    {
      path: '/',
      component: () => import('@/components/layout/AppLayout.vue'),
      meta: { auth: true },
      children: [
        {
          path: '',
          name: 'files',
          component: () => import('@/views/FileBrowserView.vue'),
        },
        {
          path: 'trash',
          name: 'trash',
          component: () => import('@/views/TrashView.vue'),
        },
        {
          path: 'profile',
          name: 'profile',
          component: () => import('@/views/ProfileView.vue'),
        },
      ],
    },
  ],
})

router.beforeEach((to) => {
  const token = localStorage.getItem('light-cloud-token')

  if (to.meta.auth && !token) {
    return { name: 'login' }
  }

  if (to.meta.guest && token && to.name !== 'share-view') {
    return { name: 'files' }
  }
})

export default router
