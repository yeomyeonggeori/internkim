import type { IncomingMessage, ServerResponse } from 'node:http';
import type { Plugin } from 'vite';
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
		previewCalendarEvent('attendance-preview-sync', '팀 진행 상황 공유', date, '16:30', '17:00', '회의실 B', userEmail)
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
