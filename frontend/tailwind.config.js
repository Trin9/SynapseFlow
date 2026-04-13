/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        // Node type colors (matching AGENTS.md: Blue for LLM, Gray for Script)
        'node-script': {
          DEFAULT: '#6b7280', // gray-500
          light: '#f3f4f6',   // gray-100
          border: '#d1d5db',  // gray-300
        },
        'node-llm': {
          DEFAULT: '#3b82f6', // blue-500
          light: '#eff6ff',   // blue-50
          border: '#93c5fd',  // blue-300
        },
        'node-mcp': {
          DEFAULT: '#8b5cf6', // violet-500
          light: '#f5f3ff',   // violet-50
          border: '#c4b5fd',  // violet-300
        },
        'node-human': {
          DEFAULT: '#f59e0b', // amber-500
          light: '#fffbeb',   // amber-50
          border: '#fcd34d',  // amber-300
        },
        'node-router': {
          DEFAULT: '#10b981', // emerald-500
          light: '#ecfdf5',   // emerald-50
          border: '#6ee7b7',  // emerald-300
        },
      },
    },
  },
  plugins: [],
}
