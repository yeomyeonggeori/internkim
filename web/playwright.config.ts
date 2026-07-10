import { defineConfig, devices } from '@playwright/test';

const baseURL = process.env.PLAYWRIGHT_BASE_URL ?? 'http://127.0.0.1:5174';
const shouldStartWebServer = process.env.PLAYWRIGHT_START_WEB_SERVER === '1';
const webServer = shouldStartWebServer ? createWebServer(baseURL) : null;

export default defineConfig({
	testDir: './tests/e2e',
	timeout: 30_000,
	expect: {
		timeout: 10_000
	},
	...(webServer ? { webServer } : {}),
	use: {
		...devices['Desktop Chrome'],
		...(process.env.PLAYWRIGHT_CHANNEL ? { channel: process.env.PLAYWRIGHT_CHANNEL } : {}),
		baseURL,
		trace: 'retain-on-failure'
	},
	reporter: [['list']]
});

function createWebServer(serverBaseURL: string) {
	const serverURL = new URL(serverBaseURL);
	if (!serverURL.port) {
		throw new Error('PLAYWRIGHT_BASE_URL must include a port when PLAYWRIGHT_START_WEB_SERVER=1');
	}
	return {
		command: `bun run dev --host ${serverURL.hostname} --port ${serverURL.port}`,
		url: serverBaseURL,
		timeout: 120_000,
		reuseExistingServer: false
	};
}
