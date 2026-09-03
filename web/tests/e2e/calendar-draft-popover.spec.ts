import { expect, test } from '@playwright/test';
import { cleanupCalendarEvents, seedCalendarEvents, signInToCalendar } from './calendar-central-test-utils';
import {
	clickOutsideDraftPopover,
	dragBetweenCells,
	routeCalendarHolidays,
	routeEventAdd,
	routeEventUpdate,
	waitForClientHydration
} from './calendar-draft-popover-test-utils';

test.describe('calendar draft popover', () => {
	test.use({ locale: 'ko-KR' });

	test.beforeEach(async ({ page }) => {
		await routeCalendarHolidays(page);
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await signInToCalendar(page);
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);
	});

	test('opens a draft popover from an empty day cell without posting a default title event', async ({ page }) => {
		const addedPayloads = await routeEventAdd(page);

		await page.locator('[data-calendar-date="2026-06-09"]').dblclick();

		const popover = page.locator('.calendar-draft-popover');
		await expect(popover).toBeVisible();
		await expect(popover.getByLabel('제목')).toBeVisible();
		await expect(popover.getByLabel('제목')).toBeFocused();
		await expect(popover.getByLabel('종일')).toBeVisible();
		await expect(popover.locator('.event-audit-card')).toHaveCount(0);
		expect(addedPayloads).toHaveLength(0);
	});

	test('posts the draft only after the popover is dismissed with a title', async ({ page }) => {
		const addedPayloads = await routeEventAdd(page);

		await page.locator('[data-calendar-date="2026-06-09"]').dblclick();
		const popover = page.locator('.calendar-draft-popover');
		await expect(popover).toBeVisible();
		await popover.getByLabel('제목').fill('디자인 리뷰');
		await popover.getByLabel('장소').fill('회의실 A');
		await popover.getByLabel('설명').fill('월간 디자인 점검');
		expect(addedPayloads).toHaveLength(0);

		await clickOutsideDraftPopover(page);

		await expect.poll(() => addedPayloads.length).toBe(1);
		expect(addedPayloads[0]).toMatchObject({
			title: '디자인 리뷰',
			location: '회의실 A',
			note: '월간 디자인 점검',
			isWholeDay: false
		});
		await expect(page.locator('.calendar-draft-popover')).toBeHidden();
	});

	test('saves a titled draft when dismissed and discards an untitled draft on escape', async ({ page }) => {
		const addedPayloads = await routeEventAdd(page);

		await page.locator('[data-calendar-date="2026-06-09"]').dblclick();
		await page.getByLabel('제목').fill('바깥 저장');
		await clickOutsideDraftPopover(page);
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		await expect.poll(() => addedPayloads.length).toBe(1);
		expect(addedPayloads[0]).toMatchObject({ title: '바깥 저장' });

		const chipCountBeforeSecondDraft = await page.locator('[data-calendar-event-id]').count();

		await page.clock.setFixedTime(new Date('2026-06-08T12:00:01'));
		await page.locator('[data-calendar-date="2026-06-11"]').dblclick();
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await expect(page.locator('[data-calendar-event-id]')).toHaveCount(chipCountBeforeSecondDraft + 1);

		await page.keyboard.press('Escape');

		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		await expect(page.locator('[data-calendar-event-id]')).toHaveCount(chipCountBeforeSecondDraft);
		expect(addedPayloads).toHaveLength(1);
	});

	test('does not save an unchanged existing event when dismissing the edit popover', async ({ page }) => {
		const updatedPayloads = await routeEventUpdate(page);
		const [eventID] = await seedCalendarEvents([
			{ title: '그대로인 일정', startISO: '2026-06-10T09:00:00+09:00', endISO: '2026-06-10T10:00:00+09:00' }
		]);
		try {
			await page.reload();
			await waitForClientHydration(page);
			const chip = page.locator(`[data-calendar-event-id="${eventID}"]`);
			await expect(chip).toBeVisible();
			await chip.click();
			await expect(page.locator('.calendar-draft-popover')).toBeVisible();

			await clickOutsideDraftPopover(page);

			await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
			expect(updatedPayloads).toHaveLength(0);
		} finally {
			await cleanupCalendarEvents([eventID]);
		}
	});

	test('saves a changed existing event when dismissing the edit popover', async ({ page }) => {
		const updatedPayloads = await routeEventUpdate(page);
		const [eventID] = await seedCalendarEvents([
			{ title: '수정 전 일정', startISO: '2026-06-10T09:00:00+09:00', endISO: '2026-06-10T10:00:00+09:00' }
		]);
		try {
			await page.reload();
			await waitForClientHydration(page);
			const chip = page.locator(`[data-calendar-event-id="${eventID}"]`);
			await expect(chip).toBeVisible();
			await chip.click();
			await expect(page.locator('.calendar-draft-popover')).toBeVisible();
			await page.getByLabel('제목').fill('수정 후 일정');
			await page.getByLabel('장소').fill('회의실 B');

			await clickOutsideDraftPopover(page);

			await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
			await expect.poll(() => updatedPayloads.length).toBe(1);
			expect(updatedPayloads[0]).toMatchObject({
				eventHint: eventID,
				title: '수정 후 일정',
				location: '회의실 B'
			});
		} finally {
			await cleanupCalendarEvents([eventID]);
		}
	});

	test('discards a timed draft when the end is not after the start', async ({ page }) => {
		const addedPayloads = await routeEventAdd(page);

		await page.locator('[data-calendar-date="2026-06-09"]').dblclick();
		const popover = page.locator('.calendar-draft-popover');
		await popover.getByLabel('제목').fill('시간 검증');
		await popover.getByLabel('종료 시간').fill('09:00');

		await clickOutsideDraftPopover(page);

		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		expect(addedPayloads).toHaveLength(0);
		await expect(page.locator('[data-calendar-event-id^="month-"]')).toHaveCount(0);
	});

	test('opens a draft popover for a day cell on double-click', async ({ page }) => {
		const dateCell = page.locator('[data-calendar-date="2026-06-10"]');
		await dateCell.dblclick();

		await expect(dateCell).toHaveAttribute('data-selected', '');
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await expect(page.getByLabel('제목')).toBeVisible();
	});

	test('cancels a new draft without leaving a local event behind', async ({ page }) => {
		await page.locator('[data-calendar-date="2026-06-09"]').dblclick();
		await expect(page.locator('[data-calendar-event-id^="month-"]')).toHaveCount(1);

		await page.keyboard.press('Escape');

		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		await expect(page.locator('[data-calendar-event-id^="month-"]')).toHaveCount(0);
	});

	test('unchecking all day on an existing all-day event gives it valid times and saves the draft on dismissal', async ({
		page
	}) => {
		const updatedPayloads = await routeEventUpdate(page);
		const [eventID] = await seedCalendarEvents([
			{
				title: '전사 워크숍',
				startISO: '2026-06-10T00:00:00+09:00',
				endISO: '2026-06-12T00:00:00+09:00',
				isAllDay: true
			}
		]);
		try {
			await page.reload();
			await waitForClientHydration(page);
			const chip = page.locator(`[data-calendar-event-id="${eventID}"]`);
			await expect(chip).toBeVisible();
			await chip.click();
			const popover = page.locator('.calendar-draft-popover');
			await expect(popover).toBeVisible();
			await expect(popover.getByLabel('종일')).toBeChecked();

			await popover.getByLabel('종일').click();

			await expect(popover.getByLabel('시작 시간')).toHaveValue('09:00');
			await expect(popover.getByLabel('종료 시간')).toHaveValue('10:00');

			await clickOutsideDraftPopover(page);

			await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
			await expect.poll(() => updatedPayloads.length).toBe(1);
			expect(updatedPayloads[0]).toMatchObject({ eventHint: eventID, isWholeDay: false });
		} finally {
			await cleanupCalendarEvents([eventID]);
		}
	});

	test('opens one all-day draft popover for a dragged month range', async ({ page }) => {
		const addedPayloads = await routeEventAdd(page);

		await dragBetweenCells(page, '2026-06-10', '2026-06-12');

		const popover = page.locator('.calendar-draft-popover');
		await expect(popover).toBeVisible();
		await expect(popover.getByLabel('종일')).toBeChecked();
		await popover.getByLabel('제목').fill('워크숍');
		await popover.getByLabel('제목').press('Enter');

		await expect.poll(() => addedPayloads.length).toBe(1);
		expect(addedPayloads[0]).toMatchObject({
			title: '워크숍',
			isWholeDay: true,
			startsAt: '2026-06-10T00:00:00.000Z',
			endsAt: '2026-06-13T00:00:00.000Z'
		});
	});
});
