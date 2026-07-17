import type { Page, Route } from '@playwright/test';
import { routeCalendarParticipants } from './calendar-embed-test-utils';

export type CalendarDraftPopoverEvent = {
	id: string;
	title: string;
	description: string;
	location: string;
	startISO: string;
	endISO: string;
	timeZone: string;
	isAllDay: boolean;
	color: string;
	updatedAt: string;
};

export async function routeCalendarAPI(page: Page): Promise<void> {
	await page.route('**/admin/api/locale', async (route) => {
		await route.fulfill({ json: { locale: 'ko' } });
	});
	await page.route('**/calendar/api/events?**', async (route) => {
		await route.fulfill({ json: { events: [] } });
	});
	await routeCalendarParticipants(page, []);
	await page.route('**/calendar/api/sync', async (route) => {
		await route.fulfill({
			json: {
				caldavURL: '',
				caldavUsername: '',
				caldavPassword: '',
				icsURL: ''
			}
		});
	});
	await page.route('**/calendar/api/account-status', async (route) => {
		await route.fulfill({
			json: {
				connected: false,
				needsReauth: false,
				googleOAuthConfigured: true,
				canManageGoogleOAuth: false
			}
		});
	});
	await page.route('**/calendar/api/remote-sync', async (route) => {
		await route.fulfill({ json: { synced: false } });
	});
	await page.route('**/calendar/api/conflicts', async (route) => {
		await route.fulfill({ json: { conflicts: [] } });
	});
	await page.route('**/calendar/api/conflicts/*', async (route) => {
		await fulfillEmptyResponse(route);
	});
}

export function draftPopoverEvent(overrides: Partial<CalendarDraftPopoverEvent>): CalendarDraftPopoverEvent {
	return {
		id: 'draft-popover-event',
		title: '일정',
		description: '',
		location: '',
		startISO: '2026-06-17T00:00:00.000Z',
		endISO: '2026-06-18T00:00:00.000Z',
		timeZone: 'Asia/Seoul',
		isAllDay: true,
		color: '#1677ff',
		updatedAt: '2026-06-10T11:30:00.000Z',
		...overrides
	};
}

export async function routeDraftPopoverEvents(page: Page, events: CalendarDraftPopoverEvent[]): Promise<void> {
	await page.unroute('**/calendar/api/events?**');
	await page.route('**/calendar/api/events?**', async (route) => {
		await route.fulfill({ json: { events } });
	});
}

export async function routeDraftPopoverEventUpdate(
	page: Page,
	event: CalendarDraftPopoverEvent
): Promise<Record<string, unknown>[]> {
	const updatedPayloads: Record<string, unknown>[] = [];
	await page.route(`**/calendar/api/events/${event.id}`, async (route) => {
		if (route.request().method() !== 'PUT') {
			await route.fulfill({ json: {} });
			return;
		}

		const payload = route.request().postDataJSON() as Record<string, unknown>;
		updatedPayloads.push(payload);
		await route.fulfill({
			json: {
				...event,
				title: String(payload.title),
				description: String(payload.description ?? ''),
				location: String(payload.location ?? ''),
				startISO: String(payload.startISO),
				endISO: String(payload.endISO),
				isAllDay: Boolean(payload.isAllDay),
				updatedAt: '2026-06-10T12:00:00.000Z'
			}
		});
	});
	return updatedPayloads;
}

