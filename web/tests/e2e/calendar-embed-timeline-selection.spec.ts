import { expect, test } from '@playwright/test';
import { routeDefaultCalendarAPI } from './calendar-embed-test-utils';
import {
	expectCalendarEventSelectedBlue,
	expectCalendarEventSelectedOnPointerDown,
	expectCalendarEventTitleAndTime,
	expectRightPanelEventContentCentered,
	expectRightPanelEventCardsShareBlockStyle
} from './calendar-embed-interaction-assertions';
import { openCalendarEmbed } from './calendar-embed-interaction-helpers';
import {
	clearCalendarSelectionFromTestPage,
	duplicateDayAnchorPanelEventSelector,
	duplicateDayAnchorTimelineEventSelector,
	routeDuplicateDayAnchorEvent,
	routeRightPanelMixedEvents
} from './calendar-embed-timeline-fixtures';

test.describe('embedded calendar timeline selection and card style', () => {
	test.beforeEach(async ({ page }) => {
		await routeDefaultCalendarAPI(page);
	});

	test('uses consistent right panel event cards for all-day and timed day events', async ({ page }) => {
		await routeRightPanelMixedEvents(page);

		await openCalendarEmbed(page, '일');
		const selectors = [
			'right-panel-all-day-a',
			'right-panel-all-day-b',
			'right-panel-all-day-c',
			'right-panel-timed-a',
			'right-panel-timed-b'
		].map((eventID) => `.calendar-stage [data-event-id="${eventID}"].df-right-panel-event-card`);
		for (const selector of selectors) {
			await expect(page.locator(selector)).toBeVisible();
		}

		await expectRightPanelEventCardsShareBlockStyle(page, selectors);
		await expectRightPanelEventContentCentered(page, selectors[0]);
		await expectRightPanelEventContentCentered(page, selectors[3]);
	});

	test('selects the matching right panel all-day card from the all-day row', async ({ page }) => {
		await routeRightPanelMixedEvents(page);

		await openCalendarEmbed(page, '일');
		const allDayRowEventSelector = '.calendar-stage .df-day-content-all-day-lane [data-event-id="right-panel-all-day-a"]';
		const panelEventSelector = '.calendar-stage [data-event-id="right-panel-all-day-a"].df-right-panel-event-card';

		await expect(page.locator(allDayRowEventSelector)).toBeVisible();
		await expect(page.locator(panelEventSelector)).toBeVisible();
		await expectCalendarEventSelectedOnPointerDown(page, allDayRowEventSelector, panelEventSelector);
		await expectCalendarEventSelectedBlue(page, panelEventSelector);
	});

	test('keeps duplicated day event selection and timed labels aligned', async ({ page }) => {
		await routeDuplicateDayAnchorEvent(page);

		await openCalendarEmbed(page, '일');
		const timelineEvent = page.locator(duplicateDayAnchorTimelineEventSelector);
		const panelEvent = page.locator(duplicateDayAnchorPanelEventSelector);
		await expect(timelineEvent).toBeVisible();
		await expect(panelEvent).toBeVisible();

		await expectCalendarEventSelectedOnPointerDown(page, duplicateDayAnchorTimelineEventSelector, duplicateDayAnchorPanelEventSelector);
		await expectCalendarEventSelectedBlue(page, duplicateDayAnchorTimelineEventSelector);
		await expectCalendarEventSelectedBlue(page, duplicateDayAnchorPanelEventSelector);
		await clearCalendarSelectionFromTestPage(page);
		await expectCalendarEventSelectedOnPointerDown(page, duplicateDayAnchorPanelEventSelector, duplicateDayAnchorTimelineEventSelector);
		await expectCalendarEventSelectedBlue(page, duplicateDayAnchorTimelineEventSelector);
		await expectCalendarEventSelectedBlue(page, duplicateDayAnchorPanelEventSelector);
		await expectCalendarEventTitleAndTime(page, duplicateDayAnchorTimelineEventSelector, 'Duplicate Day Anchor Event', '02:00');
		await expectCalendarEventTitleAndTime(page, duplicateDayAnchorPanelEventSelector, 'Duplicate Day Anchor Event', '02:00');
		await expect(timelineEvent).not.toContainText('03:00');
		await expect(panelEvent).not.toContainText('03:00');
	});
});
