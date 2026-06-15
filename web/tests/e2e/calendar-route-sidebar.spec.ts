import { expect, test, type Page } from '@playwright/test';

test.describe('calendar route sidebar', () => {
	test.beforeEach(async ({ page }) => {
		await routeCalendarShellAPI(page);
	});

	test('navigates the embedded calendar from the mini month', async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/');

		const calendarFrame = page.frameLocator('iframe');
		await expect(calendarFrame.getByRole('button', { name: '오늘' })).toBeVisible();
		const sidebarMonthLabel = page.getByRole('button', { name: '월과 연도 선택' });
		await expect(sidebarMonthLabel).toHaveText('2026년 6월');
		await expect(page.locator('[data-mini-date-key="2026-06-15"]')).toBeVisible();
		await page.getByRole('button', { name: '다음 달' }).click();
		await expect(sidebarMonthLabel).toHaveText('2026년 7월');
		const targetDate = page.locator('[data-mini-date-key="2026-07-15"]');
		await expect(targetDate).toBeVisible();
		await targetDate.click();
		await expect(targetDate).toHaveAttribute('aria-pressed', 'true');
		await expect(calendarFrame.getByRole('heading', { name: '2026년 7월' })).toBeVisible();

		await expect
			.poll(async () =>
				page.evaluate(() => window.localStorage.getItem('internkim.calendar.visibleDate')?.startsWith('2026-07'))
			)
			.toBe(true);
	});

	test('renders reference mini month visual states', async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/');

		const sundayHeader = page.locator('[data-mini-weekday-index="0"]');
		const todayDate = page.locator('[data-mini-date-key="2026-06-08"]');
		const weekendDate = page.locator('[data-mini-date-key="2026-06-14"]');
		const eventDotSlot = page.locator('[data-mini-date-key="2026-06-15"] .mini-month-event-dot-slot');
		const emptyDotSlot = page.locator('[data-mini-date-key="2026-06-16"] .mini-month-event-dot-slot');

		await expect(sundayHeader).toHaveAttribute('data-weekend', 'true');
		await expect(sundayHeader).toHaveClass(/text-red-400/);
		await expect(todayDate).toHaveCSS('background-color', 'rgb(239, 246, 255)');
		await expect(weekendDate).toHaveAttribute('data-weekend', 'true');
		await expect(weekendDate).toHaveClass(/text-red-400/);
		await expect(eventDotSlot).toHaveAttribute('data-has-event', 'true');
		await expect(emptyDotSlot).toHaveAttribute('data-has-event', 'false');

		await page.locator('[data-mini-date-key="2026-06-16"]').click();
		await expect(page.locator('[data-mini-date-key="2026-06-16"]')).toHaveAttribute('data-selected', 'true');
		await expect(page.locator('[data-mini-date-key="2026-06-16"]')).toHaveClass(/bg-transparent/);
		await expect(page.locator('[data-mini-date-key="2026-06-16"]')).toHaveClass(/rounded-none/);
	});

	test('aligns the sidebar header divider with the embedded calendar toolbar divider', async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/');
		const calendarFrame = page.frameLocator('iframe');
		await expect(calendarFrame.getByRole('button', { name: '오늘' })).toBeVisible();

		const sidebarHeaderHeight = await page.getByRole('button', { name: '일정 옵션 더보기' }).evaluate((element) => {
			const sidebar = element.closest('aside');
			const header = sidebar?.firstElementChild;
			if (!(header instanceof HTMLElement)) throw new Error('Calendar sidebar header must be visible');
			return header.getBoundingClientRect().height;
		});
		const toolbarHeight = await calendarFrame.locator('.calendar-toolbar').evaluate((element) => {
			if (!(element instanceof HTMLElement)) throw new Error('Calendar toolbar must be visible');
			return element.getBoundingClientRect().height;
		});

		expect(Math.abs(sidebarHeaderHeight - toolbarHeight)).toBeLessThanOrEqual(1);
	});

	test('syncs embedded toolbar date and view changes to the sidebar', async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/');

		const calendarFrame = page.frameLocator('iframe');
		await expect(calendarFrame.getByRole('button', { name: '오늘' })).toBeVisible();
		await calendarFrame.getByRole('button', { name: '주', exact: true }).click();

		await expect
			.poll(async () => page.evaluate(() => window.localStorage.getItem('internkim.calendar.view')))
			.toBe('week');
		await expect(page.locator('[data-mini-date-key="2026-06-08"]')).toHaveAttribute('data-week-range', 'middle');
		await expectReferenceWeekRangeStyle(page, '2026-06-07', 'start');
		await expectReferenceWeekRangeStyle(page, '2026-06-08', 'middle');
		await expectWeekRangeMiddleMarkerAboveRangeBackground(page, '2026-06-08');
		await expectReferenceWeekRangeStyle(page, '2026-06-13', 'end');
		await page.locator('[data-mini-date-key="2026-06-07"]').click();
		await expectSelectedWeekRangeDateUsesOutline(page, '2026-06-07', 'start');
		await expectTodayWeekRangeMarkerAboveRangeBackground(page, '2026-06-08');
		await page.locator('[data-mini-date-key="2026-06-13"]').click();
		await expectSelectedWeekRangeDateUsesOutline(page, '2026-06-13', 'end');
		await page.locator('[data-mini-date-key="2026-06-08"]').click();
		await expect(page.locator('[data-mini-date-key="2026-06-08"]')).toHaveAttribute('aria-pressed', 'true');

		await calendarFrame.getByRole('button', { name: '다음' }).click();
		await expect(page.locator('[data-mini-date-key="2026-06-15"]')).toHaveAttribute('aria-pressed', 'true');
		await expect(page.locator('[data-mini-date-key="2026-06-15"]')).toHaveAttribute('data-week-range', 'middle');
		await expectSelectedWeekRangeDateUsesOutline(page, '2026-06-15', 'middle');
		await expectWeekRangeMiddleMarkerAboveRangeBackground(page, '2026-06-15');

		await page.reload();
		const reloadedCalendarFrame = page.frameLocator('iframe');
		await expect(reloadedCalendarFrame.getByRole('button', { name: '주', exact: true })).toHaveClass(/active-view/);
		await expect(page.locator('[data-mini-date-key="2026-06-15"]')).toHaveAttribute('aria-pressed', 'true');
		await expect(page.locator('[data-mini-date-key="2026-06-15"]')).toHaveAttribute('data-week-range', 'middle');
	});

	test('keeps subscription copy actions disabled when values are empty', async ({ page }) => {
		await page.goto('/calendar/');
		await page.getByRole('button', { name: '구독 설정' }).click();

		const copyButtons = page.getByRole('button', { name: 'Copy' });
		await expect(copyButtons).toHaveCount(4);
		for (const index of [0, 1, 2, 3]) {
			await expect(copyButtons.nth(index)).toBeDisabled();
		}
	});

	test('does not create horizontal overflow on calendar routes', async ({ page }) => {
		await page.setViewportSize({ width: 390, height: 844 });
		await page.goto('/calendar/');
		await expect(page.locator('iframe')).toBeVisible();
		await expectHorizontalOverflow(page, false);

		await page.goto('/calendar/embed');
		await expect(page.getByRole('button', { name: '오늘' })).toBeVisible();
		await expectHorizontalOverflow(page, false);
	});
});

