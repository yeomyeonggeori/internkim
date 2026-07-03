import type { Page } from '@playwright/test';

export type CalendarTestLocale = 'ko' | 'en';

export type CalendarTestParticipant = {
	personID: string;
	name: string;
	email?: string;
	image?: string;
};

export type CalendarTestEvent = {
	id: string;
	title: string;
	startISO: string;
	endISO: string;
	isAllDay: boolean;
	participants?: CalendarTestParticipant[];
	updatedAt?: string;
};

export type CalendarEventUpdatePayload = {
	eventID: string;
	title: string;
	description: string;
	location: string;
	startISO: string;
	endISO: string;
	timeZone: string;
	isAllDay: boolean;
	color: string;
	participants: CalendarTestParticipant[];
};

type CalendarTestAccountStatus = {
	connected: boolean;
	needsReauth: boolean;
	googleOAuthConfigured: boolean;
	canManageGoogleOAuth: boolean;
};

export async function routeDefaultCalendarAPI(page: Page, locale: CalendarTestLocale = 'ko'): Promise<void> {
	await page.route('**/calendar/api/events?**', async (route) => {
		await route.fulfill({
			json: {
				events: [
					{
						id: 'existing-overlap-event',
						title: '겹치는 일정',
						startISO: '2026-06-08T01:00:00+09:00',
						endISO: '2026-06-08T02:00:00+09:00',
						isAllDay: false
					}
				]
			}
		});
	});
	await routeCalendarBackgroundAPI(page);
	await routeCalendarParticipants(page, []);
	await routeCalendarLocale(page, locale);
}

export async function routeCalendarBackgroundAPI(
	page: Page,
	accountStatus: CalendarTestAccountStatus = {
		connected: false,
		needsReauth: false,
		googleOAuthConfigured: true,
		canManageGoogleOAuth: false
	}
): Promise<void> {
	await page.route('**/calendar/api/account-status', async (route) => {
		await route.fulfill({ json: accountStatus });
	});
	await page.route('**/calendar/api/remote-sync', async (route) => {
		await route.fulfill({ json: { synced: false } });
	});
	await page.route('**/calendar/api/conflicts', async (route) => {
		await route.fulfill({ json: { conflicts: [] } });
	});
	await page.route('**/calendar/api/conflicts/*', async (route) => {
		await route.fulfill({ json: {} });
	});
}

export async function routeCalendarLocale(page: Page, locale: CalendarTestLocale): Promise<void> {
	await page.unroute('**/admin/api/locale');
	await page.route('**/admin/api/locale', async (route) => {
		await route.fulfill({ json: { locale } });
	});
}

export async function routeCalendarParticipants(page: Page, participants: CalendarTestParticipant[]): Promise<void> {
	await page.unroute('**/calendar/api/participants');
	await page.unroute('**/calendar/api/participants/*/image');
	await page.route('**/calendar/api/participants', async (route) => {
		await route.fulfill({ json: { participants } });
	});
	await page.route('**/calendar/api/participants/*/image', async (route) => {
		await route.fulfill({ status: 204, body: '' });
	});
}

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
					participants: event.participants ?? [],
					updatedAt: event.updatedAt ?? '2026-06-08T00:00:00Z'
				}))
			}
		});
	});
}

export async function routeCalendarEventUpdates(page: Page): Promise<CalendarEventUpdatePayload[]> {
	const updatedEvents: CalendarEventUpdatePayload[] = [];
	await page.route('**/calendar/api/events/*', async (route) => {
		if (route.request().method() !== 'PUT') {
			await route.fulfill({ json: {} });
			return;
		}
		const payload = calendarEventUpdatePayloadFromRequestData(route.request().postData());
		updatedEvents.push(payload);
		await route.fulfill({
			json: {
				id: payload.eventID,
				uid: payload.eventID,
				title: payload.title,
				description: payload.description,
				location: payload.location,
				startISO: payload.startISO,
				endISO: payload.endISO,
				timeZone: payload.timeZone,
				isAllDay: payload.isAllDay,
				color: payload.color,
				participants: payload.participants,
				createdByEmail: 'test@example.com',
				createdByName: 'Test User',
				updatedAt: '2026-06-08T00:00:00Z'
			}
		});
	});
	return updatedEvents;
}

