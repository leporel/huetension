import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router';

// Two top-level tabs: a single Tools page holds every generator /
// tool / checker card; Library stays separate. The workspace store lives
// in Pinia, so it survives every route change without KeepAlive.
const routes: RouteRecordRaw[] = [
  {
    path: '/',
    name: 'tools',
    component: () => import('./views/ToolsView.vue'),
    meta: { label: 'Tools' },
  },
  {
    // Optional `:id` deep-links a single palette (/library/aurora)
    // without remounting the view — `/library` and `/library/:id`
    // resolve to the same component, so the workspace store and the
    // loaded catalogue survive selection.
    path: '/library/:id?',
    name: 'library',
    component: () => import('./views/LibraryView.vue'),
    meta: { label: 'Library' },
  },
  // Legacy bookmarks: the Generate / Tools / Contrast tabs collapsed into
  // the unified Tools page. Preserve `?section=` so a deep-linked card
  // still scrolls into view after the redirect.
  { path: '/tools', redirect: (to) => ({ path: '/', query: to.query }) },
  { path: '/contrast', redirect: (to) => ({ path: '/', query: to.query }) },
  {
    path: '/:pathMatch(.*)*',
    redirect: '/',
  },
];

export const router = createRouter({
  history: createWebHistory(),
  routes,
});
