import { expect, test, type Page } from '@playwright/test';
import { calendarEmbedPath, cleanupCalendarEvents, seedCalendarEvents, signInToCalendar } from './calendar-central-test-utils';

test.describe('embedded calendar day-view mini calendar', () => {
	test.use({ locale: 'ko-KR' });

	test.beforeEach(async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-10T12:00:00'));
		await routeCalendarHolidays(page);
	});

	test('shows an event dot on the day-view mini calendar for a day with an event', async ({ page }) => {
		const [eventID] = await seedCalendarEvents([
			{ title: '겹치는 일정', startISO: '2026-06-08T01:00:00+09:00', endISO: '2026-06-08T02:00:00+09:00' }
		]);
		try {
			await openDesktopDayView(page);

			const eventDay = page.getByRole('complementary').locator('[data-value="2026-06-08"]').last();
			const emptyDay = page.getByRole('complementary').locator('[data-value="2026-06-09"]').last();
			await expect(eventDay.locator('span.rounded-full')).toBeVisible();
			await expect(emptyDay.locator('span.rounded-full')).toHaveCount(0);
		} finally {
			await cleanupCalendarEvents([eventID]);
		}
	});

	test('deletes an event with an undo toast; undo restores it, and letting it expire persists the delete', async ({ page }) => {
		const deletedEventHints = await routeEventDeleteInvoke(page);
		const [eventID] = await seedCalendarEvents([
			{ title: '삭제할 일정', startISO: '2026-06-10T02:00:00+09:00', endISO: '2026-06-10T03:00:00+09:00' }
		]);
		try {
			await openDesktopDayView(page);
			const eventChip = page.locator(`[data-calendar-event-id="${eventID}"]`);
			await expect(eventChip).toBeVisible();
			await eventChip.click();
			const popover = page.locator('.calendar-draft-popover');
			await expect(popover).toBeVisible();
			await popover.getByRole('button', { name: '삭제' }).click();
			await expect(popover).toHaveCount(0);
			await expect(eventChip).toHaveCount(0);

			const deleteUndo = page.locator('.calendar-delete-undo-toast');
			await expect(deleteUndo).toBeVisible();
			await expect(deleteUndo).toContainText('일정을 삭제했습니다.');
			await deleteUndo.getByRole('button', { name: '실행 취소' }).click();
			await expect(deleteUndo).toHaveCount(0);
			await expect(eventChip).toBeVisible();
			expect(deletedEventHints).toEqual([]);

			await eventChip.click();
			await expect(popover).toBeVisible();
			await popover.getByRole('button', { name: '삭제' }).click();
			await expect(eventChip).toHaveCount(0);
			await expect(page.locator('.calendar-delete-undo-toast')).toBeVisible();
			await expect.poll(() => deletedEventHints, { timeout: 8000 }).toEqual([eventID]);
		} finally {
			await cleanupCalendarEvents([eventID]);
		}
	});

});

async function routeCalendarHolidays(page: Page): Promise<void> {
	await page.route('**/api/calendar/holidays?**', async (route) => {
		await route.fulfill({ json: { holidays: [], degraded: false } });
	});
}

async function routeEventDeleteInvoke(page: Page): Promise<string[]> {
	const deleted: string[] = [];
	await page.route('**/api/v1/tools/event_delete/invoke', async (route) => {
		const payload = route.request().postDataJSON() as { input: { eventHint: string } };
		deleted.push(payload.input.eventHint);
		await route.fulfill({ json: { result: {} } });
	});
	return deleted;
}

async function openDesktopDayView(page: Page): Promise<void> {
	await page.setViewportSize({ width: 1280, height: 900 });
	await signInToCalendar(page);
	await page.goto(`${calendarEmbedPath}?date=2026-06-10`);
	await page.evaluate(() => window.localStorage.setItem('internkim.calendar.view', 'day'));
	await page.reload();
	await expect(page.locator('.calendar-stage')).toHaveClass(/calendar-stage-day/);
	await page.waitForTimeout(1000);
}
