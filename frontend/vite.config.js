import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// Saat development, request ke /api diteruskan ke backend Go di port 8080,
// sehingga tidak perlu mengatur CORS.
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
})
