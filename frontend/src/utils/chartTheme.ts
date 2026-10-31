/**
 * chart.js 的主题色桥接
 *
 * 图表是唯一「颜色写在 JS 里」的地方：Tailwind 的色阶映射管不到 canvas。
 * 全站 15 个图表文件里有 14 个带硬编码 hex（见方案 9.3 / 12.1），
 * 这里提供统一的取色入口，避免每个文件各写一套。
 *
 * 所有取值都从 endfield 的 CSS 变量读，主题切换后重新调用即可换色，
 * 不要在图表里写死 #RRGGBB。
 */

import { onMounted, onUnmounted, reactive } from 'vue'
import type { Chart as ChartJS } from 'chart.js'

const readVar = (name: string, fallback: string): string => {
  if (typeof window === 'undefined') return fallback
  const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  return v || fallback
}

export interface EfChartPalette {
  paper: string
  panel: string
  ink: string
  ink2: string
  rule: string
  chip: string
  signal: string
  onSignal: string
  trace: string
  danger: string
  motionDuration: number
  /** Business categories, paired with line/point encoding for accessibility. */
  steps: string[]
}

/** 读取当前主题下的完整图表调色板 */
export function efChartPalette(): EfChartPalette {
  const ink = readVar('--pt-ink', '#161616')
  const trace = readVar('--ef-trace', ink)
  return {
    paper: readVar('--pt-paper', '#f4f4f2'),
    panel: readVar('--pt-panel', '#ffffff'),
    ink,
    ink2: readVar('--pt-muted', '#5c5c5c'),
    rule: readVar('--pt-rule', '#d9d9d6'),
    chip: readVar('--pt-chip', '#e4e4e0'),
    signal: readVar('--ef-signal', '#fffa00'),
    onSignal: readVar('--ef-on-signal', '#161616'),
    trace,
    danger: readVar('--ef-danger', '#b3261e'),
    motionDuration: typeof window !== 'undefined' && window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ? 0 : 280,
    steps: [
      readVar('--pt-blue', '#2554ff'),
      readVar('--pt-teal', '#009d72'),
      readVar('--pt-purple', '#8235ff'),
      readVar('--pt-amber', '#e85d00'),
      readVar('--pt-cyan', '#008bcc'),
      readVar('--pt-orchid', '#d400e4')
    ]
  }
}

/** 安全设置透明度；不能在 rgb()/color-mix() 后面拼接十六进制后缀。 */
export function withAlpha(color: string, alpha: number): string {
  const a = Number.isFinite(alpha) ? Math.max(0, Math.min(1, alpha)) : 0
  const c = color.trim()
  const hex = /^#([\da-f]{3}|[\da-f]{6})$/i.exec(c)
  if (hex) {
    const digits = hex[1].length === 3 ? [...hex[1]].map(v => v + v).join('') : hex[1]
    const value = Number.parseInt(digits, 16)
    return `rgba(${(value >> 16) & 255}, ${(value >> 8) & 255}, ${value & 255}, ${a})`
  }
  return `color-mix(in srgb, ${c} ${a * 100}%, transparent)`
}

/** 已挂载的图表也随主题切换，避免 setup 时的快照留在旧主题。 */
export function useChartPalette(): EfChartPalette {
  const palette = reactive(efChartPalette())
  let observer: MutationObserver | undefined
  let motion: MediaQueryList | undefined
  const refresh = () => Object.assign(palette, efChartPalette())
  onMounted(() => {
    refresh()
    observer = new MutationObserver(refresh)
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme', 'class'] })
    motion = window.matchMedia?.('(prefers-reduced-motion: reduce)')
    motion?.addEventListener('change', refresh)
  })
  onUnmounted(() => { observer?.disconnect(); motion?.removeEventListener('change', refresh) })
  return palette
}

