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
  /** 墨色四阶，用于分类项（饼图改横向条后的替代） */
  steps: [string, string, string, string]
}

/** 读取当前主题下的完整图表调色板 */
export function efChartPalette(): EfChartPalette {
  const ink = readVar('--ef-ink', '#161616')
  const trace = readVar('--ef-trace', ink)
  return {
    paper: readVar('--ef-paper', '#f4f4f2'),
    panel: readVar('--ef-panel', '#ffffff'),
    ink,
    ink2: readVar('--ef-ink-2', '#5c5c5c'),
    rule: readVar('--ef-rule', '#d9d9d6'),
    chip: readVar('--ef-chip', '#e4e4e0'),
    signal: readVar('--ef-signal', '#fffa00'),
    onSignal: readVar('--ef-on-signal', '#161616'),
    trace,
    danger: readVar('--ef-danger', '#b3261e'),
    steps: [
      `color-mix(in srgb, ${ink} 45%, transparent)`,
      `color-mix(in srgb, ${ink} 65%, transparent)`,
      `color-mix(in srgb, ${ink} 85%, transparent)`,
      ink
    ]
  }
}

/** 分类色：按索引取墨色四阶，超出后循环 */
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
