import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    // Pinned rather than left to resolve however localhost happens to,
    // so `make demo`'s readiness check and the printed URL always agree.
    host: '127.0.0.1',
    port: 5173,
    strictPort: true,
  },
})
