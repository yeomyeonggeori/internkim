import { expect, test } from '@playwright/test';
import { routeCalendarAPI, waitForClientHydration } from './calendar-draft-popover-test-utils';

test.describe('calendar draft popover edit mode', () => {
	test.beforeEach(async ({ page }) => {
		await routeCalendarAPI(page);
	});

	test('opens existing events in edit mode and deletes them from the popover', async ({ page }) => {
		let deleteIntentCount = 0;
		await page.unroute('**/calendar/api/events?**');
		await page.route('**/calendar/api/events?**', async (route) => {
			await route.fulfill({
				json: {
					events: [
						{
							id: 'existing-event',
							title: '기존 일정',
							description: '기존 설명',
							location: '회의실 B',
							startISO: '2026-06-10T09:00:00.000Z',
							endISO: '2026-06-10T10:00:00.000Z',
							isAllDay: false
						}
					]
				}
			});
		});
		await page.route('**/calendar/api/events/existing-event/delete-intents/*', async (route) => {
			if (route.request().method() !== 'PUT') {
				await route.fulfill({ status: 204, body: '' });
				return;
			}
			deleteIntentCount += 1;
			const operationID = decodeURIComponent(new URL(route.request().url()).pathname.split('/').pop() ?? '');
			await route.fulfill({
				json: {
					operationID,
					executeAt: '2026-06-08T12:00:05.000Z'
				}
			});
		});
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		await page.locator('.calendar-month-direct-event[data-event-id="existing-event"]').dblclick();
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await expect(page.getByLabel('제목')).toHaveValue('기존 일정');

		await page.getByRole('button', { name: '삭제' }).click();

		await expect.poll(() => deleteIntentCount).toBe(1);
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		await expect(page.getByText('기존 일정')).toHaveCount(0);
	});
});
