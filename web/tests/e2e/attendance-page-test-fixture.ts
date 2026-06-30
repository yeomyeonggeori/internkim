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
		await page.route('**/calendar/api/events?**', async (route) => {
			await route.fulfill({ json: { events: [] } });
		});
		await page.route('**/flow/api/state', async (route) => {
			await route.fulfill({
				json: {
					members: [],
					tasks: [],
					metrics: {
						totalTasks: 0,
						completedTasks: 0,
						requestedTasks: 0,
						pausedTasks: 0,
						stoppedTasks: 0,
						statusCounts: {},
						businessCounts: {},
						typeCounts: {}
					},
					definitions: { categories: [], types: [], sizes: [] },
					statusOptions: [],
					currentUserEmail: 'tester@example.com',
					currentUserName: 'Tester',
					isAdmin: true,
					source: 'test'
				}
			});
		});

		await use(page);
	}
});

export { expect };
