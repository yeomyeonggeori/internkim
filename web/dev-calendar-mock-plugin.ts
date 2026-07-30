import type { IncomingMessage, ServerResponse } from 'node:http';
import type { Plugin } from 'vite';
import { parseJSONRecord, readRequestBody } from './dev-admin-mock';
import { devAttendancePeople } from './dev-attendance-fixture-data';
import {
	devPopupOverflowCalendarEvents,
	devPopupOverflowDate,
	devPopupOverflowEmail,
	type DevPopupOverflowCalendarEvent
} from './dev-popup-overflow-fixture';
import { todayDateInTimeZone } from './src/routes/attendance/shared/attendance-date';
import type { CalendarEvent } from './src/routes/calendar/embed/calendar-event-persistence';

type DevCalendarMockPluginOptions = {
	isEnabled: boolean;
	userEmail: string;
};

type DevCalendarMockRequest = {
	method: string;
	pathname: string;
	searchParams: URLSearchParams;
	body?: string;
};

type DevCalendarMockResponse = {
	status: number;
	body: {
		events?: CalendarEvent[];
		authenticated?: boolean;
		email?: string;
		isAdmin?: boolean;
		locale?: 'ko' | 'en';
		[key: string]: unknown;
	};
};

export type DevCalendarMockState = {
	userEmail: string;
	locale: 'ko' | 'en';
	events: CalendarEvent[];
	deleteIntents: Map<string, number>;
};

export function devCalendarMockPlugin(options: DevCalendarMockPluginOptions): Plugin {
	const state = createDevCalendarMockState(options.userEmail);
	return {
		name: 'internkim-dev-calendar-mock',
		configureServer(server) {
			if (!options.isEnabled) return;

			server.middlewares.use((request, response, next) => {
				const requestURL = new URL(request.url ?? '/', 'http://localhost');
				const method = request.method ?? 'GET';
				if (!shouldHandleDevCalendarMockRequest(method, requestURL.pathname)) {
					next();
					return;
				}
				readRequestBody(request, (body) => {
					const mockResponse = createDevCalendarMockResponse(state, {
						method,
						pathname: requestURL.pathname,
						searchParams: requestURL.searchParams,
						body
					});
					if (!mockResponse) {
						next();
						return;
					}
					writeJSON(response, mockResponse.status, mockResponse.body);
				});
			});
		}
	};
}

export function createDevCalendarMockState(userEmail: string): DevCalendarMockState {
	return { userEmail, locale: 'ko', events: createDevCalendarEvents(userEmail), deleteIntents: new Map() };
}

