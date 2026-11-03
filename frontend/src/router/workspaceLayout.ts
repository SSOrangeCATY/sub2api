import type { RouteRecordRaw } from 'vue-router'

/** Keep navigation mounted across workspace pages without retaining page state. */
export function withWorkspaceLayout(pages: RouteRecordRaw[]): RouteRecordRaw[] {
  const workspacePages = pages.filter(page => page.meta?.workspaceLayout === true)
  return pages.filter(page => page.meta?.workspaceLayout !== true).map(page => {
    if (page.path !== '/') return page
    return {
      ...page,
      component: () => import('@/components/layout/WorkspaceRouteLayout.vue'),
      children: workspacePages,
    } as RouteRecordRaw
  })
}