export async function waitForClientHydration(page: Page): Promise<void> {
	await page.waitForTimeout(1_000);
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

export async function dragBetweenCells(page: Page, startDateKey: string, endDateKey: string): Promise<void> {
	const startBox = await page.locator(`.df-month-day-cell[data-date="${startDateKey}"]`).boundingBox();
	const endBox = await page.locator(`.df-month-day-cell[data-date="${endDateKey}"]`).boundingBox();
	if (!startBox || !endBox) throw new Error('month cells must be visible before dragging');
	await page.mouse.move(startBox.x + startBox.width / 2, startBox.y + startBox.height / 2);
	await page.mouse.down();
	await page.mouse.move(endBox.x + endBox.width / 2, endBox.y + endBox.height / 2, { steps: 8 });
	await page.mouse.up();
}

export async function startDragBetweenCells(page: Page, startDateKey: string, endDateKey: string): Promise<void> {
	const startBox = await page.locator(`.df-month-day-cell[data-date="${startDateKey}"]`).boundingBox();
	const endBox = await page.locator(`.df-month-day-cell[data-date="${endDateKey}"]`).boundingBox();
	if (!startBox || !endBox) throw new Error('month cells must be visible before dragging');
	await page.mouse.move(startBox.x + startBox.width / 2, startBox.y + startBox.height / 2);
	await page.mouse.down();
	await page.mouse.move(endBox.x + endBox.width / 2, endBox.y + endBox.height / 2, { steps: 8 });
}

export async function scrollMonthViewBy(page: Page, deltaY: number): Promise<void> {
	await page.locator('.df-month-view-virtual-scroller').evaluate((element, deltaY) => {
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

export async function monthPopoverAnchorGeometry(
	page: Page,
	eventID: string
): Promise<{ arrowY: number; titleCenterY: number; popoverTop: number } | null> {
	return page.evaluate((eventID) => {
		const eventElement = document.querySelector<HTMLElement>(
			`.calendar-month-direct-event[data-event-id="${CSS.escape(eventID)}"]`
		);
		const titleElement = eventElement?.querySelector<HTMLElement>('.calendar-month-event-title, .calendar-event-title');
		const popover = document.querySelector<HTMLElement>('.calendar-draft-popover');
		if (!eventElement || !titleElement || !popover) return null;
		const titleRectangle = titleElement.getBoundingClientRect();
		const popoverRectangle = popover.getBoundingClientRect();
		const arrowTop = Number.parseFloat(window.getComputedStyle(popover, '::before').top);
		return {
			arrowY: popoverRectangle.top + arrowTop + 8,
			titleCenterY: titleRectangle.top + titleRectangle.height / 2,
			popoverTop: popoverRectangle.top
		};
	}, eventID);
}

export async function draftPopoverMotionStyle(page: Page): Promise<{ top: string; transform: string }> {
	return page.evaluate(() => {
		const popover = document.querySelector<HTMLElement>('.calendar-draft-popover');
		if (!popover) return { top: '', transform: '' };
		return {
			top: popover.style.top,
			transform: popover.style.transform
		};
	});
}

export async function dispatchMonthRangePointerDrag(
	page: Page,
	startDateKey: string,
	endDateKey: string,
	finishWith: 'cancel' | 'up'
): Promise<void> {
	await page.evaluate(
		({ startDateKey, endDateKey, finishWith }) => {
			const startCell = document.querySelector(`.df-month-day-cell[data-date="${startDateKey}"]`);
			const endCell = document.querySelector(`.df-month-day-cell[data-date="${endDateKey}"]`);
			if (!(startCell instanceof HTMLElement) || !(endCell instanceof HTMLElement)) {
				throw new Error('month cells must be visible before dragging');
			}
			const startRectangle = startCell.getBoundingClientRect();
			const endRectangle = endCell.getBoundingClientRect();
			const pointerID = 707;
			const startClientX = startRectangle.left + startRectangle.width / 2;
			const startClientY = startRectangle.top + startRectangle.height / 2;
			const endClientX = endRectangle.left + endRectangle.width / 2;
			const endClientY = endRectangle.top + endRectangle.height / 2;
			startCell.dispatchEvent(
				new PointerEvent('pointerdown', {
					bubbles: true,
					cancelable: true,
					button: 0,
					pointerId: pointerID,
					clientX: startClientX,
					clientY: startClientY
				})
			);
			document.dispatchEvent(
				new PointerEvent('pointermove', {
					bubbles: true,
					cancelable: true,
					button: 0,
					pointerId: pointerID,
					clientX: endClientX,
					clientY: endClientY
				})
			);
			document.dispatchEvent(
				new PointerEvent(finishWith === 'cancel' ? 'pointercancel' : 'pointerup', {
					bubbles: true,
					cancelable: true,
					button: 0,
					pointerId: pointerID,
					clientX: endClientX,
					clientY: endClientY
				})
			);
		},
		{ startDateKey, endDateKey, finishWith }
	);
}

async function fulfillEmptyResponse(route: Route): Promise<void> {
	await route.fulfill({ json: {} });
}
