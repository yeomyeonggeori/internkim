import { expect, test, type Page } from '@playwright/test';
import {
	routeCalendarEventCreates,
	routeCalendarEventDeletes,
	routeCalendarEventUpdates,
	routeCalendarEvents,
	routeDefaultCalendarAPI
} from './calendar-embed-test-utils';
import { navigateEmbeddedCalendar, openCalendarEmbed } from './calendar-embed-interaction-helpers';

const mobileEventHorizontalInset = 4;

type WeekGridMeasurements = {
	allDayCellRects: ElementRect[];
	allDayEventRects: EventRect[];
	allDayLabelText: string;
	allDayBottom: number;
	allDayContentBackground: string;
	allDayContentBackgroundImage: string;
	allDayRightEdges: number[];
	allDayShellBackground: string;
	allDayShellOpacity: string;
	compactHeaderBackground: string;
	compactDateTexts: string[];
	compactHeaderLabelColors: string[];
	customHeaderBackgrounds: string[];
	customHeaderBorderColors: string[];
	customHeaderColors: string[];
	customHeaderTexts: string[];
	desktopHeaderLabels: string[];
	desktopHeaderRightEdges: number[];
	hasCompactHeaderBottomLine: boolean;
	highlightedDateTexts: string[];
	isAllDayLabelPainted: boolean;
	firstVisibleTimeLabel: string;
	rangePillBackgrounds: string[];
	rangePillColors: string[];
	stageBackground: string;
	timeCellBorderColors: string[];
	timeScrollerBackground: string;
	timeScrollerBackgroundImage: string;
	timeCellRects: ElementRect[];
	timedEventRects: EventRect[];
	timeRightEdges: number[];
	timeTop: number;
};

type ElementRect = {
	left: number;
	right: number;
	width: number;
};

type EventRect = ElementRect & {
	id: string;
};

