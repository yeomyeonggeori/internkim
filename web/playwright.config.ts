import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
	testDir: './tests/e2e',
	timeout: 30_000,
	expect: {
		timeout: 10_000
	},
	use: {
		...devices['Desktop Chrome'],
		trace: 'retain-on-failure'
	},
	reporter: [['list']]
});
