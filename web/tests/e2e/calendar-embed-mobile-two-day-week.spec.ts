import { expect, test } from '@playwright/test';
import {
	activeMobileEditorField,
	closeDayFlowMobileEditor,
	doubleClickFirstVisibleAllDayCell,
	dayFlowMobileEditor,
	doubleClickFirstVisibleTimeCell,
	enableDarkMode,
	maximumEdgeDelta,
	mobileEventHorizontalInset,
	renderedEventIDs,
	selectedDateKey,
	verifyMobileEditorControls,
	weekGridMeasurements
} from './calendar-embed-mobile-two-day-week-helpers';
import {
	routeCalendarEventCreates,
	routeCalendarEventDeletes,
	routeCalendarEventUpdates,
	routeCalendarEvents,
	routeCalendarParticipants,
	routeDefaultCalendarAPI
} from './calendar-embed-test-utils';
import { navigateEmbeddedCalendar, openCalendarEmbed } from './calendar-embed-interaction-helpers';

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

		await doubleClickFirstVisibleAllDayCell(page);
		await expect(dayFlowMobileEditor(page)).toBeVisible();
		await expect(dayFlowMobileEditor(page).getByText('새 일정')).toBeVisible();
		await expect(dayFlowMobileEditor(page).locator('input[data-mobile-editor-field="allDay"]')).toBeChecked();
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		await closeDayFlowMobileEditor(page);
		await expect.poll(async () => (await renderedEventIDs(page)).some((eventID) => eventID.startsWith('all-day-'))).toBe(true);

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
		await expect(dayFlowMobileEditor(page).getByText('Participants')).toBeVisible();
		await expect(dayFlowMobileEditor(page).getByPlaceholder('Search by name to add')).toBeVisible();
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
		await dayFlowMobileEditor(page).locator('input[data-mobile-editor-field="startTime"]').fill('04:00');
		await expect(dayFlowMobileEditor(page).locator('input[data-mobile-editor-field="endTime"]')).toHaveValue('05:00');
		await dayFlowMobileEditor(page).getByRole('button', { name: '완료' }).click();

		await expect(dayFlowMobileEditor(page)).toHaveCount(0);
		await expect.poll(() => updatedEvents.length).toBe(1);
		expect(updatedEvents[0]?.eventID).toBe('mobile-two-day-edit');
		expect(updatedEvents[0]?.title).toBe('모바일 수정 일정');
		expect(new Date(updatedEvents[0]?.endISO ?? '').getTime() - new Date(updatedEvents[0]?.startISO ?? '').getTime()).toBe(
			60 * 60 * 1000
		);
	});

	test('saves mobile-edited participants through the calendar persistence path', async ({ page }) => {
		const updatedEvents = await routeCalendarEventUpdates(page);
		await routeCalendarParticipants(page, [
			{
				personID: 'person-dongha',
				name: '이샘플',
				email: 'dongha@example.com',
				image: '/calendar/api/participants/person-dongha/image'
			},
			{
				personID: 'person-yeomyeong',
				name: '김여명',
				email: 'yeomyeong@example.com'
			}
		]);
		await page.setViewportSize({ width: 390, height: 844 });
		await openCalendarEmbed(page, '주');
		await navigateEmbeddedCalendar(page, '2026-06-01');

		await page.locator('[data-event-id="mobile-two-day-edit"]').first().dblclick();
		await expect(dayFlowMobileEditor(page)).toBeVisible();
		await dayFlowMobileEditor(page).getByPlaceholder('이름으로 검색해 추가').fill('이샘플');
		await dayFlowMobileEditor(page).getByRole('option', { name: '이샘플' }).click();
		await expect(dayFlowMobileEditor(page).getByRole('button', { name: '이샘플 제거' })).toBeVisible();
		await dayFlowMobileEditor(page).getByRole('button', { name: '완료' }).click();

		await expect(dayFlowMobileEditor(page)).toHaveCount(0);
		await expect.poll(() => updatedEvents.length).toBe(1);
		expect(updatedEvents[0]?.eventID).toBe('mobile-two-day-edit');
		expect(updatedEvents[0]?.participants).toEqual([
			{
				personID: 'person-dongha',
				name: '이샘플',
				email: 'dongha@example.com'
			}
		]);
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
