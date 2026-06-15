// 캘린더 embed e2e의 공통 route mock과 DOM 측정 helper를 제공합니다.
import type { Page } from '@playwright/test';

export type CalendarTestEvent = {
	id: string;
	title: string;
	startISO: string;
	endISO: string;
	isAllDay: boolean;
};

export async function routeCalendarEvents(page: Page, events: CalendarTestEvent[]): Promise<void> {
	await page.unroute('**/calendar/api/events?**');
	await page.route('**/calendar/api/events?**', async (route) => {
		await route.fulfill({
			json: {
				events: events.map((event) => ({
					id: event.id,
					uid: event.id,
					title: event.title,
					description: '',
					location: '',
					startISO: event.startISO,
					endISO: event.endISO,
					timeZone: 'Asia/Seoul',
					isAllDay: event.isAllDay,
					color: '#1677ff',
					createdByEmail: 'test@example.com',
					createdByName: 'Test User',
					updatedAt: '2026-06-08T00:00:00Z'
				}))
			}
		});
	});
}

export async function routeCalendarEventDeletes(page: Page): Promise<string[]> {
	const deletedEventIDs: string[] = [];
	await page.route('**/calendar/api/events/*', async (route) => {
		if (route.request().method() !== 'DELETE') {
			await route.fallback();
			return;
		}
		const eventID = decodeURIComponent(route.request().url().split('/').pop() ?? '');
		deletedEventIDs.push(eventID);
		await route.fulfill({ json: { ok: true } });
	});
	return deletedEventIDs;
}

export async function browserDateKey(page: Page): Promise<string> {
	return page.evaluate(() => {
		const date = new Date();
		return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
	});
}

export async function elementBox(page: Page, selector: string): Promise<{ top: number; bottom: number; height: number }> {
	return page.evaluate((targetSelector) => {
		const element = document.querySelector(targetSelector);
		if (!(element instanceof HTMLElement)) throw new Error(`Missing element: ${targetSelector}`);
		const rectangle = element.getBoundingClientRect();
		return {
			top: Math.round(rectangle.top),
			bottom: Math.round(rectangle.bottom),
			height: Math.round(rectangle.height)
		};
	}, selector);
}

export async function computedStyle(page: Page, selector: string, properties: string[]): Promise<Record<string, string>> {
	return page.evaluate(
		({ selector: targetSelector, properties: targetProperties }) => {
			const element = document.querySelector(targetSelector);
			if (!(element instanceof HTMLElement)) throw new Error(`Missing styled element: ${targetSelector}`);
			const style = window.getComputedStyle(element);
			return Object.fromEntries(targetProperties.map((property) => [property, style.getPropertyValue(property)]));
		},
		{ selector, properties }
	);
}

export async function computedPseudoStyle(
	page: Page,
	selector: string,
	pseudoElement: string,
	properties: string[]
): Promise<Record<string, string>> {
	return page.evaluate(
		({ selector: targetSelector, pseudoElement: targetPseudoElement, properties: targetProperties }) => {
			const element = document.querySelector(targetSelector);
			if (!(element instanceof HTMLElement)) throw new Error(`Missing styled element: ${targetSelector}`);
			const style = window.getComputedStyle(element, targetPseudoElement);
			return Object.fromEntries(targetProperties.map((property) => [property, style.getPropertyValue(property)]));
		},
		{ selector, pseudoElement, properties }
	);
}
