import typography from '@tailwindcss/typography';

/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        workspace: '#E9EDF2',
        background: '#E9EDF2',
        panel: '#F8F9FB',
        'panel-muted': '#F0F3F7',
        canvas: '#DFE5EC',
        surface: '#FFFFFF',
        ink: '#17202B',
        muted: '#5F6B7A',
        subtle: '#95A0AE',
        border: '#D5DBE3',
        'border-strong': '#BCC5D0',
        accent: 'rgb(var(--ui-accent) / <alpha-value>)',
        'accent-soft': 'rgb(var(--ui-selected) / <alpha-value>)',
        'accent-hover': 'rgb(var(--ui-accent-hover) / <alpha-value>)',
        hover: 'rgb(var(--ui-hover) / <alpha-value>)',
        selected: 'rgb(var(--ui-selected) / <alpha-value>)',
        'selected-foreground': 'rgb(var(--ui-selected-foreground) / <alpha-value>)',
        success: 'rgb(var(--ui-success) / <alpha-value>)',
        'success-soft': 'rgb(var(--ui-success-soft) / <alpha-value>)',
        warning: 'rgb(var(--ui-warning) / <alpha-value>)',
        'warning-soft': 'rgb(var(--ui-warning-soft) / <alpha-value>)',
        danger: 'rgb(var(--ui-danger) / <alpha-value>)',
        'danger-soft': 'rgb(var(--ui-danger-soft) / <alpha-value>)',
        text: {
          900: '#17202B',
          800: '#2F3945',
          700: '#475260',
          600: '#5F6B7A',
          500: '#7A8694',
          400: '#95A0AE',
        },
      },
      fontFamily: {
        sans: ['Aptos', 'Inter', '-apple-system', 'BlinkMacSystemFont', '"SF Pro Text"', '"PingFang SC"', '"Microsoft YaHei"', 'sans-serif'],
        mono: ['"SFMono-Regular"', 'Consolas', '"Liberation Mono"', 'monospace'],
      },
      boxShadow: {
        canvas: '0 8px 24px rgba(71, 85, 105, 0.14)',
        overlay: '0 12px 32px rgba(51, 65, 85, 0.18)',
      },
    },
  },
  plugins: [typography],
}
