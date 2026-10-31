/**
 * 主题状态：同时维护 <html class="dark"> 与 <html data-theme="...">
 *
 * - `dark` class 是项目既有的约定，17 处读取点（图表、编辑器等）依赖它，不能动。
 * - `data-theme` 是 endfield.css 的令牌选择器，缺了它深色令牌不会生效。
 * 两者必须永远同步，所以只通过下面两个函数写入，不要在别处直接改 classList。
 */

export const THEME_STORAGE_KEY = 'theme'

export type EfTheme = 'light' | 'dark'

/** 首次进入时的主题判定：显式选择优先，否则跟随系统 */
export function resolveInitialDark(): boolean {
  const saved = localStorage.getItem(THEME_STORAGE_KEY)
  if (saved === 'dark') return true
  if (saved === 'light') return false
  return window.matchMedia('(prefers-color-scheme: dark)').matches
}

/** 同时写 class 与属性，两者永不漂移。不落盘，用于「跟随系统」的初始化 */
export function applyEfTheme(dark: boolean): void {
  const root = document.documentElement
  root.classList.toggle('dark', dark)
  root.dataset.theme = dark ? 'dark' : 'light'
}

/** 用户显式切换时用这个：写 DOM 并记住选择 */
export function setEfTheme(dark: boolean): void {
  applyEfTheme(dark)
  try {
    localStorage.setItem(THEME_STORAGE_KEY, dark ? 'dark' : 'light')
  } catch {
    // 隐私模式下 localStorage 可能抛错，主题仍然生效，不影响功能
  }
}

/** 读取当前是否深色（以 DOM 为准，不读 localStorage） */
export function isEfDark(): boolean {
  return document.documentElement.classList.contains('dark')
}
