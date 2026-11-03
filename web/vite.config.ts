import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
export default defineConfig({
 plugins: [react()],
 build: { outDir: '../internal/server/webdist', emptyOutDir: true },
 server: { proxy: { '/api': { target: 'http://127.0.0.1:7331', changeOrigin: true } } },
 test: { environment: 'jsdom', setupFiles: ['./src/test-setup.ts'], include: ['src/**/*.test.{ts,tsx}'] },
})