async function routeCalendarShellAPI(page: Page): Promise<void> {
	await page.route('**/admin/api/session', async (route) => {
		await route.fulfill({ json: { authenticated: true, email: 'tester@example.com' } });
	});
	await page.route('**/auth/session**', async (route) => {
		await route.fulfill({ json: { authenticated: true, email: 'tester@example.com' } });
	});
	await page.route('**/calendar/api/sync', async (route) => {
		await route.fulfill({
			json: {
				caldavURL: '',
				caldavUsername: '',
				caldavPassword: '',
				icsURL: ''
			}
		});
	});
	await page.route('**/calendar/api/account-status', async (route) => {
		await route.fulfill({ json: { connected: false, needsReauth: false } });
	});
	await page.route('**/calendar/api/events?**', async (route) => {
		await route.fulfill({
			json: {
				events: [
					{
						id: 'event-2026-06-15',
						title: 'Sidebar event',
						startISO: '2026-06-15T09:00:00.000Z',
						endISO: '2026-06-15T10:00:00.000Z',
						isAllDay: false
					}
				]
			}
		});
	});
	await page.route('**/calendar/api/remote-sync', async (route) => {
		await route.fulfill({ json: { synced: false } });
	});
	await page.route('**/calendar/api/conflicts', async (route) => {
		await route.fulfill({ json: { conflicts: [] } });
	});
	await page.route('**/admin/api/locale', async (route) => {
		await route.fulfill({ json: { locale: 'ko' } });
	});
}

