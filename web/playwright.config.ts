import { defineConfig } from '@playwright/test'
const shortenerServer = process.env.CODEVISUAL_TEST_SHORTENER ? [{ command: '../bin/codevisual analyze ../../Go-URL-Shortener --port 17333 --no-browser', url: 'http://127.0.0.1:17333/api/v1/project', reuseExistingServer: false, timeout: 60000 }] : []
const bookingServer = process.env.CODEVISUAL_TEST_BOOKING ? [{ command: '../bin/codevisual analyze ../../Room-Booking-Service --port 17334 --no-browser', url: 'http://127.0.0.1:17334/api/v1/project', reuseExistingServer: false, timeout: 60000 }] : []
const selfServer = process.env.CODEVISUAL_TEST_SELF ? [{ command: '../bin/codevisual analyze .. --port 17335 --no-browser', url: 'http://127.0.0.1:17335/api/v1/project', reuseExistingServer: false, timeout: 60000 }] : []
export default defineConfig({
 testDir: './e2e',
 use: { baseURL: 'http://127.0.0.1:17331', headless: true, viewport: { width: 1440, height: 960 } },
 webServer: [{ command: '../bin/codevisual analyze ../testdata/basic --port 17331 --no-browser', url: 'http://127.0.0.1:17331/api/v1/project', reuseExistingServer: false, timeout: 60000 }, { command: '../bin/codevisual analyze ../testdata/explorers --port 17332 --no-browser', url: 'http://127.0.0.1:17332/api/v1/project', reuseExistingServer: false, timeout: 60000 }, ...shortenerServer, ...bookingServer, ...selfServer],
})
