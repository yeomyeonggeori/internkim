import { expect, test, type Page } from '@playwright/test';

test.describe('embedded calendar interactions', () => {
	test.beforeEach(async ({ page }) => {
		await page.route('**/calendar/api/events?**', async (route) => {
			await route.fulfill({ json: { events: [] } });
		});
		await page.route('**/calendar/api/remote-sync', async (route) => {
			await route.fulfill({ json: { synced: false } });
		});
		await page.route('**/calendar/api/conflicts', async (route) => {
			await route.fulfill({ json: { conflicts: [] } });
		});
		await page.route('**/admin/api/locale', async (route) => {
			await route.fulfill({ json: { locale: 'ko' } });
		});
	});

	test('creates one draft when double-clicking a timeline slot', async ({ page }) => {
		await page.goto('/calendar/embed');
		await createTimelineSlotByDoubleClick(page, '일');
		await expectTimelineDraftCount(page, 1);

		await page.goto('/calendar/embed');
		await createTimelineSlotByDoubleClick(page, '주');
		await expectTimelineDraftCount(page, 1);
	});

	test('creates one draft when dragging a timeline range', async ({ page }) => {
		await page.goto('/calendar/embed');
		await createTimelineRangeByDrag(page, '일');
		await expectTimelineDraftCount(page, 1);

		await page.goto('/calendar/embed');
		await createTimelineRangeByDrag(page, '주');
		await expectTimelineDraftCount(page, 1);
	});
});

async function createTimelineSlotByDoubleClick(page: Page, viewLabel: '일' | '주'): Promise<void> {
	await page.waitForSelector('.df-month-day-cell', { state: 'attached' });
	await page.getByRole('button', { name: viewLabel, exact: true }).click();
	await page.waitForSelector(timelineTargetSelector(viewLabel), { state: 'attached' });
	await page.evaluate((selector) => {
		const target = document.querySelector(selector);
		if (!(target instanceof HTMLElement)) throw new Error(`Missing timeline target: ${selector}`);
		const rectangle = target.getBoundingClientRect();
		const clientX = rectangle.left + rectangle.width / 2;
		const clientY = rectangle.top + 80;
		target.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, cancelable: true, button: 0, clientX, clientY }));
		target.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, cancelable: true, button: 0, clientX, clientY }));
		target.dispatchEvent(new MouseEvent('dblclick', { bubbles: true, cancelable: true, button: 0, clientX, clientY }));
	}, timelineTargetSelector(viewLabel));
}

async function createTimelineRangeByDrag(page: Page, viewLabel: '일' | '주'): Promise<void> {
	await page.waitForSelector('.df-month-day-cell', { state: 'attached' });
	await page.getByRole('button', { name: viewLabel, exact: true }).click();
	await page.waitForSelector(timelineTargetSelector(viewLabel), { state: 'attached' });
	await page.evaluate((selector) => {
		const target = document.querySelector(selector);
		if (!(target instanceof HTMLElement)) throw new Error(`Missing timeline target: ${selector}`);
		const rectangle = target.getBoundingClientRect();
		const startClientX = rectangle.left + rectangle.width / 2;
		const startClientY = rectangle.top + 80;
		const endClientY = startClientY + 140;
		target.dispatchEvent(
			new PointerEvent('pointerdown', {
				bubbles: true,
				cancelable: true,
				button: 0,
				pointerId: 1,
				clientX: startClientX,
				clientY: startClientY
			})
		);
		target.dispatchEvent(
			new MouseEvent('mousedown', {
				bubbles: true,
				cancelable: true,
				button: 0,
				clientX: startClientX,
				clientY: startClientY
			})
		);
		document.dispatchEvent(
			new PointerEvent('pointermove', {
				bubbles: true,
				cancelable: true,
				button: 0,
				pointerId: 1,
				clientX: startClientX,
				clientY: endClientY
			})
		);
		window.dispatchEvent(
			new PointerEvent('pointerup', {
				bubbles: true,
				cancelable: true,
				button: 0,
				pointerId: 1,
				clientX: startClientX,
				clientY: endClientY
			})
		);
	}, timelineTargetSelector(viewLabel));
}

function timelineTargetSelector(viewLabel: '일' | '주'): string {
	if (viewLabel === '일') return '.df-day-content-grid-column';
	return '.df-week-time-grid-cell';
}

async function expectTimelineDraftCount(page: Page, expectedCount: number): Promise<void> {
	await expect
		.poll(async () =>
			page.evaluate(() => {
				const eventIDs = Array.from(document.querySelectorAll('.df-event'))
					.map((element) => element.closest('[data-event-id]')?.getAttribute('data-event-id') ?? '')
					.filter(Boolean);
				return {
					total: new Set(eventIDs).size,
					timeline: new Set(eventIDs.filter((eventID) => eventID.startsWith('timeline-'))).size
				};
			})
		)
		.toEqual({ total: expectedCount, timeline: expectedCount });
}
