import { defineConfig } from '@playwright/test'
export default defineConfig({
 testDir: './e2e',
 use: { baseURL: 'http://127.0.0.1:17331', headless: true, viewport: { width: 1440, height: 960 } },
 webServer: [{ command: '../bin/codevisual analyze ../testdata/basic --port 17331 --no-browser', url: 'http://127.0.0.1:17331/api/v1/project', reuseExistingServer: false, timeout: 60000 }, { command: '../bin/codevisual analyze ../testdata/explorers --port 17332 --no-browser', url: 'http://127.0.0.1:17332/api/v1/project', reuseExistingServer: false, timeout: 60000 }],
})
