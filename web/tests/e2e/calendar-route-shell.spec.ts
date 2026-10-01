import { expect, test, type Page } from '@playwright/test';
import {
	calendarEmbedPath,
	cleanupCalendarEvents,
	seedCalendarEvents,
	signInToCalendar
} from './calendar-central-test-utils';
import { cleanupLeave, member1ID, member2ID, seedLeave } from './central-test-utils';

async function routeCalendarHolidays(page: Page): Promise<void> {
	await page.route('**/api/calendar/holidays?**', async (route) => {
		await route.fulfill({ json: { holidays: [], degraded: false } });
	});
}

test.describe('calendar route shell', () => {
	test.use({ locale: 'ko-KR' });

	test('renders the embedded calendar without the old sidebar', async ({ page }) => {
		await routeCalendarHolidays(page);
		await page.clock.setFixedTime(new Date('2026-06-15T12:00:00'));
		const [eventID] = await seedCalendarEvents([
			{ title: '겹치는 일정', startISO: '2026-06-08T01:00:00+09:00', endISO: '2026-06-08T02:00:00+09:00' }
		]);
		try {
			await signInToCalendar(page);

			const calendarFrame = page.frameLocator('iframe');
			await expect(calendarFrame.locator('.calendar-toolbar-title')).toHaveText(/2026년 \d+월/);
			await expect(page.getByText('내 일정')).toHaveCount(0);
			await expect(page.locator('[data-mini-date-key]')).toHaveCount(0);
			await expectRouteCalendarFrameToFillContent(page);
		} finally {
			await cleanupCalendarEvents([eventID]);
		}
	});

	test('keeps event clicks inside the calendar shell', async ({ context, page }) => {
		await routeCalendarHolidays(page);
		await page.clock.setFixedTime(new Date('2026-06-15T12:00:00'));
		const [eventID] = await seedCalendarEvents([
			{ title: '클릭 확인 일정', startISO: '2026-06-15T09:00:00+09:00', endISO: '2026-06-15T10:00:00+09:00' }
		]);
		try {
			await signInToCalendar(page);

			const eventButton = page.frameLocator('iframe').locator(`[data-calendar-event-id="${eventID}"]:visible`).first();
			await expect(eventButton).toBeVisible();
			await eventButton.click();

			await expect(eventButton).toHaveAttribute('data-selected', '');
			await expect(page.frameLocator('iframe').locator('.calendar-draft-popover')).toBeVisible();
			expect(context.pages()).toHaveLength(1);
		} finally {
			await cleanupCalendarEvents([eventID]);
		}
	});

	test('renders holidays without opening edit controls', async ({ page }) => {
		await page.route('**/api/calendar/holidays?**', async (route) => {
			await route.fulfill({
				json: {
					holidays: [
						{
							id: 'holiday-2026-06-15',
							title: '공휴일',
							date: '2026-06-15',
							source: 'holiday_api',
							countryCode: 'KR',
							readOnly: true,
							color: '#ef4444'
						}
					],
					degraded: false
				}
			});
		});
		await page.clock.setFixedTime(new Date('2026-06-15T12:00:00'));
		const [eventID] = await seedCalendarEvents([
			{ title: '휴일 옆 일정', startISO: '2026-06-15T09:00:00+09:00', endISO: '2026-06-15T10:00:00+09:00' }
		]);
		try {
			await signInToCalendar(page);
			await page.goto(calendarEmbedPath);

			const holiday = page.locator('[data-calendar-event-id="holiday-2026-06-15"]:visible').first();
			await expect(holiday).toBeVisible();
			await expect(holiday).toHaveAttribute('aria-disabled', 'true');
			await holiday.click({ button: 'right', force: true });
			await expect(page.getByText('일정 삭제')).toHaveCount(0);
		} finally {
			await cleanupCalendarEvents([eventID]);
		}
	});

	test('renders approved full-day and partial leave as read-only events', async ({ page }) => {
		await routeCalendarHolidays(page);
		await page.clock.setFixedTime(new Date('2026-06-15T12:00:00'));
		const leaveIDs = await seedLeave([
			{ memberID: member1ID, kind: 'annual', days: 1, status: 'approved', startISO: '2026-06-15T00:00:00+09:00', endISO: '2026-06-16T00:00:00+09:00' },
			{ memberID: member2ID, kind: 'annual', days: 0.5, status: 'approved', startISO: '2026-06-15T14:00:00+09:00', endISO: '2026-06-15T18:00:00+09:00' }
		]);
		try {
			await signInToCalendar(page);
			await page.goto(calendarEmbedPath);

			const fullDayLeave = page.locator('[data-calendar-event-id^="leave:"]', { hasText: '이샘플' }).first();
			const partialLeave = page.locator('[data-calendar-event-id^="leave:"]', { hasText: '김예시' }).first();
			await expect(fullDayLeave).toContainText('휴가');
			await expect(partialLeave).toContainText('반차');
			await expect(fullDayLeave).toHaveAttribute('aria-disabled', 'true');
			await partialLeave.click({ force: true });
			await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		} finally {
			await cleanupLeave(leaveIDs);
		}
	});

	test('keeps calendar events visible and warns when holidays fail to load', async ({ page }) => {
		await page.route('**/api/calendar/holidays?**', async (route) => {
			await route.fulfill({ status: 503, contentType: 'text/plain', body: 'provider unavailable' });
		});
		await page.clock.setFixedTime(new Date('2026-06-15T12:00:00'));
		const [eventID] = await seedCalendarEvents([
			{ title: '공휴일 실패해도 보임', startISO: '2026-06-15T09:00:00+09:00', endISO: '2026-06-15T10:00:00+09:00' }
		]);
		try {
			await signInToCalendar(page);
			await page.goto(calendarEmbedPath);

			await expect(page.locator(`[data-calendar-event-id="${eventID}"]:visible`).first()).toBeVisible();
			await expect(page.getByRole('status')).toHaveText(
				'공휴일을 불러오지 못했습니다. 일반 일정은 계속 사용할 수 있습니다.'
			);
		} finally {
			await cleanupCalendarEvents([eventID]);
		}
	});

	test('opens the compact editor for a month event in a narrow calendar shell', async ({ context, page }) => {
		await routeCalendarHolidays(page);
		await page.setViewportSize({ width: 600, height: 900 });
		await page.clock.setFixedTime(new Date('2026-06-15T12:00:00'));
		const [eventID] = await seedCalendarEvents([
			{ title: '좁은 화면 일정', startISO: '2026-06-15T09:00:00+09:00', endISO: '2026-06-15T10:00:00+09:00' }
		]);
		try {
			await signInToCalendar(page);

			const calendarFrame = page.frameLocator('iframe');
			const eventButton = calendarFrame.locator(`[data-calendar-event-id="${eventID}"]:visible`).first();
			await expect(eventButton).toBeVisible();
			await eventButton.click();

			await expect(calendarFrame.locator('.calendar-draft-popover')).toBeVisible();
			expect(context.pages()).toHaveLength(1);
		} finally {
			await cleanupCalendarEvents([eventID]);
		}
	});

	test('says nothing about a subscription there is none of', async ({ page }) => {
		await routeCalendarHolidays(page);
		await page.route('**/api/calendar/subscription', async (route) => {
			await route.fulfill({ json: { registered: false } });
		});
		await signInToCalendar(page);

		await openCalendarSettings(page);
		await expect(page.getByText('구독 URL 준비됨')).toHaveCount(0);
		await expect(page.getByRole('button', { name: 'Copy' })).toHaveCount(0);
	});

	test('does not create horizontal overflow on calendar routes', async ({ page }) => {
		await routeCalendarHolidays(page);
		await page.setViewportSize({ width: 390, height: 844 });
		await signInToCalendar(page);
		await expect(page.locator('iframe')).toBeVisible();
		await expectHorizontalOverflow(page, false);

		await page.goto(calendarEmbedPath);
		await expect(page.locator('.calendar-toolbar-title')).toBeVisible();
		await expectHorizontalOverflow(page, false);
	});
});

