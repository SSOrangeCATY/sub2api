import { sanitizeUrl } from '@/utils/url'

export const DEFAULT_BRAND_MARK = '/ptcrys-mark.svg'

export function updateFavicon(logoUrl: string): void {
  const sanitizedLogoUrl = sanitizeUrl(logoUrl, {
    allowRelative: true,
    allowDataUrl: true,
  })
  if (logoUrl && !sanitizedLogoUrl) return
  const iconUrl = sanitizedLogoUrl || DEFAULT_BRAND_MARK

  let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
  if (!link) {
    link = document.createElement('link')
    link.rel = 'icon'
    document.head.appendChild(link)
  }

  link.type = iconUrl.endsWith('.svg') ? 'image/svg+xml' : 'image/x-icon'
  link.href = iconUrl
}
