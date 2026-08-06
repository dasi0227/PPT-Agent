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
        accent: '#2F67F6',
        'accent-soft': '#E8EFFF',
        success: '#2F7D65',
        'success-soft': '#E7F3EE',
        warning: '#B96D1F',
        'warning-soft': '#FFF3DF',
        danger: '#C84953',
        'danger-soft': '#FCECEF',
        text: {
          900: '#17202B',
          600: '#5F6B7A',
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