async function expectRouteCalendarFrameToFillContent(page: Page): Promise<void> {
	await expect
		.poll(async () =>
			page.locator('iframe').evaluate((element) => {
				const rectangle = element.getBoundingClientRect();
				const parentRectangle = element.parentElement?.getBoundingClientRect();
				if (!parentRectangle) return false;
				return Math.abs(rectangle.left - parentRectangle.left) <= 1 && Math.abs(rectangle.width - parentRectangle.width) <= 1;
			})
		)
		.toBe(true);
}

async function openCalendarSettings(page: Page): Promise<void> {
	const calendarFrame = page.frameLocator('iframe');
	await expect(calendarFrame.locator('.calendar-toolbar-title')).toBeVisible();
	const settingsButton = calendarFrame.getByRole('button', { name: '설정' });
	const settingsHeading = page.getByRole('heading', { name: '설정' });
	await expect
		.poll(async () => {
			if ((await settingsHeading.count()) > 0) return true;
			await settingsButton.click();
			await page.waitForTimeout(100);
			return (await settingsHeading.count()) > 0;
		})
		.toBe(true);
	await expect(settingsHeading).toBeVisible();
}

async function expectHorizontalOverflow(page: Page, expected: boolean): Promise<void> {
	await expect
		.poll(async () =>
			page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)
		)
		.toBe(expected);
}
