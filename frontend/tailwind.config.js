/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        background: '#f4f4f5', // cool gray / warm graphite base
        surface: '#ffffff', // warm paper
        border: 'rgba(39, 39, 42, 0.12)', // graphite 12%
        'border-strong': 'rgba(39, 39, 42, 0.24)', // graphite 24%
        text: {
          900: '#18181b', // graphite 900
          600: '#52525b', // graphite 600
          400: '#a1a1aa', // graphite 400
        },
        mode: {
          outline: '#fb923c', // light orange
          page: '#60a5fa', // light blue
          normal: '#84cc16', // sage
          talk: '#6366f1', // indigo
          ask: '#f59e0b', // amber
          repo: '#c084fc', // light purple
          overview: '#facc15', // light yellow
          error: '#ef4444', // muted red
          final: '#22c55e', // green
        }
      }
    },
  },
  plugins: [],
}