export function createDevCalendarMockResponse(
	state: DevCalendarMockState,
	request: DevCalendarMockRequest
): DevCalendarMockResponse | undefined {
	if (request.method === 'GET' && request.pathname === '/auth/session') {
		return { status: 200, body: { authenticated: true, email: state.userEmail, isAdmin: true } };
	}
	if (request.method === 'GET' && request.pathname === '/admin/api/session') {
		return { status: 200, body: { authenticated: true, email: state.userEmail, isAdmin: true } };
	}
	if (request.method === 'GET' && request.pathname === '/admin/api/locale') {
		return { status: 200, body: { locale: state.locale } };
	}
	if (request.method === 'PUT' && request.pathname === '/admin/api/locale') {
		state.locale = parseJSONRecord(request.body).locale === 'en' ? 'en' : 'ko';
		return { status: 200, body: { locale: state.locale } };
	}
	if (request.method === 'GET' && request.pathname === '/calendar/api/sync') {
		return { status: 200, body: { caldavURL: '', caldavUsername: '', caldavPassword: '', icsURL: '' } };
	}
	if (request.method === 'GET' && request.pathname === '/calendar/api/account-status') {
		return { status: 200, body: { connected: false, needsReauth: false, googleOAuthConfigured: true, canManageGoogleOAuth: false } };
	}
	if (request.method === 'GET' && request.pathname === '/calendar/api/participants') {
		return { status: 200, body: { participants: devCalendarParticipants() } };
	}
	if (request.method === 'POST' && request.pathname === '/calendar/api/remote-sync') {
		return { status: 200, body: { synced: false } };
	}
	if (request.method === 'GET' && request.pathname === '/calendar/api/conflicts') {
		return { status: 200, body: { conflicts: [] } };
	}
	const deleteIntentEventID = deleteIntentEventIDFromPath(request.pathname);
	if (deleteIntentEventID) {
		if (request.method === 'PUT') {
			const intent = deleteIntentResponse(request.pathname);
			state.deleteIntents.set(deleteIntentEventID, new Date(intent.executeAt).getTime());
			return { status: 200, body: intent };
		}
		if (request.method === 'DELETE') {
			state.deleteIntents.delete(deleteIntentEventID);
			return { status: 200, body: {} };
		}
	}
	const eventID = eventIDFromPath(request.pathname);
	if (eventID && request.method === 'PUT') {
		const updatedEvent = storeDevCalendarEvent(state, eventID, request.body);
		return { status: 200, body: { ...updatedEvent } };
	}
	if (eventID && request.method === 'DELETE') {
		state.events = state.events.filter((event) => event.id !== eventID);
		return { status: 200, body: {} };
	}
	if (request.method === 'POST' && request.pathname === '/calendar/api/events') {
		const createdEvent = storeDevCalendarEvent(state, parseJSONRecord(request.body).eventID, request.body);
		return { status: 200, body: { ...createdEvent } };
	}
	if (request.method !== 'GET' || request.pathname !== '/calendar/api/events') return undefined;
	const startTime = timestampFromISO(request.searchParams.get('startISO'), Number.NEGATIVE_INFINITY);
	const endTime = timestampFromISO(request.searchParams.get('endISO'), Number.POSITIVE_INFINITY);
	applyDueDevCalendarDeleteIntents(state);
	const events = state.events.filter((event) => {
		const eventStartTime = new Date(event.startISO).getTime();
		return eventStartTime >= startTime && eventStartTime <= endTime;
	});
	return { status: 200, body: { events } };
}

function applyDueDevCalendarDeleteIntents(state: DevCalendarMockState): void {
	const now = Date.now();
	for (const [eventID, executeTime] of state.deleteIntents) {
		if (executeTime > now) continue;
		state.events = state.events.filter((event) => event.id !== eventID);
		state.deleteIntents.delete(eventID);
	}
}

function eventIDFromPath(pathname: string): string | undefined {
	const match = /^\/calendar\/api\/events\/([^/]+)$/.exec(pathname);
	return match ? decodeURIComponent(match[1]) : undefined;
}

function deleteIntentEventIDFromPath(pathname: string): string | undefined {
	const match = /^\/calendar\/api\/events\/([^/]+)\/delete-intents\/[^/]+$/.exec(pathname);
	return match ? decodeURIComponent(match[1]) : undefined;
}

function deleteIntentResponse(pathname: string): { operationID: string; executeAt: string } {
	const operationID = decodeURIComponent(pathname.slice(pathname.lastIndexOf('/') + 1));
	return { operationID, executeAt: new Date(Date.now() + 5000).toISOString() };
}

function storeDevCalendarEvent(state: DevCalendarMockState, eventID: unknown, body: string | undefined): CalendarEvent {
	const payload = parseJSONRecord(body);
	const id = typeof eventID === 'string' && eventID ? eventID : `dev-${Date.now()}`;
	const existingEvent = state.events.find((event) => event.id === id);
	const savedEvent: CalendarEvent = {
		...(existingEvent ?? emptyDevCalendarEvent(id, state.userEmail)),
		id,
		uid: id,
		title: textField(payload.title),
		description: textField(payload.description),
		location: textField(payload.location),
		startISO: textField(payload.startISO),
		endISO: textField(payload.endISO),
		timeZone: textField(payload.timeZone) || 'Asia/Seoul',
		isAllDay: payload.isAllDay === true,
		color: textField(payload.color) || '#2563eb',
		participants: Array.isArray(payload.participants) ? (payload.participants as CalendarEvent['participants']) : [],
		updatedAt: new Date().toISOString()
	};
	state.events = [...state.events.filter((event) => event.id !== id), savedEvent];
	return savedEvent;
}

