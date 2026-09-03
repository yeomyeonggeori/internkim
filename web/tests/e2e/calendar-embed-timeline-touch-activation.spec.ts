import { expect, test, type Locator, type Page } from '@playwright/test';
import { cleanupCalendarEvents, seedCalendarEvents, signInToCalendar } from './calendar-central-test-utils';
import { openCalendarEmbed } from './calendar-embed-interaction-helpers';

async function routeCalendarHolidays(page: Page): Promise<void> {
	await page.route('**/api/calendar/holidays?**', async (route) => {
		await route.fulfill({ json: { holidays: [], degraded: false } });
	});
}

function touchActivationEvent(page: Page, eventID: string): Locator {
	return page.locator(`[data-calendar-event-id="${eventID}"]`);
}

test.describe('embedded calendar touch event activation', () => {
	test.use({ hasTouch: true, viewport: { width: 390, height: 844 }, locale: 'ko-KR' });

	test.beforeEach(async ({ page }) => {
		await routeCalendarHolidays(page);
	});

	test('opens the draft popover after a single tap', async ({ page }) => {
		const [eventID] = await seedCalendarEvents([
			{ title: 'Touch Activation Event', startISO: '2026-06-08T09:00:00+09:00', endISO: '2026-06-08T10:00:00+09:00' }
		]);
		try {
			await signInToCalendar(page);
			await openCalendarEmbed(page, '일');

			const eventBlock = touchActivationEvent(page, eventID);
			await expect(eventBlock).toBeVisible();
			await eventBlock.tap();
			await expect(eventBlock).toHaveAttribute('data-selected', '');
			await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		} finally {
			await cleanupCalendarEvents([eventID]);
		}
	});

	test('does not open a popover after a moved touch gesture', async ({ page }) => {
		const [eventID] = await seedCalendarEvents([
			{ title: 'Touch Activation Event', startISO: '2026-06-08T09:00:00+09:00', endISO: '2026-06-08T10:00:00+09:00' }
		]);
		try {
			await signInToCalendar(page);
			await openCalendarEmbed(page, '일');

			const eventBlock = touchActivationEvent(page, eventID);
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
				dispatchTouch('touchmove', 107, 140);
				dispatchTouch('touchend', 107, 140);
			});

			await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		} finally {
			await cleanupCalendarEvents([eventID]);
		}
	});

	test('opens the draft popover after a single tap at a wider, compact-boundary viewport', async ({ page }) => {
		await page.setViewportSize({ width: 768, height: 900 });
		const [eventID] = await seedCalendarEvents([
			{ title: 'Touch Activation Event', startISO: '2026-06-08T09:00:00+09:00', endISO: '2026-06-08T10:00:00+09:00' }
		]);
		try {
			await signInToCalendar(page);
			await openCalendarEmbed(page, '일');

			const eventBlock = touchActivationEvent(page, eventID);
			await expect(eventBlock).toBeVisible();
			await eventBlock.tap();

			await expect(eventBlock).toHaveAttribute('data-selected', '');
			await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		} finally {
			await cleanupCalendarEvents([eventID]);
		}
	});

	test('opens the draft popover for a visible month event after a single tap', async ({ page }) => {
		const [eventID] = await seedCalendarEvents([
			{ title: 'Touch Activation Event', startISO: '2026-06-08T09:00:00+09:00', endISO: '2026-06-08T10:00:00+09:00' }
		]);
		try {
			await signInToCalendar(page);
			await openCalendarEmbed(page, '월');

			const eventBlock = touchActivationEvent(page, eventID);
			await expect(eventBlock).toBeVisible();
			const eventBlockBox = await eventBlock.boundingBox();
			expect(eventBlockBox?.height ?? 0).toBeGreaterThanOrEqual(18);
			await eventBlock.tap();

			await expect(eventBlock).toHaveAttribute('data-selected', '');
			await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		} finally {
			await cleanupCalendarEvents([eventID]);
		}
	});
});
