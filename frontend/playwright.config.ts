import { defineConfig } from '@playwright/test';

// Q3-01 smoke. Local: Playwright starts the Go backend and the Vite dev server (compose services must be up:
// `make smoke` does that). DEV: set E2E_BASE_URL to the deployed origin; no servers are started.
const external = process.env.E2E_BASE_URL;
const baseURL = external ?? 'http://localhost:5173';

export default defineConfig({
  testDir: './e2e',
  timeout: 60_000,
  retries: 0,
  workers: 1,
  reporter: [['list']],
  use: { baseURL, locale: 'en-US', trace: 'retain-on-failure' },
  webServer: external
    ? undefined
    : [
        {
          command: 'go run ./cmd/server',
          cwd: '../backend',
          url: 'http://localhost:8081/actuator/health/readiness',
          reuseExistingServer: true,
          timeout: 120_000,
          env: { ESG_MINIO_ACCESS_KEY: 'minioadmin', ESG_MINIO_SECRET_KEY: 'minioadmin' },
        },
        { command: 'npm run dev -- --port 5173 --strictPort', url: baseURL, reuseExistingServer: true, timeout: 60_000 },
      ],
});
