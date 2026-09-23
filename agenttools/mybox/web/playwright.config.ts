import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './tests/e2e',
  timeout: 30000,
  retries: 0,
  use: {
    baseURL: 'http://127.0.0.1:19090',
    trace: 'on-first-retry',
  },
  webServer: {
    command: './tests/e2e/server.sh',
    url: "http://127.0.0.1:19090",
    reuseExistingServer: true,
    timeout: 120000,
  },
})