/** 12 种线型配合 3 种点形，图例和曲线都能独立辨认。 */
export function efLineStyle(index = 0, palette = efChartPalette()) {
  const dashes = [[], [6, 3], [2, 3], [8, 3, 2, 3], [10, 4], [2, 2, 2, 6],
    [12, 3, 2, 3], [4, 2, 4, 6], [1, 3], [8, 2, 1, 2, 1, 2], [14, 4, 4, 4], [4, 3, 1, 3]]
  const points = ['circle', 'rectRot', 'triangle'] as const
  const color = palette.steps[index % palette.steps.length]
  return {
    borderColor: color,
    backgroundColor: withAlpha(color, 0.13),
    borderWidth: index === 0 ? 2.5 : 2,
    borderDash: dashes[index % dashes.length],
    pointStyle: points[index % points.length],
    pointRadius: 2,
    pointHoverRadius: 5,
    pointHitRadius: 10,
    pointBackgroundColor: palette.panel,
    pointBorderColor: color,
    pointBorderWidth: 1.5,
    fill: false,
    tension: 0.3
  }
}

/** 图例保留实际虚线；窄屏缩写姓名，完整名称仍在 tooltip 中。 */
export function efLineLegendLabels(palette = efChartPalette()) {
  return {
    color: palette.ink2,
    usePointStyle: true,
    pointStyle: 'line' as const,
    pointStyleWidth: 32,
    boxWidth: 32,
    boxHeight: 11,
    padding: 10,
    font: { size: 11, family: 'Saira Variable, Noto Sans SC, sans-serif' },
    generateLabels(chart: ChartJS) {
      const columns = chart.width < 480 ? 2 : chart.width < 800 ? 3 : 4
      const budget = Math.max(12, chart.width / columns - 60)
      const ctx = chart.ctx
      ctx.save()
      ctx.font = '11px "Saira Variable", "Noto Sans SC", sans-serif'
      const ChartClass = chart.constructor as typeof ChartJS
      const labels = ChartClass.defaults.plugins.legend.labels.generateLabels(chart).map(item => {
        const chars = Array.from(item.text)
        while (chars.length > 1 && ctx.measureText(chars.join('') + '…').width > budget) chars.pop()
        // 读取整条线的样式；Chart.js 的 point 默认样式不含 borderDash。
        const style = chart.data.datasets[item.datasetIndex ?? 0] as {
          borderDash?: number[]; borderWidth?: number; borderColor?: string
        }
        return { ...item, text: chars.length < Array.from(item.text).length ? chars.join('') + '…' : item.text,
          pointStyle: 'line' as const, lineDash: style.borderDash, lineWidth: style.borderWidth, strokeStyle: style.borderColor }
      })
      ctx.restore()
      return labels
    }
  }
}

/** 分类色：按索引取业务分类色，超出后循环 */
export function efSeriesColor(index: number): string {
  const { steps } = efChartPalette()
  return steps[index % steps.length]
}

/** 网格线：只留横向虚线，用最浅的 rule */
export function efGridOptions() {
  const p = efChartPalette()
  return {
    color: p.rule,
    borderDash: [2, 4] as number[],
    drawTicks: false,
    lineWidth: 1
  }
}

/** 坐标轴刻度文字 */
export function efTickOptions() {
  const p = efChartPalette()
  return {
    color: p.ink2,
    font: { family: 'Saira Variable, Noto Sans SC, sans-serif', size: 11 }
  }
}

/** 提示框：实心板 + 细描边，无圆角无阴影 */
export function efTooltipOptions() {
  const p = efChartPalette()
  return {
    backgroundColor: p.panel,
    titleColor: p.ink,
    bodyColor: p.ink2,
    borderColor: p.rule,
    borderWidth: 1,
    cornerRadius: 0,
    padding: 10,
    displayColors: false
  }
}

/** Semantic data colors; red remains reserved for destructive actions. */
export interface EfSemanticColors {
  danger: string;
  dangerFill: string;
  good: string;
  goodFill: string;
  info: string;
  infoFill: string;
  warn: string;
  warnFill: string;
  muted: string;
}

export function efSemanticColors(): EfSemanticColors {
  const p = efChartPalette();
  return {
    danger: p.steps[3],
    dangerFill: withAlpha(p.steps[3], 0.13),
    good: p.steps[1],
    goodFill: withAlpha(p.steps[1], 0.1),
    info: p.steps[0],
    infoFill: withAlpha(p.steps[0], 0.1),
    warn: p.steps[3],
    warnFill: withAlpha(p.steps[3], 0.13),
    muted: p.ink2,
  };
}
