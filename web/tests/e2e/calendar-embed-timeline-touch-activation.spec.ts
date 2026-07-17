import { expect, test, type Locator, type Page } from '@playwright/test';
import { navigateEmbeddedCalendar, openCalendarEmbed } from './calendar-embed-interaction-helpers';
import { routeCalendarEvents, routeDefaultCalendarAPI } from './calendar-embed-test-utils';

test.describe('embedded calendar touch event activation', () => {
	test.use({ hasTouch: true, viewport: { width: 390, height: 844 } });

	test.beforeEach(async ({ page }) => {
		await routeDefaultCalendarAPI(page);
		await routeCalendarEvents(page, [
			{
				id: 'touch-activation-event',
				title: 'Touch Activation Event',
				startISO: '2026-06-08T09:00:00+09:00',
				endISO: '2026-06-08T10:00:00+09:00',
				isAllDay: false
			}
		]);
	});

	test('keeps a single tap as selection only', async ({ page }) => {
		await openCalendarEmbed(page, '일');

		const eventBlock = touchActivationEvent(page);
		await expect(eventBlock).toBeVisible();
		await eventBlock.tap();
		await expect(eventBlock).toHaveClass(/internkim-calendar-event-focused/);
		await expect(page.locator('.calendar-mobile-event-editor')).toHaveCount(0);
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
	});

	test('opens the compact editor after a double tap', async ({ page }) => {
		await openCalendarEmbed(page, '일');

		const eventBlock = touchActivationEvent(page);
		await expect(eventBlock).toBeVisible();
		await eventBlock.tap();
		await eventBlock.tap();

		await expect(eventBlock).toHaveClass(/internkim-calendar-event-focused/);
		await expect(page.locator('.calendar-mobile-event-editor')).toBeVisible();
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
	});

	test('lets DayFlow finish touch cleanup after a stationary tap', async ({ page }) => {
		await openCalendarEmbed(page, '일');

		const eventBlock = touchActivationEvent(page);
		await expect(eventBlock).toBeVisible();
		await eventBlock.tap();
		await page.waitForTimeout(700);

		await expect(eventBlock).toHaveAttribute('data-dragging', 'false');
		await expect(page.locator('body')).not.toHaveClass(/df-drag-active/);
		await expect(page.locator('.calendar-mobile-event-editor')).toHaveCount(0);
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
	});

	test('does not treat a tap after a moved gesture as a double tap', async ({ page }) => {
		await openCalendarEmbed(page, '일');

		const eventBlock = touchActivationEvent(page);
		await expect(eventBlock).toBeVisible();
		await eventBlock.evaluate((element) => {
			const dispatchTouch = (type: 'touchstart' | 'touchmove' | 'touchend', clientX: number, clientY: number) => {
				const touch = { identifier: 1, target: element, clientX, clientY };
				const createTouchList = (touches: typeof touch[]) =>
					Object.assign(touches, { item: (index: number) => touches[index] ?? null });
				const touchEvent = new Event(type, { bubbles: true, cancelable: true });
				Object.defineProperties(touchEvent, {
					touches: { value: createTouchList(type === 'touchend' ? [] : [touch]) },
					changedTouches: { value: createTouchList([touch]) }
				});
				element.dispatchEvent(touchEvent);
			};
			dispatchTouch('touchstart', 100, 100);
			dispatchTouch('touchend', 100, 100);
			dispatchTouch('touchstart', 100, 100);
			dispatchTouch('touchmove', 107, 100);
			dispatchTouch('touchend', 107, 100);
			dispatchTouch('touchstart', 100, 100);
			dispatchTouch('touchend', 100, 100);
		});

		await expect(page.locator('.calendar-mobile-event-editor')).toHaveCount(0);
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
	});

	test('opens the anchored editor after a double tap at the compact boundary', async ({ page }) => {
		await page.setViewportSize({ width: 768, height: 900 });
		await openCalendarEmbed(page, '일');

		const eventBlock = touchActivationEvent(page);
		await expect(eventBlock).toBeVisible();
		await eventBlock.tap();
		await eventBlock.tap();

		await expect(eventBlock).toHaveClass(/internkim-calendar-event-focused/);
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await expect(page.locator('.calendar-mobile-event-editor')).toHaveCount(0);
	});

	test('opens a visible month event compact editor after a double tap', async ({ page }) => {
		await openCalendarEmbed(page, '월');
		await navigateEmbeddedCalendar(page, '2026-06-08');

		const eventBlock = page.locator(
			'[data-event-id="touch-activation-event"].calendar-month-direct-event'
		);
		await expect(eventBlock).toBeVisible();
		const eventBlockBox = await eventBlock.boundingBox();
		expect(eventBlockBox?.height ?? 0).toBeGreaterThanOrEqual(24);
		await eventBlock.tap();
		await eventBlock.tap();

		await expect(eventBlock).toHaveClass(/internkim-calendar-event-focused/);
		await expect(page.locator('.calendar-mobile-event-editor')).toBeVisible();
	});

	test('opens a hidden month event compact editor after a double tap', async ({ page }) => {
		await routeCalendarEvents(
			page,
			Array.from({ length: 9 }, (_, index) => ({
				id: `touch-overflow-${index}`,
				title: `Touch Overflow ${index}`,
				startISO: `2026-06-17T${String(9 + index).padStart(2, '0')}:00:00+09:00`,
				endISO: `2026-06-17T${String(10 + index).padStart(2, '0')}:00:00+09:00`,
				isAllDay: false
			}))
		);
		await openCalendarEmbed(page, '월');
		await navigateEmbeddedCalendar(page, '2026-06-16');
		const moreButton = page.locator('.calendar-month-more-button[data-date-key="2026-06-17"]');
		await expect(moreButton).toBeVisible();
		const moreButtonBox = await moreButton.boundingBox();
		expect(moreButtonBox?.height ?? 0).toBeGreaterThanOrEqual(24);
		await moreButton.tap();

		const hiddenEvent = page.locator('.calendar-month-more-popover-event').first();
		await expect(hiddenEvent).toBeVisible();
		const hiddenEventBox = await hiddenEvent.boundingBox();
		expect(hiddenEventBox?.height ?? 0).toBeGreaterThanOrEqual(24);
		await hiddenEvent.tap();
		await hiddenEvent.tap();

		await expect(page.locator('.calendar-stage')).toHaveAttribute('data-calendar-selected-date-key', '2026-06-17');
		await expect(page.locator('.calendar-mobile-event-editor')).toBeVisible();
	});
});

function touchActivationEvent(page: Page): Locator {
	return page.locator(
		'.calendar-stage [data-event-id="touch-activation-event"].df-day-event:not(.df-right-panel-event-card)'
	);
}
