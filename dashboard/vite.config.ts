import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// The dashboard talks to the gateway (single base URL, same as iOS).
// In dev, /api/* is proxied to the local gateway so there is no CORS setup.
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api/, ''),
      },
    },
  },
})
