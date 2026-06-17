import { expect, test } from '@playwright/test';
import { routeCalendarEvents, routeDefaultCalendarAPI } from './calendar-embed-test-utils';
import {
	expectAllDayLabelAlignedWithTimeLabels,
	expectDayAllDayRowCompact,
	expectDayAllDayRowEmptyCompact,
	expectDayAllDayUsesContinuousTimelineBoundary,
	expectDayToolbarDividerUsesSingleBorder,
	expectElementHeightAtLeast,
	expectFirstVisibleTimeLabel,
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
	createTimelineRangeByDrag,
	createTimelineSlotByDoubleClick,
	dismissDraftPopoverFromTimeline,
	finishTimelineRangeDrag,
	navigateEmbeddedCalendar,
	openCalendarEmbed,
	startTimelineRangeDrag
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

	test('opens a timeline event popover only on double click after selecting it', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'double-click-timeline-event',
				title: 'Double Click Timeline Event',
				startISO: '2026-06-08T09:00:00+09:00',
				endISO: '2026-06-08T10:00:00+09:00',
				isAllDay: false
			}
		]);

		await openCalendarEmbed(page, '일');
		const eventBlock = page.locator(
			'.calendar-stage [data-event-id="double-click-timeline-event"].df-day-event:not(.df-right-panel-event-card)'
		);
		await expect(eventBlock).toBeVisible();
		await eventBlock.click();

		await expect(eventBlock).toHaveClass(/internkim-calendar-event-focused/);
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);

		await eventBlock.dblclick();

		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await expect(page.getByLabel('제목')).toHaveValue('Double Click Timeline Event');
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