test.describe('embedded calendar mobile two-day week view', () => {
	test.beforeEach(async ({ page }) => {
		await routeDefaultCalendarAPI(page);
		await routeCalendarEvents(page, [
			{
				id: 'mobile-two-day-first',
				title: '첫째 날 일정',
				startISO: '2026-06-04T10:00:00+09:00',
				endISO: '2026-06-04T11:00:00+09:00',
				isAllDay: false
			},
			{
				id: 'mobile-two-day-second',
				title: '둘째 날 일정',
				startISO: '2026-06-05T14:00:00+09:00',
				endISO: '2026-06-05T15:00:00+09:00',
				isAllDay: false
			},
			{
				id: 'mobile-two-day-edit',
				title: '수정할 일정',
				startISO: '2026-06-01T02:00:00+09:00',
				endISO: '2026-06-01T03:00:00+09:00',
				isAllDay: false
			},
			{
				id: 'mobile-two-day-all-day-one',
				title: '1',
				startISO: '2026-06-02T00:00:00+09:00',
				endISO: '2026-06-03T00:00:00+09:00',
				isAllDay: true
			},
			{
				id: 'mobile-two-day-all-day-two',
				title: '2',
				startISO: '2026-06-02T00:00:00+09:00',
				endISO: '2026-06-03T00:00:00+09:00',
				isAllDay: true
			}
		]);
	});

	test('shows two selected week columns on mobile while preserving seven desktop columns', async ({ page }) => {
		await page.setViewportSize({ width: 390, height: 844 });
		await openCalendarEmbed(page, '주');
		await navigateEmbeddedCalendar(page, '2026-06-01');

		const mobileMeasurements = await weekGridMeasurements(page);
		expect(mobileMeasurements.customHeaderTexts).toEqual(['6월 1일 월', '6월 2일 화']);
		expect(mobileMeasurements.customHeaderColors).toEqual(['rgb(100, 116, 139)', 'rgb(100, 116, 139)']);
		expect(mobileMeasurements.compactDateTexts).toEqual(['31', '1', '2', '3', '4', '5', '6']);
		expect(mobileMeasurements.compactHeaderLabelColors[0]).toBe('rgb(239, 68, 68)');
		expect(mobileMeasurements.compactHeaderLabelColors[6]).toBe('rgb(239, 68, 68)');
		expect(mobileMeasurements.compactHeaderLabelColors.slice(1, 6)).not.toContain('rgb(239, 68, 68)');
		expect(mobileMeasurements.highlightedDateTexts).toEqual(['1', '2']);
		expect(mobileMeasurements.hasCompactHeaderBottomLine).toBe(true);
		expect(mobileMeasurements.allDayLabelText).toBe('종일');
		expect(mobileMeasurements.allDayShellOpacity).toBe('1');
		expect(mobileMeasurements.isAllDayLabelPainted).toBe(true);
		expect(mobileMeasurements.firstVisibleTimeLabel).toBe('01:00');
		expect(mobileMeasurements.allDayContentBackgroundImage).not.toContain('linear-gradient');
		expect(mobileMeasurements.timeScrollerBackgroundImage).not.toContain('linear-gradient');
		expect(mobileMeasurements.allDayRightEdges).toHaveLength(2);
		expect(mobileMeasurements.timeRightEdges).toHaveLength(2);
		expect(maximumEdgeDelta(mobileMeasurements.allDayRightEdges, mobileMeasurements.timeRightEdges)).toBeLessThanOrEqual(1);
		expect(Math.abs(mobileMeasurements.allDayBottom - mobileMeasurements.timeTop)).toBeLessThanOrEqual(1);
		expect(mobileMeasurements.allDayEventRects.map((rect) => rect.id).sort()).toEqual([
			'mobile-two-day-all-day-one',
			'mobile-two-day-all-day-two'
		]);
		const secondAllDayCell = mobileMeasurements.allDayCellRects[1];
		for (const eventRect of mobileMeasurements.allDayEventRects) {
			expect(Math.abs(eventRect.left - (secondAllDayCell.left + mobileEventHorizontalInset))).toBeLessThanOrEqual(1);
			expect(Math.abs(eventRect.right - (secondAllDayCell.right - mobileEventHorizontalInset))).toBeLessThanOrEqual(1);
			expect(Math.abs(eventRect.width - (secondAllDayCell.width - mobileEventHorizontalInset * 2))).toBeLessThanOrEqual(1);
		}
		const firstTimeCell = mobileMeasurements.timeCellRects[0];
		const timedEvent = mobileMeasurements.timedEventRects.find((rect) => rect.id === 'mobile-two-day-edit');
		expect(timedEvent).toBeDefined();
		expect(Math.abs((timedEvent?.left ?? 0) - (firstTimeCell.left + mobileEventHorizontalInset))).toBeLessThanOrEqual(1);
		expect(Math.abs((timedEvent?.right ?? 0) - (firstTimeCell.right - mobileEventHorizontalInset))).toBeLessThanOrEqual(1);
		expect(Math.abs((timedEvent?.width ?? 0) - (firstTimeCell.width - mobileEventHorizontalInset * 2))).toBeLessThanOrEqual(1);

		await page.locator('.df-compact-header-date-button').filter({ hasText: /^3$/ }).click();
		await expect.poll(() => selectedDateKey(page)).toBe('2026-06-03');
		const movedMobileMeasurements = await weekGridMeasurements(page);
		expect(movedMobileMeasurements.highlightedDateTexts).toEqual(['3', '4']);
		expect(movedMobileMeasurements.customHeaderTexts).toEqual(['6월 3일 수', '6월 4일 목']);
		expect(movedMobileMeasurements.allDayEventRects).toHaveLength(0);

		await page.locator('.df-compact-header-date-button').filter({ hasText: /^6$/ }).click();
		await expect.poll(() => selectedDateKey(page)).toBe('2026-06-06');
		const endOfWeekMobileMeasurements = await weekGridMeasurements(page);
		expect(endOfWeekMobileMeasurements.highlightedDateTexts).toEqual(['5', '6']);
		expect(endOfWeekMobileMeasurements.allDayRightEdges).toHaveLength(2);
		expect(endOfWeekMobileMeasurements.timeRightEdges).toHaveLength(2);

		await page.setViewportSize({ width: 1280, height: 900 });
		await page.reload();
		await navigateEmbeddedCalendar(page, '2026-06-04');

		const desktopMeasurements = await weekGridMeasurements(page);
		expect(desktopMeasurements.compactDateTexts).toHaveLength(0);
		expect(desktopMeasurements.customHeaderTexts).toHaveLength(0);
		expect(desktopMeasurements.desktopHeaderLabels).toHaveLength(7);
		expect(desktopMeasurements.allDayRightEdges).toHaveLength(7);
		expect(desktopMeasurements.timeRightEdges).toHaveLength(7);
		expect(maximumEdgeDelta(desktopMeasurements.desktopHeaderRightEdges, desktopMeasurements.timeRightEdges)).toBeLessThanOrEqual(1);
		expect(Math.abs(desktopMeasurements.allDayBottom - desktopMeasurements.timeTop)).toBeLessThanOrEqual(1);
	});

	test('keeps mobile two-day all-day row and grid colors consistent in dark mode', async ({ page }) => {
		await page.setViewportSize({ width: 390, height: 844 });
		await openCalendarEmbed(page, '주');
		await navigateEmbeddedCalendar(page, '2026-06-01');
		await enableDarkMode(page);

		const mobileMeasurements = await weekGridMeasurements(page);
		expect(mobileMeasurements.stageBackground).toBe('rgb(9, 9, 11)');
		expect(mobileMeasurements.compactHeaderBackground).toBe(mobileMeasurements.stageBackground);
		expect(mobileMeasurements.allDayShellBackground).toBe(mobileMeasurements.stageBackground);
		expect(mobileMeasurements.allDayContentBackground).toBe(mobileMeasurements.stageBackground);
		expect(mobileMeasurements.timeScrollerBackground).toBe(mobileMeasurements.stageBackground);
		expect(mobileMeasurements.customHeaderBackgrounds).toEqual([
			mobileMeasurements.stageBackground,
			mobileMeasurements.stageBackground
		]);
		expect(mobileMeasurements.customHeaderColors).toEqual(['rgb(161, 161, 170)', 'rgb(161, 161, 170)']);
		expect(mobileMeasurements.customHeaderBorderColors).toEqual(['rgb(39, 39, 42)', 'rgb(39, 39, 42)']);
		expect(mobileMeasurements.timeCellBorderColors).toEqual(['rgb(39, 39, 42)', 'rgb(39, 39, 42)']);
		expect(new Set(mobileMeasurements.rangePillColors).size).toBe(1);
		expect(mobileMeasurements.rangePillColors).not.toContain('rgb(30, 58, 138)');
		expect(mobileMeasurements.rangePillBackgrounds).not.toContain('rgb(219, 234, 254)');
	});

	test('uses the DayFlow mobile editor while keeping the desktop draft popover', async ({ page }) => {
		await page.setViewportSize({ width: 390, height: 844 });
		await openCalendarEmbed(page, '주');
		await navigateEmbeddedCalendar(page, '2026-06-01');

		await doubleClickFirstVisibleTimeCell(page);
		await expect(dayFlowMobileEditor(page)).toBeVisible();
		await expect.poll(() => activeMobileEditorField(page)).toBe('title');
		await expect(dayFlowMobileEditor(page).getByRole('button', { name: '취소' })).toBeVisible();
		await expect(dayFlowMobileEditor(page).getByText('새 일정')).toBeVisible();
		await expect(dayFlowMobileEditor(page).getByRole('button', { name: '완료' })).toBeVisible();
		await expect(dayFlowMobileEditor(page).getByText('시작 날짜')).toBeVisible();
		await expect(dayFlowMobileEditor(page).getByText('종일')).toBeVisible();
		await expect(dayFlowMobileEditor(page).getByText('장소')).toBeVisible();
		await expect(dayFlowMobileEditor(page).getByRole('button', { name: '삭제' })).toBeVisible();
		await verifyMobileEditorControls(page);
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		await closeDayFlowMobileEditor(page);
		await expect.poll(async () => (await renderedEventIDs(page)).some((eventID) => eventID.startsWith('timeline-'))).toBe(true);

		await page.locator('[data-event-id="mobile-two-day-edit"]').first().dblclick();
		await expect(dayFlowMobileEditor(page)).toBeVisible();
		await expect(dayFlowMobileEditor(page).getByText('일정 편집')).toBeVisible();
		await expect(dayFlowMobileEditor(page).getByRole('button', { name: '삭제' })).toBeVisible();
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		await closeDayFlowMobileEditor(page);

		await page.getByRole('button', { name: /새로 만들기/ }).click();
		await expect(dayFlowMobileEditor(page)).toBeVisible();
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		await closeDayFlowMobileEditor(page);

		await page.setViewportSize({ width: 1280, height: 900 });
		await page.reload();
		await navigateEmbeddedCalendar(page, '2026-06-01');

		await doubleClickFirstVisibleTimeCell(page);
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await expect(dayFlowMobileEditor(page)).toHaveCount(0);
	});

	test('localizes mobile two-day headers and the mobile event editor', async ({ page }) => {
		await routeDefaultCalendarAPI(page, 'en');
		await routeCalendarEvents(page, [
			{
				id: 'mobile-two-day-english-edit',
				title: 'Edit in English',
				startISO: '2026-06-01T02:00:00+09:00',
				endISO: '2026-06-01T03:00:00+09:00',
				isAllDay: false
			}
		]);
		await page.setViewportSize({ width: 390, height: 844 });
		await openCalendarEmbed(page, '주');
		await navigateEmbeddedCalendar(page, '2026-06-01');

		const mobileMeasurements = await weekGridMeasurements(page);
		expect(mobileMeasurements.customHeaderTexts).toEqual(['Mon, Jun 1', 'Tue, Jun 2']);

		await page.locator('[data-event-id="mobile-two-day-english-edit"]').first().dblclick();
		await expect(dayFlowMobileEditor(page)).toBeVisible();
		await expect(dayFlowMobileEditor(page).getByRole('button', { name: 'Cancel' })).toBeVisible();
		await expect(dayFlowMobileEditor(page).getByText('Edit Event')).toBeVisible();
		await expect(dayFlowMobileEditor(page).getByRole('button', { name: 'Done' })).toBeVisible();
		await expect(dayFlowMobileEditor(page).getByText('Start date')).toBeVisible();
		await expect(dayFlowMobileEditor(page).getByText('All day')).toBeVisible();
		await expect(dayFlowMobileEditor(page).getByText('Location')).toBeVisible();
		await expect(dayFlowMobileEditor(page).getByRole('button', { name: 'Delete' })).toBeVisible();
	});

	test('saves mobile-created events through the calendar persistence path', async ({ page }) => {
		const createdEvents = await routeCalendarEventCreates(page);
		await page.setViewportSize({ width: 390, height: 844 });
		await openCalendarEmbed(page, '주');
		await navigateEmbeddedCalendar(page, '2026-06-01');

		await page.getByRole('button', { name: /새로 만들기/ }).click();
		await expect(dayFlowMobileEditor(page)).toBeVisible();
		await expect.poll(() => activeMobileEditorField(page)).toBe('title');
		await dayFlowMobileEditor(page).locator('input[data-mobile-editor-field="title"]').fill('모바일 저장 일정');
		await dayFlowMobileEditor(page).getByRole('button', { name: '완료' }).click();
		await expect(dayFlowMobileEditor(page)).toHaveCount(0);
		await expect.poll(() => createdEvents.length).toBe(1);
		expect(createdEvents[0]?.title).toBe('모바일 저장 일정');
	});

	test('updates mobile-edited events through the calendar persistence path', async ({ page }) => {
		const updatedEvents = await routeCalendarEventUpdates(page);
		await page.setViewportSize({ width: 390, height: 844 });
		await openCalendarEmbed(page, '주');
		await navigateEmbeddedCalendar(page, '2026-06-01');

		await page.locator('[data-event-id="mobile-two-day-edit"]').first().dblclick();
		await expect(dayFlowMobileEditor(page)).toBeVisible();
		await dayFlowMobileEditor(page).locator('input[data-mobile-editor-field="title"]').fill('모바일 수정 일정');
		await dayFlowMobileEditor(page).getByRole('button', { name: '완료' }).click();

		await expect(dayFlowMobileEditor(page)).toHaveCount(0);
		await expect.poll(() => updatedEvents.length).toBe(1);
		expect(updatedEvents[0]?.eventID).toBe('mobile-two-day-edit');
		expect(updatedEvents[0]?.title).toBe('모바일 수정 일정');
	});

	test('deletes mobile-edited events through the calendar persistence path', async ({ page }) => {
		const deletedEventIDs = await routeCalendarEventDeletes(page);
		await page.setViewportSize({ width: 390, height: 844 });
		await openCalendarEmbed(page, '주');
		await navigateEmbeddedCalendar(page, '2026-06-01');

		await page.locator('[data-event-id="mobile-two-day-edit"]').first().dblclick();
		await expect(dayFlowMobileEditor(page)).toBeVisible();
		await dayFlowMobileEditor(page).getByRole('button', { name: '삭제' }).click();

		await expect(dayFlowMobileEditor(page)).toHaveCount(0);
		await expect.poll(() => deletedEventIDs).toEqual(['mobile-two-day-edit']);
		await expect(page.locator('[data-event-id="mobile-two-day-edit"]')).toHaveCount(0);
	});
});

