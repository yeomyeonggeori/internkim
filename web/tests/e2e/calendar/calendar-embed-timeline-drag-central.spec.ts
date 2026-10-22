import { expect, test, type Page } from '@playwright/test';
import { cleanupCalendarEvents, seedCalendarEvents, signInToCalendar } from './calendar-central-test-utils';
import { openCalendarEmbed } from './calendar-embed-interaction-helpers';

const hourHeightPixels = 48;

async function routeCalendarHolidays(page: Page): Promise<void> {
	await page.route('**/api/calendar/holidays?**', async (route) => {
		await route.fulfill({ json: { holidays: [], degraded: false } });
	});
}

async function dayColumnBox(page: Page, dateKey: string): Promise<{ x: number; y: number; width: number; height: number }> {
	const box = await page.locator(`[data-calendar-date="${dateKey}"]`).first().boundingBox();
	if (!box) throw new Error(`Missing day column: ${dateKey}`);
	return box;
}

async function clickTimelineHour(page: Page, dateKey: string, hour: number): Promise<void> {
	const box = await dayColumnBox(page, dateKey);
	await page.mouse.click(box.x + box.width / 2, box.y + hour * hourHeightPixels + 4);
}

async function dragTimelineHours(page: Page, dateKey: string, startHour: number, endHour: number): Promise<void> {
	const box = await dayColumnBox(page, dateKey);
	const x = box.x + box.width / 2;
	await page.mouse.move(x, box.y + startHour * hourHeightPixels + 4);
	await page.mouse.down();
	await page.mouse.move(x, box.y + endHour * hourHeightPixels + 4, { steps: 8 });
	await page.mouse.up();
}

async function expectDraftCount(page: Page, expectedCount: number): Promise<void> {
	await expect
		.poll(async () =>
			page.locator('[data-calendar-event-id]').evaluateAll((elements) =>
				elements.filter((element) => (element.getAttribute('data-calendar-event-id') ?? '').startsWith('timeline-')).length
			)
		)
		.toBe(expectedCount);
}

test.describe('embedded calendar timeline drag interactions', () => {
	test.use({ locale: 'ko-KR' });

	test.beforeEach(async ({ page }) => {
		await routeCalendarHolidays(page);
	});

	test('creates one draft when clicking a timeline slot', async ({ page }) => {
		await signInToCalendar(page);

		await openCalendarEmbed(page, '일');
		await clickTimelineHour(page, '2026-06-08', 9);
		await expectDraftCount(page, 1);

		await openCalendarEmbed(page, '주');
		await clickTimelineHour(page, '2026-06-08', 9);
		await expectDraftCount(page, 1);
	});

	test('creates one draft when dragging a timeline range', async ({ page }) => {
		await signInToCalendar(page);

		await openCalendarEmbed(page, '일');
		await dragTimelineHours(page, '2026-06-08', 2, 4);
		await expectDraftCount(page, 1);

		await openCalendarEmbed(page, '주');
		await dragTimelineHours(page, '2026-06-08', 2, 4);
		await expectDraftCount(page, 1);
	});

	test('creates a day draft from the midnight timeline cell by dragging', async ({ page }) => {
		await signInToCalendar(page);

		await openCalendarEmbed(page, '일');
		await dragTimelineHours(page, '2026-06-08', 0, 1);

		await expectDraftCount(page, 1);
		const draftPopover = page.locator('.calendar-draft-popover');
		await expect(draftPopover).toBeVisible();
		await expect(draftPopover.getByLabel('시작 시간')).toHaveValue('00:00');
		await expect(draftPopover.getByLabel('종료 시간')).toHaveValue('01:00');
	});
});

test.describe('seeded timeline events', () => {
	test.use({ locale: 'ko-KR' });

	test.beforeEach(async ({ page }) => {
		await routeCalendarHolidays(page);
	});

	test('renders a seeded event on the day and week timelines', async ({ page }) => {
		const [eventID] = await seedCalendarEvents([
			{ title: '겹치는 일정', startISO: '2026-06-08T02:00:00+09:00', endISO: '2026-06-08T03:30:00+09:00' }
		]);
		try {
			await signInToCalendar(page);

			await openCalendarEmbed(page, '일');
			await expect(page.locator(`[data-calendar-event-id="${eventID}"]`)).toBeVisible();

			await openCalendarEmbed(page, '주');
			await expect(page.locator(`[data-calendar-event-id="${eventID}"]`)).toBeVisible();
		} finally {
			await cleanupCalendarEvents([eventID]);
		}
	});
});
