import { expect, test } from '@playwright/test';
import {
	browserDateKey,
	computedPseudoStyle,
	computedStyle,
	routeCalendarDeleteIntents,
	routeCalendarDeleteIntentsWithFailures,
	routeCalendarEvents,
	routeDefaultCalendarAPI
} from './calendar-embed-test-utils';
import { expectMiniCalendarSelectedDayTextVisible } from './calendar-embed-draft-assertions';
import { navigateEmbeddedCalendar, openCalendarEmbed, createTimelineSlotByDoubleClick } from './calendar-embed-interaction-helpers';

test.describe('embedded calendar mini calendar', () => {
	test.beforeEach(async ({ page }) => {
		await routeDefaultCalendarAPI(page);
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
		expect(Number.parseFloat(popoverStyle.width)).toBe(400);
		expect(popoverStyle['border-radius']).toBe('18px');
		expect(popoverStyle['backdrop-filter']).toContain('blur');
		await expect(page.locator('.calendar-draft-popover .draft-popover-title-row')).toBeVisible();
		const draftEvent = page.locator('.draft-empty-title-event').first();
		await expect(draftEvent).toBeVisible();
		const draftEventStyle = await computedStyle(page, '.draft-empty-title-event', ['background-image', 'color']);
		expect(draftEventStyle['background-image']).toContain('linear-gradient');
		expect(draftEventStyle.color).toBe('rgb(255, 255, 255)');
	});

	test('navigates from mini calendar and reserves selected event deletes for undo', async ({ page }) => {
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
		const deleteIntentRequests = await routeCalendarDeleteIntents(page);

		await openCalendarEmbed(page, '일');
		await expectMiniCalendarSelectedDayTextVisible(page, '2026-06-08');
		await page.locator('.df-mini-calendar-day[data-mini-date-key="2026-06-10"]').click();
		await expect(page.locator('.df-mini-calendar-day[data-mini-date-key="2026-06-10"]')).toHaveAttribute('data-selected', 'true');
		await expect(page.getByRole('heading', { name: '6월 10일 수요일' })).toBeVisible();

		await page.locator('.calendar-stage [data-event-id="deletable-event"]:not(.df-right-panel-event-card)').first().dblclick();
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await page.locator('.calendar-draft-popover .draft-popover-delete').click();
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		await expect(page.locator('[data-event-id="deletable-event"]')).toHaveCount(0);
		const deleteUndo = page.locator('.calendar-delete-undo-toast');
		await expect(deleteUndo).toBeVisible();
		await expect(deleteUndo).toContainText('일정을 삭제했습니다.');
		await expect.poll(() => deleteIntentRequests.registeredEventIDs.slice()).toEqual(['deletable-event']);

		await deleteUndo.getByRole('button', { name: '실행 취소' }).click();
		await expect(deleteUndo).toHaveCount(0);
		await expect(page.locator('.calendar-stage [data-event-id="deletable-event"]:not(.df-right-panel-event-card)')).toHaveCount(1);
		await expect.poll(() => deleteIntentRequests.canceledEventIDs.slice()).toEqual(['deletable-event']);

		await page.locator('.calendar-stage [data-event-id="deletable-event"]:not(.df-right-panel-event-card)').first().dblclick();
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await page.locator('.calendar-draft-popover .draft-popover-delete').click();
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		await expect(page.locator('.calendar-stage [data-event-id="deletable-event"]:not(.df-right-panel-event-card)')).toHaveCount(0);
		await expect(deleteUndo).toBeVisible();
		await deleteUndo.getByRole('button', { name: '실행 취소' }).click();
		await expect(deleteUndo).toHaveCount(0);
		await expect(page.locator('.calendar-stage [data-event-id="deletable-event"]:not(.df-right-panel-event-card)')).toHaveCount(1);
		await expect.poll(() => deleteIntentRequests.registeredEventIDs.slice()).toEqual([
			'deletable-event',
			'deletable-event'
		]);
		await expect.poll(() => deleteIntentRequests.canceledEventIDs.slice()).toEqual([
			'deletable-event',
			'deletable-event'
		]);

		await page.locator('.calendar-stage [data-event-id="deletable-event"]:not(.df-right-panel-event-card)').first().dblclick();
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await page.locator('.calendar-draft-popover .draft-popover-delete').click();
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		await expect(deleteUndo).toBeVisible();
		await deleteUndo.evaluate((element) => element.setAttribute('data-stability', 'kept'));

		await page.locator('.df-mini-calendar-day[data-mini-date-key="2026-06-11"]').click();
		const keyboardDeleteEvent = page.locator('.calendar-stage [data-event-id="keyboard-delete-event"]:not(.df-right-panel-event-card)').first();
		await keyboardDeleteEvent.click();
		await expect(keyboardDeleteEvent).toHaveClass(/internkim-calendar-event-focused/);
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		await page.keyboard.press('Backspace');
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		await expect(page.locator('[data-event-id="keyboard-delete-event"]')).toHaveCount(0);
		await expect(deleteUndo).toBeVisible();
		await expect.poll(() => deleteUndo.evaluate((element) => element.getAttribute('data-stability'))).toBe('kept');
		await expect.poll(() => deleteIntentRequests.registeredEventIDs.slice()).toEqual([
			'deletable-event',
			'deletable-event',
			'deletable-event',
			'keyboard-delete-event'
		]);
		expect(deleteIntentRequests.canceledEventIDs).toEqual(['deletable-event', 'deletable-event']);
		await expect(deleteUndo).toHaveCount(0);
	});

	test('keeps the current undo delete hidden when the previous intent registration fails', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'failed-delete-event',
				title: '실패할 삭제 일정',
				startISO: '2026-06-10T02:00:00+09:00',
				endISO: '2026-06-10T03:00:00+09:00',
				isAllDay: false
			},
			{
				id: 'pending-delete-event',
				title: '대기 중 삭제 일정',
				startISO: '2026-06-11T02:00:00+09:00',
				endISO: '2026-06-11T03:00:00+09:00',
				isAllDay: false
			}
		]);
		const deleteIntentRequests = await routeCalendarDeleteIntentsWithFailures(page, ['failed-delete-event']);

		await openCalendarEmbed(page, '일');
		await page.locator('.df-mini-calendar-day[data-mini-date-key="2026-06-10"]').click();
		await page.locator('.calendar-stage [data-event-id="failed-delete-event"]:not(.df-right-panel-event-card)').first().dblclick();
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await page.locator('.calendar-draft-popover .draft-popover-delete').click();
		const deleteUndo = page.locator('.calendar-delete-undo-toast');
		await expect(deleteUndo).toBeVisible();

		await page.locator('.df-mini-calendar-day[data-mini-date-key="2026-06-11"]').click();
		const pendingDeleteEvent = page.locator('.calendar-stage [data-event-id="pending-delete-event"]:not(.df-right-panel-event-card)').first();
		await pendingDeleteEvent.click();
		await page.keyboard.press('Backspace');
		await expect(page.locator('.calendar-stage [data-event-id="pending-delete-event"]:not(.df-right-panel-event-card)')).toHaveCount(0);
		await expect.poll(() => deleteIntentRequests.registeredEventIDs.slice()).toEqual([
			'failed-delete-event',
			'pending-delete-event'
		]);
		deleteIntentRequests.releaseRegistrationFailures();
		await expect(page.locator('.calendar-stage [data-event-id="pending-delete-event"]:not(.df-right-panel-event-card)')).toHaveCount(0);
		await expect(deleteUndo).toBeVisible();
		await expect(deleteUndo).toHaveCount(0);

		await page.locator('.df-mini-calendar-day[data-mini-date-key="2026-06-10"]').click();
		await expect(page.locator('.calendar-stage [data-event-id="failed-delete-event"]:not(.df-right-panel-event-card)')).toHaveCount(1);
	});

	test('does not issue another delete request when the page is hidden', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'pagehide-delete-event',
				title: '이탈 전 삭제 일정',
				startISO: '2026-06-10T02:00:00+09:00',
				endISO: '2026-06-10T03:00:00+09:00',
				isAllDay: false
			}
		]);
		const deleteIntentRequests = await routeCalendarDeleteIntents(page);

		await openCalendarEmbed(page, '일');
		await page.locator('.df-mini-calendar-day[data-mini-date-key="2026-06-10"]').click();
		await page.locator('.calendar-stage [data-event-id="pagehide-delete-event"]:not(.df-right-panel-event-card)').first().dblclick();
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await page.locator('.calendar-draft-popover .draft-popover-delete').click();
		const deleteUndo = page.locator('.calendar-delete-undo-toast');
		await expect(deleteUndo).toBeVisible();
		await expect.poll(() => deleteIntentRequests.registeredEventIDs.slice()).toEqual(['pagehide-delete-event']);

		await page.evaluate(() => {
			window.dispatchEvent(new Event('pagehide'));
		});

		await page.waitForTimeout(100);
		expect(deleteIntentRequests.registeredEventIDs).toEqual(['pagehide-delete-event']);
		expect(deleteIntentRequests.canceledEventIDs).toEqual([]);
		await expect(deleteUndo).toBeVisible();
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
