import { expect, test, type Locator, type Page } from '@playwright/test';
import { routeCalendarShellAPI } from './calendar-route-shell-test-utils';

test.describe('calendar route shell', () => {
	test.beforeEach(async ({ page }) => {
		await routeCalendarShellAPI(page);
	});

	test('renders the embedded calendar without the old sidebar', async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/');

		const calendarFrame = page.frameLocator('iframe');
		await expect(calendarFrame.locator('.calendar-toolbar-title')).toHaveText('2026년 6월');
		await expect(page.getByText('내 일정')).toHaveCount(0);
		await expect(page.locator('[data-mini-date-key]')).toHaveCount(0);
		await expectRouteCalendarFrameToFillContent(page);
	});

	test('keeps event clicks inside the calendar shell', async ({ context, page }) => {
		await page.clock.setFixedTime(new Date('2026-06-15T12:00:00'));
		await page.goto('/calendar/');

		const eventButton = page.frameLocator('iframe').locator('[data-event-id="event-2026-06-15"]:visible').first();
		await expect(eventButton).toBeVisible();
		await eventButton.click();

		await expect(eventButton).toHaveClass(/internkim-calendar-event-focused/);
		await expect(page.frameLocator('iframe').locator('.calendar-draft-popover')).toHaveCount(0);
		expect(context.pages()).toHaveLength(1);
	});

	test('opens an event popover on double click inside the calendar shell', async ({ context, page }) => {
		await page.clock.setFixedTime(new Date('2026-06-15T12:00:00'));
		await page.goto('/calendar/');

		const calendarFrame = page.frameLocator('iframe');
		const eventButton = calendarFrame.locator('[data-event-id="event-2026-06-15"]:visible').first();
		await expect(eventButton).toBeVisible();
		await eventButton.dblclick();

		await expect(calendarFrame.locator('.calendar-draft-popover')).toBeVisible();
		expect(context.pages()).toHaveLength(1);
	});

	test('opens subscription settings from the embedded toolbar', async ({ page }) => {
		await page.goto('/calendar/');

		await openCalendarSettings(page);
		await expect(page.getByText('CalDAV/ICS 구독 URL 준비됨')).toBeVisible();
		await expect(page.getByText('연결된 Google 캘린더')).toHaveCount(0);
		const copyButtons = page.getByRole('button', { name: 'Copy' });
		await expect(copyButtons).toHaveCount(4);
		for (const index of [0, 1, 2, 3]) {
			await expect(copyButtons.nth(index)).toBeDisabled();
		}
	});

	test('refreshes the embedded calendar from the toolbar', async ({ page }) => {
		let eventRequestCount = 0;
		page.on('request', (request) => {
			if (request.url().includes('/calendar/api/events?')) eventRequestCount += 1;
		});
		await page.goto('/calendar/');

		const calendarFrame = page.frameLocator('iframe');
		await expect(calendarFrame.locator('.calendar-toolbar-title')).toBeVisible();
		await expect.poll(() => eventRequestCount).toBeGreaterThan(0);
		await calendarFrame.getByRole('button', { name: '새로고침' }).click();

		await expect.poll(() => eventRequestCount).toBeGreaterThan(1);
		await expect(page.locator('iframe')).toBeVisible();
		await expectRouteCalendarFrameToFillContent(page);
	});

	test('orders toolbar actions and opens the month year picker from the title', async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/');

		const calendarFrame = page.frameLocator('iframe');
		const refreshButton = calendarFrame.getByRole('button', { name: '새로고침' });
		const settingsButton = calendarFrame.getByRole('button', { name: '설정' });
		const searchInput = calendarFrame.getByRole('textbox', { name: '일정 검색' });
		const searchBox = calendarFrame.locator('.calendar-search-shell');
		await expectToolbarOrder(refreshButton, settingsButton, searchInput);
		await expectToolbarAdjacent(settingsButton, searchBox);
		await expect(calendarFrame.locator('.calendar-toolbar-title')).toHaveCSS('font-size', '20px');

		await calendarFrame.getByRole('button', { name: '2026년 6월' }).click();
		const picker = calendarFrame.getByRole('dialog', { name: '월과 연도 선택' });
		await expect(picker).toBeVisible();
		await picker.getByRole('button', { name: '7월' }).click();
		await expect(calendarFrame.locator('.calendar-toolbar-title')).toHaveText('2026년 7월');
	});

	test('does not create horizontal overflow on calendar routes', async ({ page }) => {
		await page.setViewportSize({ width: 390, height: 844 });
		await page.goto('/calendar/');
		await expect(page.locator('iframe')).toBeVisible();
		await expectHorizontalOverflow(page, false);

		await page.goto('/calendar/embed');
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

async function expectToolbarOrder(leftLocator: Locator, middleLocator: Locator, rightLocator: Locator): Promise<void> {
	await expect(leftLocator).toBeVisible();
	await expect(middleLocator).toBeVisible();
	await expect(rightLocator).toBeVisible();
	const [leftBox, middleBox, rightBox] = await Promise.all([
		leftLocator.boundingBox(),
		middleLocator.boundingBox(),
		rightLocator.boundingBox()
	]);
	if (!leftBox || !middleBox || !rightBox) throw new Error('Missing toolbar element box');
	expect(leftBox.x).toBeLessThan(middleBox.x);
	expect(middleBox.x).toBeLessThan(rightBox.x);
}

async function expectToolbarAdjacent(leftLocator: Locator, rightLocator: Locator): Promise<void> {
	const [leftBox, rightBox] = await Promise.all([leftLocator.boundingBox(), rightLocator.boundingBox()]);
	if (!leftBox || !rightBox) throw new Error('Missing toolbar element box');
	expect(rightBox.x - (leftBox.x + leftBox.width)).toBeLessThanOrEqual(12);
}
