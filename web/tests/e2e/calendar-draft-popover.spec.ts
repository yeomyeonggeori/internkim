import { expect, test } from '@playwright/test';
import {
	clickOutsideDraftPopover,
	draftPopoverEvent,
	dragBetweenCells,
	routeCalendarAPI,
	routeDraftPopoverEvents,
	routeDraftPopoverEventUpdate,
	waitForClientHydration
} from './calendar-draft-popover-test-utils';

test.describe('calendar draft popover', () => {
	test.beforeEach(async ({ page }) => {
		await routeCalendarAPI(page);
	});

	test('opens a custom draft popover from the New button without posting a default title event', async ({ page }) => {
		let eventCreateCount = 0;
		await page.route('**/calendar/api/events', async (route) => {
			if (route.request().method() === 'POST') eventCreateCount += 1;
			await route.fulfill({
				json: {
					id: 'created-event',
					title: 'Created event',
					startISO: '2026-06-08T09:00:00.000Z',
					endISO: '2026-06-08T10:00:00.000Z',
					isAllDay: false
				}
			});
		});

		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		await page.getByRole('button', { name: '새로 만들기' }).click();

		const popover = page.locator('.calendar-draft-popover');
		await expect(popover).toBeVisible();
		await expect(popover.getByLabel('제목')).toBeVisible();
		await expect(popover.getByLabel('제목')).toBeFocused();
		await expect(popover.getByLabel('종일')).toBeVisible();
		await expect(popover.locator('.event-audit-card')).toHaveCount(0);
		expect(eventCreateCount).toBe(0);
	});

	test('uses compact date time summaries and an accessible picker', async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		await page.getByRole('button', { name: '새로 만들기' }).click();
		const popover = page.locator('.calendar-draft-popover');

		await expect(popover.getByRole('button', { name: /시작 날짜 2026\.06\.08 09:00/ })).toBeVisible();
		await expect(popover.getByRole('button', { name: /종료 날짜 2026\.06\.08 10:00/ })).toBeVisible();
		await expect(popover.locator('input[type="date"]')).toHaveCount(0);
		await expect(popover.locator('input[type="time"]')).toHaveCount(0);

		await popover.getByRole('button', { name: /시작 날짜 2026\.06\.08 09:00/ }).click();
		const picker = page.locator('.draft-date-time-picker');
		await expect(picker).toBeVisible();
		await expect(picker).toHaveAttribute('aria-label', '시작 날짜 및 시간 수정');
		await picker.getByRole('button', { name: '2026년 6월 17일' }).click();
		await picker.getByLabel('시').selectOption('14');
		await picker.getByLabel('분').selectOption('50');
		await picker.getByRole('button', { name: '저장하기' }).click();

		await expect(popover.getByRole('button', { name: /시작 날짜 2026\.06\.17 14:50/ })).toBeVisible();
		await expect(popover.getByRole('button', { name: /종료 날짜 2026\.06\.17 15:50/ })).toBeVisible();
	});

	test('posts the draft only after the popover is completed', async ({ page }) => {
		const postedPayloads: unknown[] = [];
		await page.route('**/calendar/api/events', async (route) => {
			const payload = route.request().postDataJSON() as Record<string, unknown>;
			postedPayloads.push(payload);
			await route.fulfill({
				json: {
					id: String(payload.eventID),
					title: String(payload.title),
					description: String(payload.description ?? ''),
					location: String(payload.location ?? ''),
					startISO: String(payload.startISO),
					endISO: String(payload.endISO),
					isAllDay: Boolean(payload.isAllDay),
					updatedAt: '2026-06-08T12:00:00.000Z'
				}
			});
		});

		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		await page.getByRole('button', { name: '새로 만들기' }).click();
		await page.getByLabel('제목').fill('디자인 리뷰');
		await page.getByLabel('장소').fill('회의실 A');
		await page.getByLabel('설명').fill('월간 디자인 점검');
		expect(postedPayloads).toHaveLength(0);

		await page.getByRole('button', { name: '완료' }).click();

		await expect.poll(() => postedPayloads.length).toBe(1);
		expect(postedPayloads[0]).toMatchObject({
			title: '디자인 리뷰',
			location: '회의실 A',
			description: '월간 디자인 점검',
			isAllDay: false
		});
		await expect(page.locator('.calendar-draft-popover')).toBeHidden();
	});

	test('saves titled drafts and cancels untitled drafts when clicking outside the popover', async ({ page }) => {
		const postedPayloads: unknown[] = [];
		await page.route('**/calendar/api/events', async (route) => {
			const payload = route.request().postDataJSON() as Record<string, unknown>;
			postedPayloads.push(payload);
			await route.fulfill({
				json: {
					id: String(payload.eventID),
					title: String(payload.title),
					description: String(payload.description ?? ''),
					location: String(payload.location ?? ''),
					startISO: String(payload.startISO),
					endISO: String(payload.endISO),
					isAllDay: Boolean(payload.isAllDay),
					updatedAt: '2026-06-08T12:00:00.000Z'
				}
			});
		});
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		for (const viewLabel of ['일', '주', '월']) {
			await page.getByRole('button', { name: viewLabel, exact: true }).click();
			await page.getByRole('button', { name: '새로 만들기' }).click();
			await page.getByLabel('제목').fill(`${viewLabel} 바깥 저장`);
			await clickOutsideDraftPopover(page);
			await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		}

		await expect.poll(() => postedPayloads.length).toBe(3);
		expect(postedPayloads.map((payload) => (payload as { title?: string }).title)).toEqual([
			'일 바깥 저장',
			'주 바깥 저장',
			'월 바깥 저장'
		]);

		for (const viewLabel of ['일', '주', '월']) {
			await page.getByRole('button', { name: viewLabel, exact: true }).click();
			await page.getByRole('button', { name: '새로 만들기' }).click();
			await clickOutsideDraftPopover(page);
			await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		}

		await expect.poll(() => postedPayloads.length).toBe(3);
	});

	test('does not save an unchanged existing event when dismissing the edit popover', async ({ page }) => {
		const existingEvent = draftPopoverEvent({ id: 'unchanged-edit-event', title: '그대로인 일정' });
		await routeDraftPopoverEvents(page, [existingEvent]);
		const updatedPayloads = await routeDraftPopoverEventUpdate(page, existingEvent);
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		await page.locator('.calendar-month-direct-event[data-event-id="unchanged-edit-event"]').dblclick();
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		const unexpectedUpdate = page
			.waitForRequest(
				(request) => request.method() === 'PUT' && request.url().includes('/calendar/api/events/unchanged-edit-event'),
				{ timeout: 500 }
			)
			.then(() => true, () => false);

		await clickOutsideDraftPopover(page);

		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		await expect(unexpectedUpdate).resolves.toBe(false);
		expect(updatedPayloads).toHaveLength(0);
	});

	test('saves a changed existing event when dismissing the edit popover', async ({ page }) => {
		const existingEvent = draftPopoverEvent({ id: 'changed-edit-event', title: '수정 전 일정' });
		await routeDraftPopoverEvents(page, [existingEvent]);
		const updatedPayloads = await routeDraftPopoverEventUpdate(page, existingEvent);
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		await page.locator('.calendar-month-direct-event[data-event-id="changed-edit-event"]').dblclick();
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await page.getByLabel('제목').fill('수정 후 일정');
		await page.getByLabel('장소').fill('회의실 B');

		await clickOutsideDraftPopover(page);

		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		await expect.poll(() => updatedPayloads.length).toBe(1);
		expect(updatedPayloads[0]).toMatchObject({
			title: '수정 후 일정',
			location: '회의실 B'
		});
	});

	test('disables completion when a timed draft end is not after the start', async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		await page.getByRole('button', { name: '새로 만들기' }).click();
		const popover = page.locator('.calendar-draft-popover');
		await page.getByLabel('제목').fill('시간 검증');
		await expect(popover.getByRole('button', { name: '완료' })).toBeEnabled();

		await popover.getByRole('button', { name: /종료 날짜 2026\.06\.08 10:00/ }).click();
		const picker = page.locator('.draft-date-time-picker');
		await picker.getByLabel('시').selectOption('09');
		await picker.getByLabel('분').selectOption('00');
		await picker.getByRole('button', { name: '저장하기' }).click();

		await expect(popover.getByRole('button', { name: '완료' })).toBeDisabled();
	});

	test('keeps month single click as selection and opens the popover on double click', async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		const dateCell = page.locator('.df-month-day-cell[data-date="2026-06-10"]');
		await dateCell.click();
		await expect(dateCell).toHaveClass(/month-selected-date/);
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);

		await dateCell.dblclick();

		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await expect(page.getByLabel('제목')).toBeVisible();
	});

	test('cancels a new draft without leaving a local event behind', async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		await page.getByRole('button', { name: '새로 만들기' }).click();
		await expect(page.locator('[data-event-id^="quick-"]')).toHaveCount(1);

		await page.locator('.draft-popover-footer').getByRole('button', { name: '취소' }).click();

		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		await expect(page.locator('[data-event-id^="quick-"]')).toHaveCount(0);
	});

	test('opens one all-day draft popover for a dragged month range', async ({ page }) => {
		const postedPayloads: unknown[] = [];
		await page.route('**/calendar/api/events', async (route) => {
			const payload = route.request().postDataJSON() as Record<string, unknown>;
			postedPayloads.push(payload);
			await route.fulfill({
				json: {
					id: String(payload.eventID),
					title: String(payload.title),
					description: String(payload.description ?? ''),
					location: String(payload.location ?? ''),
					startISO: String(payload.startISO),
					endISO: String(payload.endISO),
					isAllDay: Boolean(payload.isAllDay),
					updatedAt: '2026-06-08T12:00:00.000Z'
				}
			});
		});
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		await dragBetweenCells(page, '2026-06-10', '2026-06-12');

		const popover = page.locator('.calendar-draft-popover');
		await expect(popover).toBeVisible();
		await expect(popover.getByLabel('종일')).toBeChecked();
		await page.getByLabel('제목').fill('워크숍');
		await page.getByRole('button', { name: '완료' }).click();

		await expect.poll(() => postedPayloads.length).toBe(1);
		expect(postedPayloads[0]).toMatchObject({
			title: '워크숍',
			isAllDay: true,
			startISO: '2026-06-10T00:00:00.000Z',
			endISO: '2026-06-13T00:00:00.000Z'
		});
	});

	test('places month creation popovers beside the rendered draft event block', async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		await page.locator('.calendar-stage .df-month-day-cell[data-date="2026-06-20"]').dblclick();
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await expect(page.locator('.calendar-month-direct-event.draft-empty-title-event')).toBeVisible();

		await expect
			.poll(async () =>
				page.evaluate(() => {
					const draftEvent = document.querySelector<HTMLElement>('.calendar-month-direct-event.draft-empty-title-event');
					const popover = document.querySelector<HTMLElement>('.calendar-draft-popover');
					if (!draftEvent || !popover) return false;
					const draftRectangle = draftEvent.getBoundingClientRect();
					const popoverRectangle = popover.getBoundingClientRect();
					return popover.classList.contains('popover-arrow-right') && popoverRectangle.right <= draftRectangle.left - 8;
				})
			)
			.toBe(true);
	});

	test('does not visibly flash month creation popovers before the draft event block is ready', async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		await page.locator('.calendar-stage .df-month-day-cell[data-date="2026-06-20"]').dblclick();

		const showedPopoverBeforeDraftEvent = await page.evaluate(() => {
			const popover = document.querySelector<HTMLElement>('.calendar-draft-popover');
			const draftEvent = document.querySelector<HTMLElement>('.calendar-month-direct-event.draft-empty-title-event');
			if (!popover || draftEvent) return false;
			const rectangle = popover.getBoundingClientRect();
			const style = window.getComputedStyle(popover);
			return rectangle.width > 0 && rectangle.height > 0 && style.visibility !== 'hidden' && style.opacity !== '0';
		});

		expect(showedPopoverBeforeDraftEvent).toBe(false);
		await expect(page.locator('.calendar-month-direct-event.draft-empty-title-event')).toBeVisible();
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
	});

});
