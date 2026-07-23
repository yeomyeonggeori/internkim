import { expect, test } from '@playwright/test';
import {
	dispatchMonthRangePointerDrag,
	dragBetweenCells,
	routeCalendarAPI,
	scrollMonthViewBy,
	startDragBetweenCells,
	waitForClientHydration
} from './calendar-draft-popover-test-utils';

test.describe('calendar draft popover anchors', () => {
	test.beforeEach(async ({ page }) => {
		await routeCalendarAPI(page);
	});

	test('keeps edit popovers attached to their anchor with bounded internal layout', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 760 });
		await page.unroute('**/calendar/api/events?**');
		await page.route('**/calendar/api/events?**', async (route) => {
			await route.fulfill({
				json: {
					events: [
						{
							id: 'lower-edit-event',
							title: '하단 편집 일정',
							description: '',
							location: '',
							startISO: '2026-06-17T00:00:00.000Z',
							endISO: '2026-06-18T00:00:00.000Z',
							timeZone: 'Asia/Seoul',
							isAllDay: true,
							color: '#1677ff',
							updatedAt: '2026-06-10T11:30:00.000Z'
						}
					]
				}
			});
		});
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		await page.locator('.calendar-month-direct-event[data-event-id="lower-edit-event"]').click();
		const popover = page.locator('.calendar-draft-popover');
		await expect(popover).toBeVisible();

		await expect
			.poll(async () =>
				page.evaluate(() => {
					const eventElement = document.querySelector<HTMLElement>(
						'.calendar-month-direct-event[data-event-id="lower-edit-event"]'
					);
					const titleElement = eventElement?.querySelector<HTMLElement>('.calendar-month-event-title, .calendar-event-title');
					const popover = document.querySelector<HTMLElement>('.calendar-draft-popover');
					const scrollContainer = document.querySelector<HTMLElement>('.draft-popover-body');
					if (!titleElement || !popover || !scrollContainer) return false;
					const titleRectangle = titleElement.getBoundingClientRect();
					const popoverRectangle = popover.getBoundingClientRect();
					const arrowTop = Number.parseFloat(window.getComputedStyle(popover, '::before').top);
					const overflowY = window.getComputedStyle(scrollContainer).overflowY;
					const arrowY = popoverRectangle.top + arrowTop + 8;
					const titleCenterY = titleRectangle.top + titleRectangle.height / 2;
					return (
						Math.abs(arrowY - titleCenterY) <= 18 &&
						popoverRectangle.bottom <= window.innerHeight &&
						overflowY === 'scroll'
					);
				})
			)
			.toBe(true);
	});

	test('closes month edit popovers immediately when the calendar scrolls', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 560 });
		const savedPayloads: unknown[] = [];
		await page.unroute('**/calendar/api/events?**');
		await page.route('**/calendar/api/events?**', async (route) => {
			await route.fulfill({
				json: {
					events: [
						{
							id: 'scroll-anchor-event',
							title: '스크롤 기준 일정',
							description: '',
							location: '',
							startISO: '2026-06-17T00:00:00.000Z',
							endISO: '2026-06-18T00:00:00.000Z',
							timeZone: 'Asia/Seoul',
							isAllDay: true,
							color: '#1677ff',
							updatedAt: '2026-06-10T11:30:00.000Z'
						}
					]
				}
			});
		});
		await page.route('**/calendar/api/events/scroll-anchor-event', async (route) => {
			if (route.request().method() === 'PUT') savedPayloads.push(route.request().postDataJSON());
			await route.fulfill({
				json: {
					id: 'scroll-anchor-event',
					title: '스크롤 저장 일정',
					description: '',
					location: '',
					startISO: '2026-06-17T00:00:00.000Z',
					endISO: '2026-06-18T00:00:00.000Z',
					timeZone: 'Asia/Seoul',
					isAllDay: true,
					color: '#1677ff',
					updatedAt: '2026-06-10T12:00:00.000Z'
				}
			});
		});
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		await page.locator('.calendar-month-direct-event[data-event-id="scroll-anchor-event"]').click();
		const popover = page.locator('.calendar-draft-popover');
		await expect(popover).toBeVisible();
		await page.getByLabel('제목').fill('스크롤 저장 일정');

		await scrollMonthViewBy(page, 80);

		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		await expect.poll(() => savedPayloads.length).toBe(1);
		expect(savedPayloads[0]).toMatchObject({ title: '스크롤 저장 일정' });
	});

	test('places a dragged month range preview below overlapping all-day events', async ({ page }) => {
		await page.unroute('**/calendar/api/events?**');
		await page.route('**/calendar/api/events?**', async (route) => {
			await route.fulfill({
				json: {
					events: [
						{
							id: 'existing-all-day',
							title: '기존 종일 일정',
							startISO: '2026-06-10T00:00:00.000Z',
							endISO: '2026-06-13T00:00:00.000Z',
							isAllDay: true
						}
					]
				}
			});
		});
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);
		await expect(page.locator('.calendar-month-direct-event[data-event-id="existing-all-day"]')).toBeVisible();

		await startDragBetweenCells(page, '2026-06-10', '2026-06-12');

		const verticalGap = await page.evaluate(() => {
			const existingEvent = document.querySelector('.calendar-month-direct-event[data-event-id="existing-all-day"]');
			const preview = document.querySelector('.month-range-preview');
			if (!(existingEvent instanceof HTMLElement) || !(preview instanceof HTMLElement)) return Number.NEGATIVE_INFINITY;
			const existingRectangle = existingEvent.getBoundingClientRect();
			const previewRectangle = preview.getBoundingClientRect();
			return Math.round(previewRectangle.top - existingRectangle.bottom);
		});
		expect(verticalGap).toBeGreaterThanOrEqual(2);
		await page.mouse.up();
	});

	test('cancels a dragged month range on pointer cancel without opening a draft', async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		await dispatchMonthRangePointerDrag(page, '2026-06-10', '2026-06-12', 'cancel');

		await expect(page.locator('.month-range-preview')).toHaveCount(0);
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		await expect(page.locator('[data-event-id^="range-"]')).toHaveCount(0);
	});

	test('uses safe timed values when converting a dragged all-day range to timed', async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		await dragBetweenCells(page, '2026-06-10', '2026-06-12');

		const popover = page.locator('.calendar-draft-popover');
		await expect(popover).toBeVisible();
		await expect(popover.getByLabel('종일')).toBeChecked();

		await popover.getByLabel('종일').uncheck();

		await expect(popover.getByRole('button', { name: /시작 날짜 2026\.06\.10 09:00/ })).toBeVisible();
		await expect(popover.getByRole('button', { name: /종료 날짜 2026\.06\.12 10:00/ })).toBeVisible();
	});

	test('keeps compact event editor actions accessible on a narrow viewport', async ({ page }) => {
		await page.setViewportSize({ width: 390, height: 640 });
		await page.unroute('**/calendar/api/events?**');
		await page.route('**/calendar/api/events?**', async (route) => {
			await route.fulfill({
				json: {
					events: [
						{
							id: 'narrow-popover-event',
							title: '좁은 화면 일정',
							description: '작은 화면에서 버튼과 시간 입력이 보여야 합니다.',
							location: '회의실',
							startISO: '2026-06-17T00:30:00.000Z',
							endISO: '2026-06-17T01:30:00.000Z',
							timeZone: 'Asia/Seoul',
							isAllDay: false,
							color: '#1677ff',
							updatedAt: '2026-06-10T11:30:00.000Z'
						}
					]
				}
			});
		});
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		await page.locator('.calendar-month-direct-event[data-event-id="narrow-popover-event"]').click();
		const editor = page.locator('.calendar-mobile-event-editor');
		await expect(editor).toBeVisible();
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		await expect(editor.locator('.df-mobile-event-drawer-header-action').filter({ hasText: '취소' })).toBeVisible();
		await expect(editor.getByRole('button', { name: '완료' })).toBeVisible();
		await expect(editor.locator('input[data-mobile-editor-field="title"]')).toHaveValue('좁은 화면 일정');
		const deleteButton = editor.getByRole('button', { name: '삭제' });
		await deleteButton.scrollIntoViewIfNeeded();
		await expect(deleteButton).toBeVisible();

		const geometry = await page.evaluate(() => {
			const panel = document.querySelector<HTMLElement>('.calendar-mobile-event-editor .df-mobile-event-drawer-panel');
			const body = document.querySelector<HTMLElement>('.calendar-mobile-event-editor .df-mobile-event-drawer-body');
			if (!panel || !body) throw new Error('Missing compact event editor elements');
			const panelRectangle = panel.getBoundingClientRect();
			return {
				panelBottom: Math.round(panelRectangle.bottom),
				panelLeft: Math.round(panelRectangle.left),
				panelRight: Math.round(panelRectangle.right),
				panelTop: Math.round(panelRectangle.top),
				scrollOverflowY: window.getComputedStyle(body).overflowY,
				viewportHeight: window.innerHeight,
				viewportWidth: window.innerWidth
			};
		});

		expect(geometry.panelTop).toBeGreaterThanOrEqual(0);
		expect(geometry.panelLeft).toBeGreaterThanOrEqual(0);
		expect(geometry.panelRight).toBeLessThanOrEqual(geometry.viewportWidth);
		expect(geometry.panelBottom).toBeLessThanOrEqual(geometry.viewportHeight);
		expect(geometry.scrollOverflowY).toBe('auto');
	});

	test('keeps more popovers and edit actions accessible when viewport height is limited', async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 640 });
		await page.unroute('**/calendar/api/events?**');
		await page.route('**/calendar/api/events?**', async (route) => {
			await route.fulfill({
				json: {
					events: Array.from({ length: 8 }, (_, index) => ({
						id: `low-height-more-event-${index}`,
						title: index === 3 ? '긴급 회식 장소 확인' : `낮은 화면 더보기 ${index}`,
						description: index === 3 ? '17일 더보기 안에서 열어볼 수 있는 일정입니다.' : '',
						location: index === 3 ? '성수' : '',
						startISO: `2026-06-17T${String(9 + index).padStart(2, '0')}:00:00+09:00`,
						endISO: `2026-06-17T${String(10 + index).padStart(2, '0')}:00:00+09:00`,
						timeZone: 'Asia/Seoul',
						isAllDay: false,
						color: '#1677ff',
						updatedAt: `2026-06-${String(10 + index).padStart(2, '0')}T00:00:00Z`
					}))
				}
			});
		});
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		await page.locator('.calendar-month-more-button[data-date-key="2026-06-17"]').click();
		const morePopover = page.locator('.calendar-month-more-popover');
		await expect(morePopover).toBeVisible();
		await morePopover.getByRole('button', { name: /긴급 회식 장소 확인/ }).click();

		const popover = page.locator('.calendar-draft-popover');
		await expect(popover).toBeVisible();
		await expect(morePopover).toBeVisible();
		await expect(popover.getByRole('button', { name: /시작 날짜 2026\.06\.17/ })).toBeVisible();
		await expect(popover).toHaveClass(/draft-popover-can-scroll-down/);

		const geometry = await page.evaluate(() => {
			const popover = document.querySelector<HTMLElement>('.calendar-draft-popover');
			const scrollContainer = document.querySelector<HTMLElement>('.draft-popover-body');
			const startSummary = document.querySelector<HTMLButtonElement>('.calendar-draft-popover [data-date-time-summary="start"]');
			const footer = document.querySelector<HTMLElement>('.draft-popover-footer');
			const deleteButton = document.querySelector<HTMLButtonElement>('.draft-popover-delete');
			const cancelButton = document.querySelector<HTMLButtonElement>('.draft-popover-cancel');
			const completeButton = document.querySelector<HTMLButtonElement>('.draft-popover-complete');
			if (!popover || !scrollContainer || !startSummary || !footer || !deleteButton || !cancelButton || !completeButton) {
				throw new Error('Missing low-height popover elements');
			}
			const popoverRectangle = popover.getBoundingClientRect();
			const startSummaryRectangle = startSummary.getBoundingClientRect();
			const footerRectangle = footer.getBoundingClientRect();
			const buttonRectangles = [deleteButton, cancelButton, completeButton].map((button) => button.getBoundingClientRect());
			const footerTopBeforeScroll = Math.round(footerRectangle.top);
			const scrollDownAffordance = popover.classList.contains('draft-popover-can-scroll-down');
			scrollContainer.scrollTop = Math.min(48, scrollContainer.scrollHeight - scrollContainer.clientHeight);
			scrollContainer.dispatchEvent(new Event('scroll', { bubbles: true }));
			const footerTopAfterScroll = Math.round(footer.getBoundingClientRect().top);
			return {
				bodyCanScroll: scrollContainer.scrollHeight > scrollContainer.clientHeight,
				bodyScrollTop: scrollContainer.scrollTop,
				popoverBottom: Math.round(popoverRectangle.bottom),
				popoverRight: Math.round(popoverRectangle.right),
				popoverWidth: Math.round(popoverRectangle.width),
				footerBottom: Math.round(footerRectangle.bottom),
				footerHeight: Math.round(footerRectangle.height),
				footerInsideBody: scrollContainer.contains(footer),
				footerTopAfterScroll,
				footerTopBeforeScroll,
				maxButtonBottom: Math.max(...buttonRectangles.map((rectangle) => Math.round(rectangle.bottom))),
				scrollDownAffordance,
				scrollOverflowY: window.getComputedStyle(scrollContainer).overflowY,
				startSummaryRight: Math.round(startSummaryRectangle.right),
				viewportHeight: window.innerHeight
			};
		});

		expect(geometry.bodyCanScroll).toBe(true);
		expect(geometry.bodyScrollTop).toBeGreaterThan(0);
		expect(geometry.footerInsideBody).toBe(false);
		expect(geometry.footerTopAfterScroll).toBe(geometry.footerTopBeforeScroll);
		expect(geometry.popoverBottom).toBeLessThanOrEqual(geometry.viewportHeight);
		expect(geometry.popoverWidth).toBe(400);
		expect(geometry.footerBottom).toBeLessThanOrEqual(geometry.viewportHeight);
		expect(geometry.footerHeight).toBe(65);
		expect(geometry.maxButtonBottom).toBeLessThanOrEqual(geometry.viewportHeight);
		expect(geometry.scrollDownAffordance).toBe(true);
		expect(geometry.startSummaryRight).toBeLessThanOrEqual(geometry.popoverRight - 12);
		expect(geometry.scrollOverflowY).toBe('scroll');
	});
});
