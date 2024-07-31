import { createWebHistory, createRouter } from 'vue-router'
import Layout from '~/layout/index.vue'
import Home from '~/layout/home.vue'

const routes = [{
  path: '',
  component: Layout,
  redirect: '/home',
  children: [{
    path: 'home',
    component: Home,
    redirect: '/home/ollama',
    children: [{
      path: 'ollama',
      component: () => import('~/views/home/ollama.vue')
    }, {
      path: 'tags',
      component: () => import('~/views/home/tags.vue')
    }, {
      path: 'online',
      component: () => import('~/views/home/online.vue')
    }]
  },
  {
    path: 'chat',
    component: () => import('~/views/about/index.vue')
  },
  {
    path: 'setting',
    component: () => import('~/views/about/index.vue')
  },
  {
    path: 'about',
    component: () => import('~/views/about/index.vue')
  }
  ]
},
{
  path: '/:pathMatch(.*)*',
  redirect: '/home'
}
]

const router = createRouter({
  history: createWebHistory(), // 路由类型
  routes, // short for `routes: routes`
  scrollBehavior(to, from, savedPosition) {
    if (savedPosition) {
      return savedPosition
    } else {
      return {
        top: 0
      }
    }
  }
})

export default router