async function expectHorizontalOverflow(page: Page, expected: boolean): Promise<void> {
	await expect
		.poll(async () =>
			page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)
		)
		.toBe(expected);
}

async function expectReferenceWeekRangeStyle(page: Page, dateKey: string, rangePosition: 'start' | 'middle' | 'end'): Promise<void> {
	const style = await page.evaluate((targetDateKey) => {
		const element = document.querySelector(`[data-mini-date-key="${targetDateKey}"]`);
		if (!(element instanceof HTMLElement)) return null;
		const numberElement = element.querySelector('.mini-month-date-number');
		if (!(numberElement instanceof HTMLElement)) return null;
		const buttonStyle = window.getComputedStyle(element);
		const numberStyle = window.getComputedStyle(numberElement);
		const beforeStyle = window.getComputedStyle(element, '::before');
		const afterStyle = window.getComputedStyle(element, '::after');
		return {
			buttonBackground: buttonStyle.backgroundColor,
			numberColor: numberStyle.color,
			beforeBackground: beforeStyle.backgroundColor,
			beforeContent: beforeStyle.content,
			beforeLeft: beforeStyle.left,
			beforeRight: beforeStyle.right,
			afterContent: afterStyle.content,
			isSelected: element.getAttribute('data-selected'),
			isToday: element.getAttribute('data-today'),
			rangePosition: element.getAttribute('data-week-range')
		};
	}, dateKey);

	expect(style).not.toBeNull();
	expect(style?.rangePosition).toBe(rangePosition);
	if (style?.isToday === 'true' && style?.isSelected !== 'true') expect(style?.buttonBackground).toBe('rgb(239, 246, 255)');
	else expect(style?.buttonBackground).toBe('rgba(0, 0, 0, 0)');
	expect(style?.beforeContent).toBe('""');
	expect(style?.beforeBackground).toBe('rgb(229, 239, 251)');
	expect(style?.beforeLeft).toBe('0px');
	expect(style?.beforeRight).toBe('0px');
	expect(['none', '']).toContain(style?.afterContent);
	expect(style?.numberColor).not.toBe('rgb(255, 255, 255)');
}

async function expectSelectedWeekRangeDateUsesOutline(
	page: Page,
	dateKey: string,
	rangePosition: 'start' | 'middle' | 'end'
): Promise<void> {
	const style = await page.evaluate((targetDateKey) => {
		const element = document.querySelector(`[data-mini-date-key="${targetDateKey}"]`);
		if (!(element instanceof HTMLElement)) return null;
		const numberElement = element.querySelector('.mini-month-date-number');
		if (!(numberElement instanceof HTMLElement)) return null;
		const buttonStyle = window.getComputedStyle(element);
		const numberStyle = window.getComputedStyle(numberElement);
		return {
			borderRadius: buttonStyle.borderRadius,
			outlineColor: buttonStyle.outlineColor,
			outlineOffset: buttonStyle.outlineOffset,
			outlineStyle: buttonStyle.outlineStyle,
			outlineWidth: buttonStyle.outlineWidth,
			numberColor: numberStyle.color,
			boxShadow: buttonStyle.boxShadow,
			isSelected: element.getAttribute('data-selected'),
			rangePosition: element.getAttribute('data-week-range')
		};
	}, dateKey);

	expect(style).not.toBeNull();
	expect(style?.isSelected).toBe('true');
	expect(style?.rangePosition).toBe(rangePosition);
	expect(style?.boxShadow).toBe('none');
	expect(style?.outlineColor).toBe('rgb(80, 150, 232)');
	expect(style?.outlineOffset).toBe('-1px');
	expect(style?.outlineStyle).toBe('solid');
	expect(style?.outlineWidth).toBe('1px');
	expect(style?.borderRadius).toBe('0px');
	expect(style?.numberColor).not.toBe('rgb(255, 255, 255)');
}

