import { expect, test } from '@playwright/test';
import { routeCalendarEvents, routeDefaultCalendarAPI } from './calendar-embed-test-utils';
import {
	expectAllDayLabelAlignedWithTimeLabels,
	expectDayAllDayCompactEventsCentered,
	expectDayAllDayRowCompact,
	expectDayAllDayRowEmptyCompact,
	expectDayRightPanelDividerContinuous,
	expectDayAllDayUsesContinuousTimelineBoundary,
	expectDayTimelineRowsRightBorderHidden,
	expectDayToolbarDividerUsesSingleBorder,
	expectElementHeightAtLeast,
	expectFirstVisibleTimeLabel,
	expectTimelineEndsAt24,
	expectTimelineScrollState,
	expectWeekendGridStyle,
	expectWeekAllDayEventsCompactAndLabelCentered,
	expectWeekAllDayExtendedDivider,
	expectWeekAllDayReferenceGrid
} from './calendar-embed-interaction-assertions';
import {
	expectPopoverAnchoredToDraftEvent,
	expectPopoverArrowPointsToDraftEvent,
	expectTimelineDraftCount
} from './calendar-embed-draft-assertions';
import {
	createTimelineSlotByDoubleClick,
	dismissDraftPopoverFromTimeline,
	navigateEmbeddedCalendar,
	openCalendarEmbed
} from './calendar-embed-interaction-helpers';

test.describe('embedded calendar timeline layout', () => {
	test.beforeEach(async ({ page }) => {
		await routeDefaultCalendarAPI(page);
	});

	test('starts day and week timelines below the all-day row at 01:00', async ({ page }) => {
		await openCalendarEmbed(page, '일');
		await expectFirstVisibleTimeLabel(page, '01:00');
		await expectDayAllDayRowEmptyCompact(page);
		await expectDayAllDayUsesContinuousTimelineBoundary(page);
		await expectDayRightPanelDividerContinuous(page);
		await expectDayToolbarDividerUsesSingleBorder(page);
		await expectTimelineEndsAt24(page, '.df-day-content-grid', '.df-day-content-grid-boundary-bottom .df-midnight-label');

		await openCalendarEmbed(page, '주');
		await expectFirstVisibleTimeLabel(page, '01:00');
		await expectTimelineEndsAt24(page, '.df-week-time-grid-scroller', '.df-week-time-grid-boundary-tail > .df-time-label');
	});

	test('clips the day timeline right border at 24:00 while the document is scrolled', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 2382 });
		await openCalendarEmbed(page, '일');
		await expectDayTimelineRowsRightBorderHidden(page);
	});

	test('navigates day view from the right mini calendar month arrows', async ({ page }) => {
		await openCalendarEmbed(page, '일');
		const previousButton = page.locator('.df-mini-calendar-header-nav .df-mini-calendar-nav-btn').first();
		const nextButton = page.locator('.df-mini-calendar-header-nav .df-mini-calendar-nav-btn').last();

		await expect(page.locator('.df-mini-calendar-day[data-mini-date-key="2026-06-08"]')).toHaveAttribute('data-selected', 'true');
		await nextButton.click();
		await expect(page.locator('.df-mini-calendar-month-label')).toHaveText('2026년 7월');
		await expect(page.locator('.df-mini-calendar-day[data-mini-date-key="2026-07-08"]')).toHaveAttribute('data-selected', 'true');

		await previousButton.click();
		await expect(page.locator('.df-mini-calendar-month-label')).toHaveText('2026년 6월');
		await expect(page.locator('.df-mini-calendar-day[data-mini-date-key="2026-06-08"]')).toHaveAttribute('data-selected', 'true');
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
		await expectDayAllDayCompactEventsCentered(page, 2);
		await expectDayAllDayUsesContinuousTimelineBoundary(page);
		await expectTimelineScrollState(page, '.df-day-content-grid', '.df-day-content-all-day-row');
		await expectDayAllDayUsesContinuousTimelineBoundary(page, false);

		await openCalendarEmbed(page, '주');
		await expectElementHeightAtLeast(page, '.df-week-all-day-shell', 80);
		await expectTimelineScrollState(page, '.df-week-time-grid-scroller', '.df-week-all-day-shell');
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
});
