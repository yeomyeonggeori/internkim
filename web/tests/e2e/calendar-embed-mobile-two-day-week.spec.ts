import { expect, test, type Page } from '@playwright/test';
import { calendarEmbedPath, signInToCalendar } from './calendar-central-test-utils';

test.describe('embedded calendar week view at a narrow mobile viewport', () => {
	test.use({ locale: 'ko-KR' });

	test.beforeEach(async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await routeCalendarHolidays(page);
	});

	test('renders the week view without horizontal overflow at a narrow mobile viewport', async ({ page }) => {
		await openMobileWeekView(page);

		await expect(page.locator('.calendar-stage')).toBeVisible();
		const hasOverflow = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth);
		expect(hasOverflow).toBe(false);
	});

	test('creates an event with a participant through the draft popover at a narrow mobile viewport', async ({ page }) => {
		await routeParticipantsInvoke(page);
		const createdEvents = await routeEventCreateInvoke(page);
		await openMobileWeekView(page);

		const dayColumn = page.locator('[data-calendar-date="2026-06-08"]').first();
		await dayColumn.scrollIntoViewIfNeeded();
		const box = await dayColumn.boundingBox();
		if (!box) throw new Error('missing day column bounding box');
		await page.mouse.click(box.x + box.width / 2, box.y + Math.min(120, box.height / 3));

		const popover = page.locator('.calendar-draft-popover');
		await expect(popover).toBeVisible();
		await popover.getByLabel('제목').fill('모바일 저장 일정');

		const participantCombobox = popover.getByRole('combobox', { name: '참여자' });
		await participantCombobox.click();
		await page.getByRole('option', { name: '박예시' }).click();
		await expect(participantCombobox).toContainText('박예시');
		await page.keyboard.press('Escape');
		await popover.getByLabel('제목').press('Enter');

		await expect(popover).toHaveCount(0);
		await expect.poll(() => createdEvents.length).toBe(1);
		expect(createdEvents[0]?.title).toBe('모바일 저장 일정');
		expect(createdEvents[0]?.participantPersonHints).toEqual(['mobile-participant']);
	});
});

async function routeCalendarHolidays(page: Page): Promise<void> {
	await page.route('**/api/calendar/holidays?**', async (route) => {
		await route.fulfill({ json: { holidays: [], degraded: false } });
	});
}

async function routeParticipantsInvoke(page: Page): Promise<void> {
	await page.route('**/api/v1/tools/person_list/invoke', async (route) => {
		await route.fulfill({
			json: { result: { people: [{ personID: 'mobile-participant', name: '예시 박', email: 'mobile-participant@example.com' }] } }
		});
	});
}

type WrittenEvent = { eventID: string; title: string; participantPersonHints?: string[] };

async function routeEventCreateInvoke(page: Page): Promise<WrittenEvent[]> {
	const created: WrittenEvent[] = [];
	let sequence = 0;
	await page.route('**/api/v1/tools/event_add/invoke', async (route) => {
		const payload = route.request().postDataJSON() as { input: Record<string, unknown> };
		sequence += 1;
		const eventID = `mobile-created-event-${sequence}`;
		created.push({
			eventID,
			title: String(payload.input.title ?? ''),
			participantPersonHints: (payload.input.participantPersonHints as string[] | undefined) ?? []
		});
		await route.fulfill({
			json: {
				result: {
					eventID,
					title: payload.input.title,
					note: payload.input.note ?? '',
					location: payload.input.location ?? '',
					startsAt: payload.input.startsAt,
					endsAt: payload.input.endsAt,
					isWholeDay: payload.input.isWholeDay ?? false,
					participants: [],
					updatedAt: '2026-06-08T00:00:00.000Z'
				}
			}
		});
	});
	return created;
}

async function openMobileWeekView(page: Page): Promise<void> {
	await page.setViewportSize({ width: 390, height: 844 });
	await signInToCalendar(page);
	await page.goto(`${calendarEmbedPath}?date=2026-06-08`);
	await page.evaluate(() => window.localStorage.setItem('internkim.calendar.view', 'week'));
	await page.reload();
	await expect(page.locator('.calendar-stage')).toHaveClass(/calendar-stage-week/);
	await page.waitForTimeout(1000);
}