async function expectWeekRangeMiddleMarkerAboveRangeBackground(page: Page, dateKey: string): Promise<void> {
	const style = await page.evaluate((targetDateKey) => {
		const element = document.querySelector(`[data-mini-date-key="${targetDateKey}"]`);
		if (!(element instanceof HTMLElement)) return null;
		const numberElement = element.querySelector('.mini-month-date-number');
		if (!(numberElement instanceof HTMLElement)) return null;
		const beforeStyle = window.getComputedStyle(element, '::before');
		const afterStyle = window.getComputedStyle(element, '::after');
		const numberStyle = window.getComputedStyle(numberElement);
		return {
			afterContent: afterStyle.content,
			beforeBackground: beforeStyle.backgroundColor,
			beforeContent: beforeStyle.content,
			beforeZIndex: beforeStyle.zIndex,
			numberZIndex: numberStyle.zIndex
		};
	}, dateKey);

	expect(style).not.toBeNull();
	expect(style?.beforeContent).toBe('""');
	expect(style?.beforeBackground).toBe('rgb(229, 239, 251)');
	expect(style?.beforeZIndex).toBe('-1');
	expect(['none', '']).toContain(style?.afterContent);
	expect(style?.numberZIndex).toBe('1');
}

async function expectTodayWeekRangeMarkerAboveRangeBackground(page: Page, dateKey: string): Promise<void> {
	const style = await page.evaluate((targetDateKey) => {
		const element = document.querySelector(`[data-mini-date-key="${targetDateKey}"]`);
		if (!(element instanceof HTMLElement)) return null;
		const numberElement = element.querySelector('.mini-month-date-number');
		if (!(numberElement instanceof HTMLElement)) return null;
		const beforeStyle = window.getComputedStyle(element, '::before');
		const afterStyle = window.getComputedStyle(element, '::after');
		const buttonStyle = window.getComputedStyle(element);
		const numberStyle = window.getComputedStyle(numberElement);
		return {
			afterContent: afterStyle.content,
			background: buttonStyle.backgroundColor,
			borderRadius: buttonStyle.borderRadius,
			beforeBackground: beforeStyle.backgroundColor,
			beforeContent: beforeStyle.content,
			beforeZIndex: beforeStyle.zIndex,
			outlineStyle: buttonStyle.outlineStyle,
			isSelected: element.getAttribute('data-selected'),
			isToday: element.getAttribute('data-today'),
			numberColor: numberStyle.color,
			numberFontWeight: numberStyle.fontWeight,
			numberZIndex: numberStyle.zIndex,
			rangePosition: element.getAttribute('data-week-range')
		};
	}, dateKey);

	expect(style).not.toBeNull();
	expect(style?.isSelected).toBe('false');
	expect(style?.isToday).toBe('true');
	expect(style?.rangePosition).toBe('middle');
	expect(style?.beforeContent).toBe('""');
	expect(style?.beforeBackground).toBe('rgb(229, 239, 251)');
	expect(style?.beforeZIndex).toBe('-1');
	expect(['none', '']).toContain(style?.afterContent);
	expect(style?.background).toBe('rgb(239, 246, 255)');
	expect(style?.borderRadius).toBe('0px');
	expect(style?.outlineStyle).toBe('none');
	expect(style?.numberColor).toBe('rgb(29, 78, 216)');
	expect(Number(style?.numberFontWeight)).toBeGreaterThanOrEqual(700);
	expect(style?.numberZIndex).toBe('1');
}