async function selectedDateKey(page: Page): Promise<string> {
	return page.evaluate(() => document.querySelector<HTMLElement>('.calendar-stage')?.dataset.calendarSelectedDateKey ?? '');
}

async function activeMobileEditorField(page: Page): Promise<string> {
	return page.evaluate(() => {
		const element = document.activeElement;
		return element instanceof HTMLElement ? (element.dataset.mobileEditorField ?? '') : '';
	});
}

async function renderedEventIDs(page: Page): Promise<string[]> {
	return page.locator('[data-event-id]').evaluateAll((elements) =>
		elements
			.map((element) => element.getAttribute('data-event-id'))
			.filter((eventID): eventID is string => Boolean(eventID))
	);
}

async function enableDarkMode(page: Page): Promise<void> {
	await page.evaluate(() => {
		document.documentElement.classList.add('dark');
	});
	await page.evaluate(
		() =>
			new Promise<void>((resolve) => {
				requestAnimationFrame(() => requestAnimationFrame(() => resolve()));
			})
	);
}

async function verifyMobileEditorControls(page: Page): Promise<void> {
	const editor = dayFlowMobileEditor(page);
	const startDateInput = editor.locator('input[data-mobile-editor-field="startDate"]');
	const startTimeInput = editor.locator('input[data-mobile-editor-field="startTime"]');
	const allDayInput = editor.locator('input[data-mobile-editor-field="allDay"]');
	await startDateInput.click();
	await expect.poll(() => activeMobileEditorField(page)).toBe('startDate');
	await startDateInput.fill('2026-06-02');
	await expect(startDateInput).toHaveValue('2026-06-02');
	await startTimeInput.click();
	await expect.poll(() => activeMobileEditorField(page)).toBe('startTime');
	await startTimeInput.fill('11:15');
	await expect(startTimeInput).toHaveValue('11:15');
	await editor.locator('[data-mobile-editor-switch-field]').click();
	await expect(allDayInput).toBeChecked();
	await expect(startTimeInput).toHaveCount(0);
}

