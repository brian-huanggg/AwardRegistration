import { fileURLToPath } from 'node:url';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';

const src = fileURLToPath(new URL('./src', import.meta.url));
const testdata = fileURLToPath(new URL('../testdata', import.meta.url));

export default defineConfig({
  plugins: [react()],
  resolve: { alias: { '@': src, testdata } },
  server: { fs: { allow: ['..'] } },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test-setup.ts'],
  },
});
