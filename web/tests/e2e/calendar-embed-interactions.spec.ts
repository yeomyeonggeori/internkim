import { expect, test } from '@playwright/test';
import {
	browserDateKey,
	computedPseudoStyle,
	computedStyle,
	elementBox,
	routeCalendarEventDeletes,
	routeCalendarEvents
} from './calendar-embed-test-utils';
import {
	expectRenderableCalendarEvent,
	expectFirstVisibleTimeLabel,
	expectElementHeightAtLeast,
	expectTimelineScrollState,
	expectAllDayLabelAlignedWithTimeLabels,
	expectWeekendGridStyle,
	expectWeekAllDayReferenceGrid,
	expectWeekAllDayEventsCompactAndLabelCentered,
	expectMonthOverlayAlignedWithMonthStartRow,
	expectWeekAllDayExtendedDivider,
	expectDayAllDayUsesContinuousTimelineBoundary,
	expectDayToolbarDividerUsesSingleBorder,
	expectMonthWeekendCellsKeepGridLines,
	expectDayAllDayRowCompact,
	expectDayAllDayRowEmptyCompact
} from './calendar-embed-interaction-assertions';
import {
	expectPopoverAnchoredToDraftEvent,
	expectPopoverArrowPointsToDraftEvent,
	expectMiniCalendarSelectedDayTextVisible,
	expectTimelineDraftCount
} from './calendar-embed-draft-assertions';
import {
	dismissDraftPopoverFromTimeline,
	createTimelineSlotByDoubleClick,
	createTimelineRangeByDrag,
	startTimelineRangeDrag,
	finishTimelineRangeDrag,
	openCalendarEmbed,
	navigateEmbeddedCalendar
} from './calendar-embed-interaction-helpers';

