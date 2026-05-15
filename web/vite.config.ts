import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import { fileURLToPath, URL } from 'node:url';

// huetension web SPA build config.
//
// outDir: dist/ — slurped by the Go binary via `go:embed all:dist` in
// web/embed.go. Empty when running `huetension web --dev http://localhost:5173`;
// the Go server reverse-proxies non-API paths here for HMR.
//
// The /api proxy below is for contributors who hit http://localhost:5173
// directly (Vite as front door). The primary dev flow is the opposite
// direction — Go in front, Vite behind --dev — but this leaves both
// options open.
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      '/api': 'http://127.0.0.1:8080',
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    target: 'es2022',
    sourcemap: false,
  },
});
