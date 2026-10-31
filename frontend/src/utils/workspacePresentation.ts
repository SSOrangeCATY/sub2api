/** Presentation only. Route access, business state and custom content stay in their views. */
export type WorkspaceSurface = 'records' | 'settings' | 'task' | 'console' | 'monitor' | 'embedded'

export function workspacePresentation(path: string) {
  if (path === '/channel-status' || path === '/monitor' || path === '/admin/channels/monitor') {
    return { surface: 'monitor' as const, heading: false }
  }
  if (path.startsWith('/custom/')) return { surface: 'embedded' as const, heading: false }
  if (['/dashboard', '/admin/dashboard', '/admin/ops', '/admin/orders/dashboard'].includes(path)) {
    return { surface: 'console' as const, heading: false }
  }
  if (path === '/profile' || path.startsWith('/admin/settings')) {
    return { surface: 'settings' as const, heading: true }
  }
  if (['/redeem', '/purchase', '/subscriptions', '/orders'].includes(path) || path.startsWith('/payment/')) {
    return { surface: 'task' as const, heading: !path.startsWith('/payment/') }
  }
  return { surface: 'records' as const, heading: !['/batch-image', '/docs/batch-image', '/admin/risk-control', '/admin/prompt-audit'].includes(path) }
}