test.describe('embedded calendar interactions', () => {
	test.beforeEach(async ({ page }) => {
		await page.route('**/calendar/api/events?**', async (route) => {
			await route.fulfill({
				json: {
					events: [
						{
							id: 'existing-overlap-event',
							title: '겹치는 일정',
							startISO: '2026-06-08T01:00:00+09:00',
							endISO: '2026-06-08T02:00:00+09:00',
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
	});

	test('renders persisted timed, all-day, and multi-day events across views', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'timed-meeting',
				title: 'Timed Meeting',
				startISO: '2026-06-08T09:00:00+09:00',
				endISO: '2026-06-08T10:30:00+09:00',
				isAllDay: false
			},
			{
				id: 'all-day-event',
				title: 'All Day Event',
				startISO: '2026-06-08T00:00:00+09:00',
				endISO: '2026-06-09T00:00:00+09:00',
				isAllDay: true
			},
			{
				id: 'multi-day-event',
				title: 'Multi Day Event',
				startISO: '2026-06-09T00:00:00+09:00',
				endISO: '2026-06-12T00:00:00+09:00',
				isAllDay: true
			}
		]);

		await openCalendarEmbed(page, '일');
		await expectRenderableCalendarEvent(page, 'all-day-event', '.df-day-event.df-event-all-day');
		await expectRenderableCalendarEvent(page, 'timed-meeting', '.df-day-event.df-event-timed');

		await openCalendarEmbed(page, '주');
		await expectRenderableCalendarEvent(page, 'all-day-event', '.df-week-event.df-event-all-day');
		await expectRenderableCalendarEvent(page, 'multi-day-event', '.df-week-event.df-event-all-day');
		await expectRenderableCalendarEvent(page, 'timed-meeting', '.df-week-event.df-event-timed');

		await openCalendarEmbed(page, '월');
		await expectRenderableCalendarEvent(page, 'timed-meeting', '.df-month-event.df-event-timed');
		await expectRenderableCalendarEvent(page, 'all-day-event', '.df-month-event.df-event-all-day');
		await expectRenderableCalendarEvent(page, 'multi-day-event', '.df-month-event.df-event-all-day');
	});

	test('starts day and week timelines below the all-day row at 01:00', async ({ page }) => {
		await openCalendarEmbed(page, '일');
		await expectFirstVisibleTimeLabel(page, '01:00');
		await expectDayAllDayRowEmptyCompact(page);
		await expectDayAllDayUsesContinuousTimelineBoundary(page);
		await expectDayToolbarDividerUsesSingleBorder(page);

		await openCalendarEmbed(page, '주');
		await expectFirstVisibleTimeLabel(page, '01:00');
	});

	test('keeps the all-day header aligned and marked while scrolling day and week timelines', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'first-all-day-event',
				title: 'First All Day Event',
				startISO: '2026-06-08T00:00:00+09:00',
				endISO: '2026-06-09T00:00:00+09:00',
				isAllDay: true
			},
			{
				id: 'second-all-day-event',
				title: 'Second All Day Event',
				startISO: '2026-06-08T00:00:00+09:00',
				endISO: '2026-06-09T00:00:00+09:00',
				isAllDay: true
			}
		]);

		await openCalendarEmbed(page, '일');
		await expectElementHeightAtLeast(page, '.df-day-content-all-day-row', 44);
		await expectDayAllDayUsesContinuousTimelineBoundary(page);
		await expectTimelineScrollState(page, '.df-day-content-grid', '.df-day-content-all-day-row');
		await expectDayAllDayUsesContinuousTimelineBoundary(page, false);

		await openCalendarEmbed(page, '주');
		await expectElementHeightAtLeast(page, '.df-week-all-day-shell', 80);
		await expectTimelineScrollState(page, '.df-week-time-grid-scroller', '.df-week-all-day-shell');
	});

	test('scrolls the month virtual scroller on wheel gestures', async ({ page }) => {
		await openCalendarEmbed(page, '월');
		const scroller = page.locator('.df-month-view-virtual-scroller');
		await expect(scroller).toBeVisible();
		const initialScrollTop = await scroller.evaluate((element) => element.scrollTop);

		await scroller.hover();
		await page.mouse.wheel(0, 640);

		await expect.poll(async () => scroller.evaluate((element) => element.scrollTop)).toBeGreaterThan(initialScrollTop);
	});

	test('shows a month overlay label while scrolling the month view', async ({ page }) => {
		await openCalendarEmbed(page, '월');
		const scroller = page.locator('.df-month-view-virtual-scroller');
		await expect(scroller).toBeVisible();

		await scroller.hover();
		await page.mouse.wheel(0, 640);

		await expect(page.locator('.month-scroll-overlay').first()).toBeVisible();
		await expect(page.locator('.month-scroll-overlay').first()).toHaveText(/\d{4}년 \d{1,2}월/);
		await expectMonthOverlayAlignedWithMonthStartRow(page);
	});

	test('creates one draft when double-clicking a timeline slot', async ({ page }) => {
		await openCalendarEmbed(page, '일');
		await createTimelineSlotByDoubleClick(page, '일');
		await expectTimelineDraftCount(page, 1, 2);

		await openCalendarEmbed(page, '주');
		await createTimelineSlotByDoubleClick(page, '주');
		await expectTimelineDraftCount(page, 1, 2);
	});

	test('creates one draft when dragging a timeline range', async ({ page }) => {
		await openCalendarEmbed(page, '일');
		await createTimelineRangeByDrag(page, '일');
		await expectTimelineDraftCount(page, 1, 2);

		await openCalendarEmbed(page, '주');
		await createTimelineRangeByDrag(page, '주');
		await expectTimelineDraftCount(page, 1, 2);
	});

	test('shows timeline range preview with time label and overlap lane while dragging', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'existing-overlap-event',
				title: '겹치는 일정',
				startISO: '2026-06-08T02:00:00+09:00',
				endISO: '2026-06-08T03:30:00+09:00',
				isAllDay: false
			}
		]);
		await openCalendarEmbed(page, '주');
		await expect(page.locator('[data-event-id="existing-overlap-event"]')).toBeVisible();
		await startTimelineRangeDrag(page, '주');

		const preview = page.locator('.timeline-range-preview');
		await expect(preview).toBeVisible();
		await expect(preview.locator('.timeline-range-preview-time')).toHaveText(/\d{2}:\d{2} - \d{2}:\d{2}/);
		await expect(preview).toHaveClass(/calendar-timeline-overlap-adjusted/);
		await expect(page.locator('[data-event-id="existing-overlap-event"]')).toHaveClass(/calendar-timeline-overlap-adjusted/);

		await finishTimelineRangeDrag(page, '주');
	});

	test('enhances the DayFlow mini calendar visual states', async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-14T03:00:00.000Z'));
		await openCalendarEmbed(page, '일');
		await page.waitForSelector('.df-mini-calendar-day', { state: 'attached' });
		const todayDateKey = await browserDateKey(page);

		const selectedDay = page.locator('.df-mini-calendar-day[data-mini-date-key="2026-06-08"]');
		const todayDay = page.locator(`.df-mini-calendar-day[data-mini-date-key="${todayDateKey}"]`);
		const weekendDay = page.locator('.df-mini-calendar-day[data-mini-date-key="2026-06-14"]');
		const eventDotSlot = page.locator('.df-mini-calendar-day[data-mini-date-key="2026-06-08"] .mini-month-event-dot-slot');
		const emptyDotSlot = page.locator('.df-mini-calendar-day[data-mini-date-key="2026-06-09"] .mini-month-event-dot-slot');

		await expect(page.locator('.df-mini-calendar-header').first()).toHaveAttribute('data-mini-weekday-index', '0');
		await expect(page.locator('.df-mini-calendar-header')).toHaveText(['일', '월', '화', '수', '목', '금', '토']);
		await expect(page.locator('.df-mini-calendar-day').first()).toHaveAttribute('data-mini-date-key', '2026-05-31');
		await expect(page.locator('.df-mini-calendar-day').nth(1)).toHaveAttribute('data-mini-date-key', '2026-06-01');
		await expect(selectedDay).toHaveAttribute('data-selected', 'true');
		await expect(todayDay).toHaveAttribute('data-today', 'true');
		await expect(weekendDay).toHaveAttribute('data-weekend', 'true');
		await expect(eventDotSlot).toHaveAttribute('data-has-event', 'true');
		await expect(emptyDotSlot).toHaveAttribute('data-has-event', 'false');
		await expect(page.locator('.df-mini-calendar-month-label')).toHaveAttribute('role', 'button');
		await expect(page.locator('.df-mini-calendar-header-nav .df-mini-calendar-nav-btn')).toHaveCount(2);
		await expect(page.locator('.df-mini-calendar-header-nav .df-mini-calendar-nav-btn').first()).toHaveAttribute('aria-label', '이전');
		await expect(page.locator('.df-mini-calendar-header-nav .df-mini-calendar-nav-btn').last()).toHaveAttribute('aria-label', '다음');
	});

	test('keeps a selected weekend today legible in the DayFlow mini calendar', async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-13T03:00:00.000Z'));
		await openCalendarEmbed(page, '일');
		await navigateEmbeddedCalendar(page, '2026-06-13');
		const todayButton = page.locator('.df-mini-calendar-day[data-mini-date-key="2026-06-13"]');

		await expect(todayButton).toHaveAttribute('data-today', 'true');
		await expect(todayButton).toHaveAttribute('data-selected', 'true');
		await expect(todayButton).toHaveAttribute('data-weekend', 'true');
		await expect(todayButton).toHaveCSS('color', 'rgb(255, 255, 255)');
	});

	test('keeps an unselected today legible in the DayFlow mini calendar', async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-14T03:00:00.000Z'));
		await openCalendarEmbed(page, '일');
		await navigateEmbeddedCalendar(page, '2026-06-01');
		const todayButton = page.locator('.df-mini-calendar-day[data-mini-date-key="2026-06-14"]');

		await expect(todayButton).toHaveAttribute('data-today', 'true');
		await expect(todayButton).toHaveAttribute('data-selected', 'false');
		await expect(todayButton).toHaveCSS('color', 'rgb(255, 255, 255)');
	});

	test('matches reference mini calendar selection and draft popover surface', async ({ page }) => {
		await openCalendarEmbed(page, '일');
		await page.waitForSelector('.df-mini-calendar-day', { state: 'attached' });

		await expect(page.locator('.df-right-panel-calendar-header')).toHaveCount(0);
		const selectedDayStyle = await computedStyle(page, '.df-mini-calendar-day[data-mini-date-key="2026-06-08"]', [
			'background-color',
			'border-radius',
			'color'
		]);
		const selectedDayMarkerStyle = await computedPseudoStyle(page, '.df-mini-calendar-day[data-mini-date-key="2026-06-08"]', '::before', [
			'background-color',
			'content'
		]);
		expect(selectedDayStyle['background-color']).toBe('rgba(0, 0, 0, 0)');
		expect(selectedDayStyle['border-radius']).toBe('9999px');
		expect(selectedDayStyle.color).toBe('rgb(51, 65, 85)');
		expect(selectedDayMarkerStyle['background-color']).toBe('rgb(195, 203, 214)');
		expect(selectedDayMarkerStyle.content).toBe('""');

		await createTimelineSlotByDoubleClick(page, '일');
		const popoverStyle = await computedStyle(page, '.calendar-draft-popover', [
			'position',
			'width',
			'border-radius',
			'backdrop-filter'
		]);
		expect(popoverStyle.position).toBe('absolute');
		expect(Number.parseFloat(popoverStyle.width)).toBeGreaterThanOrEqual(500);
		expect(popoverStyle['border-radius']).toBe('18px');
		expect(popoverStyle['backdrop-filter']).toContain('blur');
		await expect(page.locator('.calendar-draft-popover .draft-popover-title-row')).toBeVisible();
		const draftEvent = page.locator('.draft-empty-title-event').first();
		await expect(draftEvent).toBeVisible();
		const draftEventStyle = await computedStyle(page, '.draft-empty-title-event', ['background-image', 'color']);
		expect(draftEventStyle['background-image']).toContain('linear-gradient');
		expect(draftEventStyle.color).toBe('rgb(255, 255, 255)');
	});

	test('matches reference all-day axis, weekend grid, and draft dismissal behavior', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'week-all-day-event',
				title: 'Week All Day Event',
				startISO: '2026-06-08T00:00:00+09:00',
				endISO: '2026-06-09T00:00:00+09:00',
				isAllDay: true
			}
		]);

		await openCalendarEmbed(page, '주');
		await expectAllDayLabelAlignedWithTimeLabels(page, '.df-week-all-day-label');
		await expectWeekendGridStyle(page);
		await expectWeekAllDayReferenceGrid(page);
		await expectWeekAllDayExtendedDivider(page, false);
		await expectTimelineScrollState(page, '.df-week-time-grid-scroller', '.df-week-all-day-shell');
		await expectAllDayLabelAlignedWithTimeLabels(page, '.df-week-all-day-label');
		await expectWeekAllDayExtendedDivider(page, true);

		await createTimelineSlotByDoubleClick(page, '주');
		await expectPopoverAnchoredToDraftEvent(page);
		await expectPopoverArrowPointsToDraftEvent(page);
		await dismissDraftPopoverFromTimeline(page);
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		await expectTimelineDraftCount(page, 0, 1);

		await openCalendarEmbed(page, '일');
		await expectDayAllDayRowCompact(page);
		await expectAllDayLabelAlignedWithTimeLabels(page, '.df-all-day-label');
		await createTimelineSlotByDoubleClick(page, '일');
		await expectPopoverAnchoredToDraftEvent(page);
		await expectPopoverArrowPointsToDraftEvent(page);
		await dismissDraftPopoverFromTimeline(page);
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		await expectTimelineDraftCount(page, 0, 1);
	});

	test('compacts week all-day event rows and centers the all-day label', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'stacked-all-day-event-1',
				title: '1',
				startISO: '2026-06-17T00:00:00+09:00',
				endISO: '2026-06-18T00:00:00+09:00',
				isAllDay: true
			},
			{
				id: 'stacked-all-day-event-2',
				title: '2',
				startISO: '2026-06-17T00:00:00+09:00',
				endISO: '2026-06-18T00:00:00+09:00',
				isAllDay: true
			},
			{
				id: 'stacked-all-day-event-3',
				title: '3',
				startISO: '2026-06-17T00:00:00+09:00',
				endISO: '2026-06-18T00:00:00+09:00',
				isAllDay: true
			},
			{
				id: 'stacked-all-day-event-4',
				title: '4',
				startISO: '2026-06-17T00:00:00+09:00',
				endISO: '2026-06-18T00:00:00+09:00',
				isAllDay: true
			},
			{
				id: 'stacked-all-day-event-5',
				title: '5',
				startISO: '2026-06-17T00:00:00+09:00',
				endISO: '2026-06-18T00:00:00+09:00',
				isAllDay: true
			}
		]);

		await openCalendarEmbed(page, '주');
		await navigateEmbeddedCalendar(page, '2026-06-17');
		await expectWeekAllDayEventsCompactAndLabelCentered(page);
	});

	test('keeps month and week calendar markers legible without hiding grid lines', async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-07T03:00:00.000Z'));
		await openCalendarEmbed(page, '월');
		await expectMonthWeekendCellsKeepGridLines(page);
		await expect(page.locator('.df-month-day-cell[data-date="2026-06-07"] .df-month-date-number')).toHaveCSS('color', 'rgb(255, 255, 255)');

		await openCalendarEmbed(page, '주');
		await navigateEmbeddedCalendar(page, '2026-06-07');
		await expect(page.locator('.df-week-header > .df-week-day-cell:first-child .df-date-number')).toHaveCSS('color', 'rgb(255, 255, 255)');

		await navigateEmbeddedCalendar(page, '2026-06-16');
		const selectedWeekHeader = page.locator('.df-week-day-cell.calendar-selected-week-date, .df-week-day-header.calendar-selected-week-date');
		await expect(selectedWeekHeader).toBeVisible();
		await expect(selectedWeekHeader).toContainText(/화\s*16/);
		const selectedDateNumber = selectedWeekHeader.locator('.df-date-number, .df-week-date-number').first();
		await expect(selectedDateNumber).toHaveCSS('background-color', 'rgba(0, 0, 0, 0)');
	});

	test('navigates from mini calendar and deletes selected events from the popover or keyboard', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'deletable-event',
				title: '삭제할 일정',
				startISO: '2026-06-10T02:00:00+09:00',
				endISO: '2026-06-10T03:00:00+09:00',
				isAllDay: false
			},
			{
				id: 'keyboard-delete-event',
				title: '키보드 삭제 일정',
				startISO: '2026-06-11T02:00:00+09:00',
				endISO: '2026-06-11T03:00:00+09:00',
				isAllDay: false
			}
		]);
		const deletedEventIDs = await routeCalendarEventDeletes(page);

		await openCalendarEmbed(page, '일');
		await expectMiniCalendarSelectedDayTextVisible(page, '2026-06-08');
		await page.locator('.df-mini-calendar-day[data-mini-date-key="2026-06-10"]').click();
		await expect(page.locator('.df-mini-calendar-day[data-mini-date-key="2026-06-10"]')).toHaveAttribute('data-selected', 'true');
		await expect(page.getByRole('heading', { name: '6월 10일 수요일' })).toBeVisible();

		await page.locator('[data-event-id="deletable-event"]').first().click();
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await page.locator('.calendar-draft-popover .draft-popover-delete').click();
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		await expect(page.locator('[data-event-id="deletable-event"]')).toHaveCount(0);

		await page.locator('.df-mini-calendar-day[data-mini-date-key="2026-06-11"]').click();
		await page.locator('[data-event-id="keyboard-delete-event"]').first().click();
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await page.keyboard.press('Backspace');
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		await expect(page.locator('[data-event-id="keyboard-delete-event"]')).toHaveCount(0);
		expect(deletedEventIDs).toEqual(['deletable-event', 'keyboard-delete-event']);
	});

	test('opens the right mini calendar month picker and syncs selected dates in week and month views', async ({ page }) => {
		await openCalendarEmbed(page, '일');
		await page.waitForSelector('.df-mini-calendar-day', { state: 'attached' });

		await page.locator('.df-mini-calendar-month-label').click();
		const picker = page.locator('.calendar-mini-month-picker');
		await expect(picker).toBeVisible();
		await expect(picker.getByRole('button', { name: '2026' })).toBeVisible();
		await picker.getByRole('button', { name: '7월' }).click();
		await expect(page.locator('.df-mini-calendar-month-label')).toHaveText('2026년 7월');
		await expect(page.locator('.df-mini-calendar-day[data-mini-date-key="2026-07-08"]')).toHaveAttribute('data-selected', 'true');

		await openCalendarEmbed(page, '주');
		await navigateEmbeddedCalendar(page, '2026-06-10');
		const selectedWeekHeader = page.locator('.df-week-day-cell.calendar-selected-week-date, .df-week-day-header.calendar-selected-week-date');
		await expect(selectedWeekHeader).toBeVisible();
		await expect(selectedWeekHeader).toContainText(/수\s*10/);
		await expect(selectedWeekHeader).toHaveClass(/calendar-selected-week-date/);

		await openCalendarEmbed(page, '월');
		await navigateEmbeddedCalendar(page, '2026-06-10');
		await expect(page.locator('.df-month-day-cell[data-date="2026-06-10"]')).toHaveClass(/month-selected-date/);
	});
});
