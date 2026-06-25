import { expect, test as base } from '@playwright/test';
import { buildAttendanceSummaryFixture } from '../../dev-attendance-summary-fixture';

export const test = base.extend({
	page: async ({ page }, use) => {
		await page.route('**/admin/api/session', async (route) => {
			await route.fulfill({ json: { email: 'tester@example.com', isAdmin: true } });
		});
		await page.route('**/admin/api/locale', async (route) => {
			await route.fulfill({ json: { locale: 'ko' } });
		});
		await page.route('**/auth/session**', async (route) => {
			await route.fulfill({ json: { authenticated: true, email: 'tester@example.com' } });
		});
		await page.route('**/attendance/api/summary**', async (route) => {
			const requestURL = new URL(route.request().url());
			const month = requestURL.searchParams.get('month') ?? '2026-05';
			await route.fulfill({ json: buildAttendanceSummaryFixture(month) });
		});

		await use(page);
	}
});

export { expect };
