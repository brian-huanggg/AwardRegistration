import { fileURLToPath } from 'node:url';
import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';

const src = fileURLToPath(new URL('./src', import.meta.url));
const testdata = fileURLToPath(new URL('../testdata', import.meta.url));

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: { alias: { '@': src, testdata } },
  server: {
    fs: { allow: ['..'] },
    // The app calls same-origin /api/v1; in dev the Go backend (ESG_PUBLIC_ADDR, default :8080) answers it.
    proxy: { '/api': process.env.ESG_API_TARGET ?? 'http://localhost:8080' },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test-setup.ts'],
  },
});
