/* eslint-disable */
/** @type {import('tailwindcss').Config} */

/* ── 函数式颜色：让 var() 也能吃 /50 这类透明度修饰符 ───────────────
   Tailwind 的 withAlphaValue 对 var(...) 调 parseColor 会返回 null，
   直接写 'var(--x)' 会让 bg-x/50 完全不产出 CSS。函数式返回 color-mix
   即可绕过，且 endfield 本身就以 color-mix 为浏览器基线。 */
const v = (name) => ({ opacityValue }) =>
  opacityValue === undefined
    ? `var(${name})`
    : `color-mix(in srgb, var(${name}) calc(${opacityValue} * 100%), transparent)`

const T = {
  paper: v('--ef-paper'),   panel: v('--ef-panel'),  ink:  v('--ef-ink'),
  ink2:  v('--ef-ink-2'),   rule:  v('--ef-rule'),   chip: v('--ef-chip'),
  tagBg: v('--ef-tag-bg'),  tagFg: v('--ef-tag-fg'), soft: v('--ef-signal-soft')
}

const STEPS = [50, 100, 200, 300, 400, 500, 600, 700, 800, 900, 950]
const mk = (pick) => Object.fromEntries(STEPS.map((s) => [s, T[pick(s)]]))

/* 底色阶梯（普通色族） */
const bgL = mk((s) => (s <= 50 ? 'paper' : s <= 100 ? 'chip' : s <= 300 ? 'rule'
                      : s <= 400 ? 'chip' : s <= 900 ? 'tagBg' : 'paper'))

/* 文字阶梯（所有色族共用：低阶次级墨，高阶墨） */
const txL = mk((s) => (s <= 50 ? 'paper' : s <= 600 ? 'ink2' : 'ink'))

/* 描边阶梯 */
const bdL = mk((s) => (s <= 300 ? 'rule' : s === 400 ? 'ink2' : s <= 900 ? 'tagBg' : 'paper'))

/* 警告色族（amber / orange / yellow / red）：低阶三档换浅黄底 */
const bgW = mk((s) => (s <= 200 ? 'soft' : s <= 300 ? 'rule' : s <= 400 ? 'chip'
                      : s <= 900 ? 'tagBg' : 'paper'))
const bdW = mk((s) => (s <= 200 ? 'soft' : s <= 300 ? 'rule' : s === 400 ? 'ink2'
                      : s <= 900 ? 'tagBg' : 'paper'))

/* dark 色族专属：它是反色阶，必须单独一张表，否则深色面板会翻转 */
const bgD = mk((s) => (s <= 300 ? 'paper' : s === 400 ? 'chip' : s === 500 ? 'tagBg'
                      : s === 600 ? 'panel' : s === 700 ? 'chip' : s === 800 ? 'panel' : 'paper'))
const txD = mk((s) => (s <= 50 ? 'paper' : s <= 200 ? 'ink' : s <= 600 ? 'ink2' : 'ink'))
const bdD = mk((s) => (s <= 300 ? 'rule' : s === 400 ? 'ink2' : 'rule'))

const PLAIN = ['primary', 'gray', 'slate', 'zinc', 'neutral', 'stone', 'blue', 'indigo',
               'sky', 'cyan', 'teal', 'violet', 'purple', 'pink', 'rose', 'fuchsia',
               'lime', 'emerald', 'green', 'accent']
const WARN = ['amber', 'orange', 'yellow', 'red']
const ALL = [...PLAIN, ...WARN, 'dark']

const fromEntries = (list, obj) => Object.fromEntries(list.map((f) => [f, obj]))

export default {
  darkMode: 'class', // 保持不变：仍靠 html.dark
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        ...fromEntries(PLAIN, bgL),
        ...fromEntries(WARN, bgW),
        dark: bgD,
        white: T.panel,
        black: '#000000'
      },

      /* 同一台阶、不同属性必须取不同令牌，所以这四个键要单独覆盖。
         没在这里列出的族，text-* 会退回用 colors 的 bg 阶梯。 */
      textColor: {
        ...fromEntries(ALL, txL),
        dark: txD,
        white: T.tagFg,
        black: T.panel
      },
      borderColor: {
        ...fromEntries(PLAIN, bdL),
        ...fromEntries(WARN, bdW),
        dark: bdD,
        white: T.tagFg,
        black: T.tagFg
      },
      ringColor: {
        ...fromEntries(PLAIN, bdL),
        ...fromEntries(WARN, bdW),
        dark: bdD,
        white: T.tagFg,
        black: T.tagFg
      },
      divideColor: {
        ...fromEntries(PLAIN, bdL),
        ...fromEntries(WARN, bdW),
        dark: bdD,
        white: T.tagFg,
        black: T.tagFg
      },

      /* ── 几何归零：endfield 是直角 ── */
      borderRadius: {
        none: '0', sm: '0', DEFAULT: '0', md: '0', lg: '0',
        xl: '0', '2xl': '0', '3xl': '0', '4xl': '0', full: '0'
      },
      /* ── 阴影归零：endfield 用描边，不用投影 ── */
      boxShadow: {
        sm: 'none', DEFAULT: 'none', md: 'none', lg: 'none', xl: 'none',
        '2xl': 'none', inner: 'none',
        glass: 'none', 'glass-sm': 'none', glow: 'none', 'glow-lg': 'none',
        card: 'none', 'card-hover': 'none', 'inner-glow': 'none'
      },
      /* ── 渐变归零 ── */
      backgroundImage: {
        'gradient-radial': 'none',
        'gradient-primary': 'none',
        'gradient-dark': 'none',
        'gradient-glass': 'none',
        'mesh-gradient': 'none'
      },

      fontFamily: {
        sans: ['Saira Variable', 'Saira', 'Noto Sans SC Variable', 'Noto Sans SC',
               'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', 'system-ui', 'sans-serif'],
        mono: ['ui-monospace', 'SFMono-Regular', 'Menlo', 'Consolas', 'monospace']
      },

      /* ── 原样保留：动画是中性能力 ── */
      animation: {
        'fade-in': 'fadeIn 0.3s ease-out',
        'slide-up': 'slideUp 0.3s ease-out',
        'slide-down': 'slideDown 0.3s ease-out',
        'slide-in-right': 'slideInRight 0.3s ease-out',
        'scale-in': 'scaleIn 0.2s ease-out',
        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        shimmer: 'shimmer 2s linear infinite',
        glow: 'glow 2s ease-in-out infinite alternate'
      },
      keyframes: {
        fadeIn: { '0%': { opacity: '0' }, '100%': { opacity: '1' } },
        slideUp: {
          '0%': { opacity: '0', transform: 'translateY(10px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' }
        },
        slideDown: {
          '0%': { opacity: '0', transform: 'translateY(-10px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' }
        },
        slideInRight: {
          '0%': { opacity: '0', transform: 'translateX(20px)' },
          '100%': { opacity: '1', transform: 'translateX(0)' }
        },
        scaleIn: {
          '0%': { opacity: '0', transform: 'scale(0.95)' },
          '100%': { opacity: '1', transform: 'scale(1)' }
        },
        shimmer: {
          '0%': { backgroundPosition: '-200% 0' },
          '100%': { backgroundPosition: '200% 0' }
        },
        glow: {
          '0%': { boxShadow: '0 0 20px rgba(20, 184, 166, 0.25)' },
          '100%': { boxShadow: '0 0 30px rgba(20, 184, 166, 0.4)' }
        }
      },
      backdropBlur: { xs: '2px' }
    }
  },
  plugins: []
}
