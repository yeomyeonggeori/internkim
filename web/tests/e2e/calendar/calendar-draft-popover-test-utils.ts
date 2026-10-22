import type { Page } from '@playwright/test';

export async function routeCalendarHolidays(page: Page): Promise<void> {
	await page.route('**/api/calendar/holidays?**', async (route) => {
		await route.fulfill({ json: { holidays: [], degraded: false } });
	});
}

export type CalendarToolInvokeInput = Record<string, unknown>;

export async function routeEventAdd(page: Page): Promise<CalendarToolInvokeInput[]> {
	const payloads: CalendarToolInvokeInput[] = [];
	await page.route('**/api/v1/tools/event_add/invoke', async (route) => {
		const body = route.request().postDataJSON() as { input: CalendarToolInvokeInput };
		payloads.push(body.input);
		await route.fulfill({
			json: {
				result: {
					eventID: `added-${payloads.length}`,
					title: String(body.input.title ?? ''),
					note: String(body.input.note ?? ''),
					location: String(body.input.location ?? ''),
					startsAt: String(body.input.startsAt),
					endsAt: String(body.input.endsAt),
					isWholeDay: Boolean(body.input.isWholeDay),
					participants: [],
					notifyMinutesBefore: Number(body.input.notifyMinutesBefore ?? 0),
					updatedAt: '2026-06-08T12:00:00.000Z'
				}
			}
		});
	});
	return payloads;
}

export async function routeEventUpdate(page: Page): Promise<CalendarToolInvokeInput[]> {
	const payloads: CalendarToolInvokeInput[] = [];
	await page.route('**/api/v1/tools/event_update/invoke', async (route) => {
		const body = route.request().postDataJSON() as { input: CalendarToolInvokeInput };
		payloads.push(body.input);
		await route.fulfill({
			json: {
				result: {
					eventID: String(body.input.eventHint ?? ''),
					title: String(body.input.title ?? ''),
					note: String(body.input.note ?? ''),
					location: String(body.input.location ?? ''),
					startsAt: String(body.input.startsAt),
					endsAt: String(body.input.endsAt),
					isWholeDay: Boolean(body.input.isWholeDay),
					participants: [],
					notifyMinutesBefore: Number(body.input.notifyMinutesBefore ?? 0),
					updatedAt: '2026-06-08T12:00:05.000Z'
				}
			}
		});
	});
	return payloads;
}

export async function routeEventDelete(page: Page): Promise<CalendarToolInvokeInput[]> {
	const payloads: CalendarToolInvokeInput[] = [];
	await page.route('**/api/v1/tools/event_delete/invoke', async (route) => {
		const body = route.request().postDataJSON() as { input: CalendarToolInvokeInput };
		payloads.push(body.input);
		await route.fulfill({ json: { result: {} } });
	});
	return payloads;
}

export async function waitForClientHydration(page: Page): Promise<void> {
	await page.locator('.calendar-toolbar-title').waitFor({ state: 'visible' });
	await page.waitForTimeout(1_500);
	await page.evaluate(
		() =>
			new Promise<void>((resolve) => {
				requestAnimationFrame(() => requestAnimationFrame(() => resolve()));
			})
	);
}

export async function clickOutsideDraftPopover(page: Page): Promise<void> {
	const clickPoint = await page.evaluate(() => {
		const popover = document.querySelector('.calendar-draft-popover');
		const stage = document.querySelector('.calendar-stage');
		if (!(popover instanceof HTMLElement) || !(stage instanceof HTMLElement)) {
			throw new Error('Missing draft popover or calendar stage');
		}
		const stageRectangle = stage.getBoundingClientRect();
		const popoverRectangle = popover.getBoundingClientRect();
		const candidates = [
			{ x: stageRectangle.left + 24, y: stageRectangle.top + 72 },
			{ x: stageRectangle.right - 24, y: stageRectangle.top + 72 },
			{ x: stageRectangle.left + 24, y: stageRectangle.bottom - 24 },
			{ x: stageRectangle.left + stageRectangle.width / 2, y: stageRectangle.top + stageRectangle.height / 2 }
		];
		const point = candidates.find(
			(candidate) =>
				candidate.x < popoverRectangle.left ||
				candidate.x > popoverRectangle.right ||
				candidate.y < popoverRectangle.top ||
				candidate.y > popoverRectangle.bottom
		);
		if (!point) throw new Error('Could not find a point outside the draft popover');
		return point;
	});
	await page.mouse.click(clickPoint.x, clickPoint.y);
}

async function monthDayCellBox(
	page: Page,
	dateKey: string
): Promise<{ x: number; y: number; width: number; height: number }> {
	const cell = page.locator(`[data-calendar-date="${dateKey}"]`);
	await cell.scrollIntoViewIfNeeded();
	const box = await cell.boundingBox();
	if (!box) throw new Error(`month cell must be visible before dragging: ${dateKey}`);
	return box;
}

export async function dragBetweenCells(page: Page, startDateKey: string, endDateKey: string): Promise<void> {
	const startBox = await monthDayCellBox(page, startDateKey);
	const endBox = await monthDayCellBox(page, endDateKey);
	await page.mouse.move(startBox.x + startBox.width / 2, startBox.y + startBox.height / 2);
	await page.mouse.down();
	await page.mouse.move(endBox.x + endBox.width / 2, endBox.y + endBox.height / 2, { steps: 8 });
	await page.mouse.up();
}

export async function startDragBetweenCells(page: Page, startDateKey: string, endDateKey: string): Promise<void> {
	const startBox = await monthDayCellBox(page, startDateKey);
	const endBox = await monthDayCellBox(page, endDateKey);
	await page.mouse.move(startBox.x + startBox.width / 2, startBox.y + startBox.height / 2);
	await page.mouse.down();
	await page.mouse.move(endBox.x + endBox.width / 2, endBox.y + endBox.height / 2, { steps: 8 });
}

export async function cancelMonthRangeDrag(page: Page): Promise<void> {
	await page.evaluate(() => {
		const scrollElement = document.querySelector('[role="grid"]');
		if (!(scrollElement instanceof HTMLElement)) throw new Error('missing month scroll element');
		scrollElement.dispatchEvent(new PointerEvent('pointercancel', { bubbles: true, cancelable: true, pointerId: 1 }));
	});
}

export async function scrollMonthViewBy(page: Page, deltaY: number): Promise<void> {
	await page.locator('[role="grid"]').evaluate((element, deltaY) => {
		element.dispatchEvent(new WheelEvent('wheel', { deltaY, bubbles: true, cancelable: true }));
		element.scrollTop += deltaY;
		element.dispatchEvent(new Event('scroll', { bubbles: true }));
	}, deltaY);
	await page.evaluate(
		() =>
			new Promise<void>((resolve) => {
				requestAnimationFrame(() => requestAnimationFrame(() => resolve()));
			})
	);
}