function emptyDevCalendarEvent(id: string, userEmail: string): CalendarEvent {
	return {
		id,
		uid: id,
		title: '',
		description: '',
		location: '',
		startISO: '',
		endISO: '',
		timeZone: 'Asia/Seoul',
		isAllDay: false,
		color: '#2563eb',
		participants: [],
		createdByEmail: userEmail,
		createdByName: userEmail,
		updatedAt: new Date().toISOString()
	};
}

function textField(value: unknown): string {
	return typeof value === 'string' ? value : '';
}

function shouldHandleDevCalendarMockRequest(method: string, pathname: string): boolean {
	if (pathname.startsWith('/calendar/api/events/')) return true;
	if (method === 'POST' && pathname === '/calendar/api/events') return true;
	if (method === 'GET' && pathname === '/auth/session') return true;
	if (method === 'GET' && pathname === '/admin/api/session') return true;
	if ((method === 'GET' || method === 'PUT') && pathname === '/admin/api/locale') return true;
	if (method === 'GET' && pathname === '/calendar/api/sync') return true;
	if (method === 'GET' && pathname === '/calendar/api/account-status') return true;
	if (method === 'GET' && pathname === '/calendar/api/participants') return true;
	if (method === 'POST' && pathname === '/calendar/api/remote-sync') return true;
	if (method === 'GET' && pathname === '/calendar/api/conflicts') return true;
	return method === 'GET' && pathname === '/calendar/api/events';
}

function devCalendarParticipants(): { personID: string; name: string; email: string }[] {
	return devAttendancePeople.map((person) => ({
		personID: person.mattermostUsername,
		name: person.name,
		email: person.email
	}));
}

function createDevCalendarEvents(userEmail: string): CalendarEvent[] {
	return [
		...devPopupOverflowCalendarEvents.map((event) => calendarEvent(event, userEmail)),
		...createAttendancePreviewCalendarEvents(userEmail)
	];
}

function createAttendancePreviewCalendarEvents(userEmail: string): CalendarEvent[] {
	const date = todayDateInTimeZone('Asia/Seoul', new Date());
	return [
		previewCalendarEvent('attendance-preview-standup', '오늘의 우선순위 정렬', date, '09:30', '10:00', '회의실 A', userEmail),
		previewCalendarEvent('attendance-preview-review', '근태 상세 화면 UI 리뷰', date, '14:00', '15:00', '디자인 룸', userEmail),
		previewCalendarEvent('attendance-preview-sync', '팀 진행 상황 공유', date, '16:30', '17:00', '회의실 B', userEmail),
		{
			...previewCalendarEvent('today-all-day-preview', '오늘 종일 일정', date, '00:00', '23:59', '', userEmail),
			isAllDay: true
		}
	];
}

function calendarEvent(event: DevPopupOverflowCalendarEvent, userEmail: string): CalendarEvent {
	return previewCalendarEvent(event.id, event.title, devPopupOverflowDate, event.startTime, event.endTime, event.location, userEmail);
}

function previewCalendarEvent(
	id: string,
	title: string,
	date: string,
	startTime: string,
	endTime: string,
	location: string,
	userEmail: string
): CalendarEvent {
	const person = devAttendancePeople.find((candidate) => candidate.email === userEmail) ??
		devAttendancePeople.find((candidate) => candidate.email === devPopupOverflowEmail) ??
		devAttendancePeople[0];
	const email = person?.email ?? userEmail;
	const name = person?.name ?? email;
	return {
		id,
		uid: id,
		title,
		description: '',
		location,
		startISO: `${date}T${startTime}:00+09:00`,
		endISO: `${date}T${endTime}:00+09:00`,
		timeZone: 'Asia/Seoul',
		isAllDay: false,
		color: '#2563eb',
		participants: [{ personID: person?.mattermostUsername ?? email, name, email }],
		createdByEmail: email,
		createdByName: name,
		updatedAt: `${date}T00:00:00+09:00`
	};
}

function timestampFromISO(value: string | null, fallback: number): number {
	if (!value) return fallback;
	const timestamp = new Date(value).getTime();
	return Number.isFinite(timestamp) ? timestamp : fallback;
}

function writeJSON(response: ServerResponse<IncomingMessage>, status: number, body: unknown): void {
	response.statusCode = status;
	response.setHeader('Content-Type', 'application/json');
	response.end(JSON.stringify(body));
}
