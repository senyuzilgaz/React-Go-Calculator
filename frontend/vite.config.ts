/// <reference types="vitest/config" />
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// The API is addressed by relative path, so no request leaves the app's own origin and
// CORS never arises. nginx proxies the same paths in production (ADR-0012).
const apiProxy = {
  '/api': { target: 'http://localhost:8080', changeOrigin: true },
  '/healthz': { target: 'http://localhost:8080', changeOrigin: true },
}

export default defineConfig({
  plugins: [react()],
  server: { proxy: apiProxy },
  preview: { proxy: apiProxy },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    restoreMocks: true,
  },
})
