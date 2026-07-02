import type { IncomingMessage, ServerResponse } from 'node:http';
import type { Plugin } from 'vite';
import { devAttendancePeople } from './dev-attendance-fixture-data';
import {
	devPopupOverflowCalendarEvents,
	devPopupOverflowDate,
	devPopupOverflowEmail,
	type DevPopupOverflowCalendarEvent
} from './dev-popup-overflow-fixture';
import type { CalendarEvent } from './src/routes/calendar/embed/calendar-event-persistence';

type DevCalendarMockPluginOptions = {
	isEnabled: boolean;
	userEmail: string;
};

type DevCalendarMockRequest = {
	method: string;
	pathname: string;
	searchParams: URLSearchParams;
};

type DevCalendarMockResponse = {
	status: number;
	body: {
		events: CalendarEvent[];
	};
};

export function devCalendarMockPlugin(options: DevCalendarMockPluginOptions): Plugin {
	return {
		name: 'internkim-dev-calendar-mock',
		configureServer(server) {
			if (!options.isEnabled) return;

			server.middlewares.use((request, response, next) => {
				const requestURL = new URL(request.url ?? '/', 'http://localhost');
				const mockResponse = createDevCalendarMockResponse({
					method: request.method ?? 'GET',
					pathname: requestURL.pathname,
					searchParams: requestURL.searchParams
				}, options.userEmail);
				if (!mockResponse) {
					next();
					return;
				}
				writeJSON(response, mockResponse.status, mockResponse.body);
			});
		}
	};
}

export function createDevCalendarMockResponse(
	request: DevCalendarMockRequest,
	userEmail = 'kim@example.com'
): DevCalendarMockResponse | undefined {
	if (request.method !== 'GET' || request.pathname !== '/calendar/api/events') return undefined;
	const startTime = timestampFromISO(request.searchParams.get('startISO'), Number.NEGATIVE_INFINITY);
	const endTime = timestampFromISO(request.searchParams.get('endISO'), Number.POSITIVE_INFINITY);
	const events = createDevCalendarEvents(userEmail).filter((event) => {
		const eventStartTime = new Date(event.startISO).getTime();
		return eventStartTime >= startTime && eventStartTime <= endTime;
	});
	return { status: 200, body: { events } };
}

function createDevCalendarEvents(userEmail: string): CalendarEvent[] {
	return devPopupOverflowCalendarEvents.map((event) => calendarEvent(event, userEmail));
}

function calendarEvent(event: DevPopupOverflowCalendarEvent, userEmail: string): CalendarEvent {
	const person = devAttendancePeople.find((candidate) => candidate.email === userEmail) ??
		devAttendancePeople.find((candidate) => candidate.email === devPopupOverflowEmail) ??
		devAttendancePeople[0];
	const email = person?.email ?? userEmail;
	const name = person?.name ?? email;
	return {
		id: event.id,
		uid: event.id,
		title: event.title,
		description: '',
		location: event.location,
		startISO: `${devPopupOverflowDate}T${event.startTime}:00+09:00`,
		endISO: `${devPopupOverflowDate}T${event.endTime}:00+09:00`,
		timeZone: 'Asia/Seoul',
		isAllDay: false,
		color: '#2563eb',
		participants: [{ personID: person?.mattermostUsername ?? email, name, email }],
		createdByEmail: email,
		createdByName: name,
		updatedAt: `${devPopupOverflowDate}T00:00:00+09:00`
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