async function weekGridMeasurements(page: Page): Promise<WeekGridMeasurements> {
	return page.evaluate(() => {
		const isVisibleElement = (element: HTMLElement): boolean => {
			const rectangle = element.getBoundingClientRect();
			const style = window.getComputedStyle(element);
			return rectangle.width > 0 && rectangle.height > 0 && style.display !== 'none' && style.visibility !== 'hidden';
		};
		const visibleElements = (selector: string): HTMLElement[] =>
			Array.from(document.querySelectorAll<HTMLElement>(selector)).filter(isVisibleElement);
		const elementRect = (element: HTMLElement): ElementRect => {
			const rectangle = element.getBoundingClientRect();
			return {
				left: Math.round(rectangle.left * 100) / 100,
				right: Math.round(rectangle.right * 100) / 100,
				width: Math.round(rectangle.width * 100) / 100
			};
		};
		const rightEdges = (elements: HTMLElement[]): number[] =>
			elements.map((element) => Math.round(element.getBoundingClientRect().right * 100) / 100);
		const compactDateButtons = visibleElements('.df-compact-header-date-button');
		const highlightedDateButtons = visibleElements('.df-compact-header-date-button.calendar-mobile-two-day-week-range-date');
		const desktopHeaderCells = visibleElements('.df-week-header > .df-week-day-cell');
		const allDayCells = visibleElements('.df-week-all-day-row-content > .df-week-all-day-cell, .df-all-day-row > .df-all-day-cell');
		const firstTimeRow = document.querySelector<HTMLElement>('.df-time-grid-row');
		const timeCells = Array.from(firstTimeRow?.children ?? []).filter(
			(element): element is HTMLElement =>
				element instanceof HTMLElement && element.classList.contains('df-week-time-grid-cell') && isVisibleElement(element)
		);
		const allDayEvents = visibleElements('.df-week-all-day-event-layer [data-event-id]');
		const timedEvents = visibleElements('.df-week-event.df-event-timed[data-event-id]');
		const compactHeader = document.querySelector<HTMLElement>('.df-compact-header');
		const compactHeaderStyle = compactHeader ? window.getComputedStyle(compactHeader) : null;
		const compactHeaderLabels = Array.from(document.querySelectorAll<HTMLElement>('.df-compact-header-label'));
		const rangePills = visibleElements('.df-compact-header-date-button.calendar-mobile-two-day-week-range-date .df-compact-header-date-pill');
		const allDayShell = document.querySelector<HTMLElement>('.df-week-all-day-shell');
		const allDayShellStyle = allDayShell ? window.getComputedStyle(allDayShell) : null;
		const allDayContentWrap = document.querySelector<HTMLElement>('.df-week-all-day-content-wrap');
		const allDayContentWrapStyle = allDayContentWrap ? window.getComputedStyle(allDayContentWrap) : null;
		const timeScroller = document.querySelector<HTMLElement>('.df-week-time-grid-scroller');
		const timeScrollerStyle = timeScroller ? window.getComputedStyle(timeScroller) : null;
		const stage = document.querySelector<HTMLElement>('.calendar-stage');
		const stageStyle = stage ? window.getComputedStyle(stage) : null;
		const allDayLabel = document.querySelector<HTMLElement>('.df-week-all-day-label, .df-all-day-label');
		const allDayLabelRectangle = allDayLabel?.getBoundingClientRect();
		const topElementAtAllDayLabel =
			allDayLabelRectangle && allDayLabelRectangle.width > 0 && allDayLabelRectangle.height > 0
				? document.elementFromPoint(
						allDayLabelRectangle.left + allDayLabelRectangle.width / 2,
						allDayLabelRectangle.top + allDayLabelRectangle.height / 2
					)
				: null;
		const firstVisibleTimeLabel =
			Array.from(document.querySelectorAll<HTMLElement>('.df-week-time-grid-time-column .df-time-label')).find((element) => {
				const rectangle = element.getBoundingClientRect();
				const style = window.getComputedStyle(element);
				return rectangle.width > 0 && rectangle.height > 0 && style.display !== 'none' && style.visibility !== 'hidden';
			})?.textContent ?? '';
		return {
			allDayCellRects: allDayCells.map(elementRect),
			allDayEventRects: allDayEvents.map((element) => ({
				id: element.dataset.eventId ?? '',
				...elementRect(element)
			})),
			allDayLabelText: allDayLabel?.textContent?.replace(/\s+/g, ' ').trim() ?? '',
			allDayBottom: Math.round((allDayShell?.getBoundingClientRect().bottom ?? 0) * 100) / 100,
			allDayContentBackground: allDayContentWrapStyle?.backgroundColor ?? '',
			allDayContentBackgroundImage: allDayContentWrapStyle?.backgroundImage ?? '',
			allDayRightEdges: rightEdges(allDayCells),
			allDayShellBackground: allDayShellStyle?.backgroundColor ?? '',
			allDayShellOpacity: allDayShellStyle?.opacity ?? '',
			compactHeaderBackground: compactHeaderStyle?.backgroundColor ?? '',
			compactDateTexts: compactDateButtons.map((element) => element.textContent?.replace(/\s+/g, ' ').trim() ?? ''),
			compactHeaderLabelColors: compactHeaderLabels.map((element) => window.getComputedStyle(element).color),
			customHeaderBackgrounds: visibleElements('.calendar-mobile-two-day-week-column-header-cell').map(
				(element) => window.getComputedStyle(element.parentElement ?? element).backgroundColor
			),
			customHeaderBorderColors: visibleElements('.calendar-mobile-two-day-week-column-header-cell').map(
				(element) => window.getComputedStyle(element).borderRightColor
			),
			customHeaderTexts: visibleElements('.calendar-mobile-two-day-week-column-header-cell').map(
				(element) => element.textContent?.replace(/\s+/g, ' ').trim() ?? ''
			),
			customHeaderColors: visibleElements('.calendar-mobile-two-day-week-column-header-cell').map(
				(element) => window.getComputedStyle(element).color
			),
			desktopHeaderLabels: desktopHeaderCells.map((element) => element.textContent?.replace(/\s+/g, ' ').trim() ?? ''),
			desktopHeaderRightEdges: rightEdges(desktopHeaderCells),
			hasCompactHeaderBottomLine:
				Number.parseFloat(compactHeaderStyle?.borderBottomWidth ?? '0') >= 1 &&
				compactHeaderStyle?.borderBottomColor !== 'rgba(0, 0, 0, 0)',
			highlightedDateTexts: highlightedDateButtons.map((element) => element.textContent?.replace(/\s+/g, ' ').trim() ?? ''),
			isAllDayLabelPainted: Boolean(allDayLabel && topElementAtAllDayLabel && allDayLabel.contains(topElementAtAllDayLabel)),
			firstVisibleTimeLabel: firstVisibleTimeLabel.replace(/\s+/g, ' ').trim(),
			rangePillBackgrounds: rangePills.map((element) => window.getComputedStyle(element).backgroundColor),
			rangePillColors: rangePills.map((element) => window.getComputedStyle(element).color),
			stageBackground: stageStyle?.backgroundColor ?? '',
			timeCellBorderColors: timeCells.map((element) => window.getComputedStyle(element).borderRightColor),
			timeScrollerBackground: timeScrollerStyle?.backgroundColor ?? '',
			timeScrollerBackgroundImage: timeScrollerStyle?.backgroundImage ?? '',
			timeCellRects: timeCells.map(elementRect),
			timedEventRects: timedEvents.map((element) => ({
				id: element.dataset.eventId ?? '',
				...elementRect(element)
			})),
			timeRightEdges: rightEdges(timeCells),
			timeTop: Math.round((timeScroller?.getBoundingClientRect().top ?? 0) * 100) / 100
		};
	});
}