export async function routeCalendarEventCreates(page: Page): Promise<CalendarEventUpdatePayload[]> {
	const createdEvents: CalendarEventUpdatePayload[] = [];
	await page.route('**/calendar/api/events', async (route) => {
		if (route.request().method() !== 'POST') {
			await route.fallback();
			return;
		}
		const payload = calendarEventUpdatePayloadFromRequestData(route.request().postData());
		createdEvents.push(payload);
		await route.fulfill({
			json: {
				id: payload.eventID,
				uid: payload.eventID,
				title: payload.title,
				description: payload.description,
				location: payload.location,
				startISO: payload.startISO,
				endISO: payload.endISO,
				timeZone: payload.timeZone,
				isAllDay: payload.isAllDay,
				color: payload.color,
				participants: payload.participants,
				createdByEmail: 'test@example.com',
				createdByName: 'Test User',
				updatedAt: '2026-06-08T00:00:00Z'
			}
		});
	});
	return createdEvents;
}

export async function routeCalendarEventDeletes(page: Page): Promise<string[]> {
	return routeCalendarEventDeletesWithFailures(page, []);
}

export async function routeCalendarEventDeletesWithFailures(page: Page, failingEventIDs: string[]): Promise<string[]> {
	const deletedEventIDs: string[] = [];
	const failingEventIDSet = new Set(failingEventIDs);
	await page.route('**/calendar/api/events/*', async (route) => {
		if (route.request().method() !== 'DELETE') {
			await route.fulfill({ json: {} });
			return;
		}
		const eventID = decodeURIComponent(route.request().url().split('/').pop() ?? '');
		deletedEventIDs.push(eventID);
		if (failingEventIDSet.has(eventID)) {
			failingEventIDSet.delete(eventID);
			await route.fulfill({ status: 500, body: 'delete failed' });
			return;
		}
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

export function dateTimeKeyInSeoul(isoDate: string): string {
	const date = new Date(isoDate);
	if (Number.isNaN(date.getTime())) throw new Error(`Invalid ISO date: ${isoDate}`);
	const parts = new Intl.DateTimeFormat('en-CA', {
		timeZone: 'Asia/Seoul',
		year: 'numeric',
		month: '2-digit',
		day: '2-digit',
		hour: '2-digit',
		minute: '2-digit',
		hourCycle: 'h23'
	}).formatToParts(date);
	const values = new Map(parts.map((part) => [part.type, part.value]));
	return `${values.get('year')}-${values.get('month')}-${values.get('day')} ${values.get('hour')}:${values.get('minute')}`;
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

function calendarEventUpdatePayloadFromRequestData(requestData: string | null): CalendarEventUpdatePayload {
	if (!requestData) throw new Error('Missing calendar event update request body');
	const payload: unknown = JSON.parse(requestData);
	if (!isCalendarEventUpdatePayload(payload)) throw new Error('Invalid calendar event update request body');
	return payload;
}

function isCalendarEventUpdatePayload(payload: unknown): payload is CalendarEventUpdatePayload {
	if (!isStringRecord(payload)) return false;
	return (
		typeof payload.eventID === 'string' &&
		typeof payload.title === 'string' &&
		typeof payload.description === 'string' &&
		typeof payload.location === 'string' &&
		typeof payload.startISO === 'string' &&
		typeof payload.endISO === 'string' &&
		typeof payload.timeZone === 'string' &&
		typeof payload.isAllDay === 'boolean' &&
		typeof payload.color === 'string' &&
		Array.isArray(payload.participants) &&
		payload.participants.every(isCalendarTestParticipant)
	);
}

function isStringRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null;
}

function isCalendarTestParticipant(value: unknown): value is CalendarTestParticipant {
	if (!isStringRecord(value)) return false;
	const hasValidEmail = value.email === undefined || typeof value.email === 'string';
	const hasValidImage = value.image === undefined || typeof value.image === 'string';
	return (
		typeof value.personID === 'string' &&
		typeof value.name === 'string' &&
		hasValidEmail &&
		hasValidImage
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
