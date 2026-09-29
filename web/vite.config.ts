/// <reference types="vitest/config" />
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

// The Go server embeds dist/app (see embed.go). During development the Vite
// dev server proxies API calls to a Consigna server on port 7431.
//
// The dev server only listens on localhost, so every browser reaching it is on
// this computer and the Go server rightly treats it as the host. Do not expose
// it with --host: phones would then reach the API through the proxy as the host.
const backend = process.env.CONSIGNA_BACKEND ?? 'http://127.0.0.1:7431';

export default defineConfig({
  plugins: [react()],
  build: {
    outDir: 'dist/app',
    emptyOutDir: true,
    target: 'es2022',
    // Never inline assets as data: URIs; the server's CSP only allows fonts
    // from its own origin.
    assetsInlineLimit: 0,
  },
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      '/api': { target: backend },
      '/t/': { target: backend },
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.test.{ts,tsx}'],
    css: { modules: { classNameStrategy: 'non-scoped' } },
    restoreMocks: true,
  },
});
