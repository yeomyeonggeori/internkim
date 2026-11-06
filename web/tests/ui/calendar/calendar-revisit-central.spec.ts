import { expect, test } from '@playwright/test';
import { cleanupCalendarEvents, seedCalendarEvents, signInToCalendar } from './calendar-central-test-utils';

test.use({ locale: 'ko-KR', timezoneId: 'Asia/Seoul' });

test('a reloaded calendar shows the events it last read while the range is still being read', async ({ page }) => {
	const today = new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Seoul' }).format(new Date());
	const title = '다시 열 때 바로 보이는 일정';
	const eventIDs = await seedCalendarEvents([{ title, startISO: `${today}T10:00:00+09:00`, endISO: `${today}T11:00:00+09:00` }]);
	let release = () => {};
	const gate = new Promise<void>((resolve) => { release = resolve; });
	try {
		await signInToCalendar(page);
		await expect(page.getByText(title).first()).toBeVisible();
		await page.route(/\/api\/v1\/tools\/event_list\/invoke$/, async (route) => {
			await gate;
			await route.continue().catch(() => undefined);
		});
		await page.reload();
		await expect(page.getByText(title).first()).toBeVisible();
	} finally {
		release();
		await cleanupCalendarEvents(eventIDs);
	}
});
