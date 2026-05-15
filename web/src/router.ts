import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router';

// Four top-level tabs per §3 of 01-phase3-ui.md. The workspace store
// lives in Pinia, so it survives every route change without KeepAlive.
const routes: RouteRecordRaw[] = [
  {
    path: '/',
    name: 'generate',
    component: () => import('./views/GenerateView.vue'),
    meta: { label: 'Generate' },
  },
  {
    path: '/library',
    name: 'library',
    component: () => import('./views/LibraryView.vue'),
    meta: { label: 'Library' },
  },
  {
    path: '/tools',
    name: 'tools',
    component: () => import('./views/ToolsView.vue'),
    meta: { label: 'Tools' },
  },
  {
    path: '/contrast',
    name: 'contrast',
    component: () => import('./views/ContrastView.vue'),
    meta: { label: 'Contrast' },
  },
  {
    path: '/:pathMatch(.*)*',
    redirect: '/',
  },
];

export const router = createRouter({
  history: createWebHistory(),
  routes,
});

export const TAB_ROUTES = ['generate', 'library', 'tools', 'contrast'] as const;
export type TabName = (typeof TAB_ROUTES)[number];
