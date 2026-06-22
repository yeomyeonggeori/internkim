import { expect, test } from '@playwright/test';
import { routeCalendarEvents, routeDefaultCalendarAPI } from './calendar-embed-test-utils';
import { expectTimelinePreviewWithinGrid } from './calendar-embed-interaction-assertions';
import { expectTimelineDraftCount } from './calendar-embed-draft-assertions';
import {
	createDayTimelineRangeByDragAtHour,
	createTimelineRangeByDrag,
	createTimelineSlotByDoubleClick,
	finishTimelineRangeDrag,
	openCalendarEmbed,
	startTimelineRangeDrag
} from './calendar-embed-interaction-helpers';

test.describe('embedded calendar timeline drag interactions', () => {
	test.beforeEach(async ({ page }) => {
		await routeDefaultCalendarAPI(page);
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

	test('creates a day draft from the midnight timeline cell by dragging', async ({ page }) => {
		await openCalendarEmbed(page, '일');
		await createDayTimelineRangeByDragAtHour(page, 0, 1);

		await expectTimelineDraftCount(page, 1, 2);
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await expect(
			page.locator('.draft-popover-date-time-summary[data-date-time-summary="start"] .draft-popover-date-time-value')
		).toContainText('00:00');
		await expect(
			page.locator('.draft-popover-date-time-summary[data-date-time-summary="end"] .draft-popover-date-time-value')
		).toContainText('01:00');
		await expect(page.locator('.draft-empty-title-event:not(.df-right-panel-event-card)').first().locator('.calendar-event-time')).toHaveText(
			'00:00'
		);
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
		await expectTimelinePreviewWithinGrid(page, '.timeline-range-preview', '.df-week-time-grid-grid');
		await expect(preview).toHaveClass(/calendar-timeline-overlap-adjusted/);
		await expect(page.locator('[data-event-id="existing-overlap-event"]')).toHaveClass(/calendar-timeline-overlap-adjusted/);

		await finishTimelineRangeDrag(page, '주');
	});

	test('keeps day timeline range preview inside the event column while dragging', async ({ page }) => {
		await openCalendarEmbed(page, '일');
		await startTimelineRangeDrag(page, '일');

		const preview = page.locator('.timeline-range-preview');
		await expect(preview).toBeVisible();
		await expectTimelinePreviewWithinGrid(page, '.timeline-range-preview', '.df-day-content-grid-column');

		await finishTimelineRangeDrag(page, '일');
	});
});