async function doubleClickFirstVisibleTimeCell(page: Page): Promise<void> {
	await page.evaluate(() => {
		const visibleCells = Array.from(document.querySelectorAll('.df-time-grid-row .df-week-time-grid-cell')).filter(
			(element): element is HTMLElement => {
				if (!(element instanceof HTMLElement)) return false;
				const rectangle = element.getBoundingClientRect();
				const style = window.getComputedStyle(element);
				return rectangle.width > 0 && rectangle.height > 0 && style.display !== 'none' && style.visibility !== 'hidden';
			}
		);
		const target = visibleCells[0];
		if (!(target instanceof HTMLElement)) throw new Error('Missing visible week time cell');
		const rectangle = target.getBoundingClientRect();
		const clientX = rectangle.left + rectangle.width / 2;
		const clientY = rectangle.top + Math.min(120, rectangle.height / 2);
		target.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, cancelable: true, button: 0, clientX, clientY }));
		target.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, cancelable: true, button: 0, clientX, clientY }));
		target.dispatchEvent(new MouseEvent('dblclick', { bubbles: true, cancelable: true, button: 0, clientX, clientY }));
	});
}

async function closeDayFlowMobileEditor(page: Page): Promise<void> {
	await dayFlowMobileEditor(page).getByRole('button', { name: '취소' }).click();
	await expect(dayFlowMobileEditor(page)).toHaveCount(0);
}

function dayFlowMobileEditor(page: Page) {
	return page.locator('.calendar-mobile-event-editor .df-mobile-event-drawer-panel, .df-dialog-container');
}

function maximumEdgeDelta(firstEdges: number[], secondEdges: number[]): number {
	if (firstEdges.length !== secondEdges.length) return Number.POSITIVE_INFINITY;
	return Math.max(...firstEdges.map((edge, index) => Math.abs(edge - secondEdges[index])));
}
