/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    // In development the API runs separately (`make dev` in backend/); proxying
    // /api keeps the browser on one origin, just like nginx does in production.
    proxy: { '/api': 'http://127.0.0.1:5000' },
  },
  test: {
    environment: 'node',
  },
})
