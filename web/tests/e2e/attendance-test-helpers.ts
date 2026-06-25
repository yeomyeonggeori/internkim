import type { Page } from '@playwright/test';
import { buildAttendanceSummaryFixture } from '../../dev-attendance-summary-fixture';
import type { UpdateAttendanceEventRequest } from '../../src/routes/attendance/attendance-api';
import type {
	AttendanceEvent,
	AttendanceSummary
} from '../../src/routes/attendance/attendance-context.svelte';

export async function selectKorean(page: Page): Promise<void> {
	await page.getByRole('button', { name: /Change language|언어 변경/ }).click();
	await page.getByRole('menuitemradio', { name: '한국어' }).click();
}

export function todayDateInSeoul(): string {
	const parts = new Intl.DateTimeFormat('en-US', {
		timeZone: 'Asia/Seoul',
		year: 'numeric',
		month: '2-digit',
		day: '2-digit'
	}).formatToParts(new Date());
	const dateParts = Object.fromEntries(parts.map((part) => [part.type, part.value]));
	return `${dateParts.year}-${dateParts.month}-${dateParts.day}`;
}

export function currentUserEvent(
	summary: AttendanceSummary,
	localDate: string,
	kind: AttendanceEvent['kind']
): AttendanceEvent | undefined {
	return summary.events.find(
		(event) => event.email === summary.currentUserEmail && event.localDate === localDate && event.kind === kind
	);
}

export function buildOvernightAttendanceSummary(month: string): AttendanceSummary {
	const summary = buildAttendanceSummaryFixture(month);
	return {
		...summary,
		events: [
			attendanceEvent(summary, {
				id: 'overnight-in',
				kind: 'clock_in',
				occurredAt: '2026-06-01T22:00:00+09:00',
				localDate: '2026-06-01',
				localTime: '22:00:00',
			}),
			attendanceEvent(summary, {
				id: 'overnight-out',
				kind: 'clock_out',
				occurredAt: '2026-06-02T02:00:00+09:00',
				localDate: '2026-06-02',
				localTime: '02:00:00',
			}),
		],
		absences: []
	};
}

export function buildOpenOvernightAttendanceSummary(todayDate: string, previousDate: string): AttendanceSummary {
	const summary = buildAttendanceSummaryFixture(todayDate.slice(0, 7));
	return {
		...summary,
		events: [
			attendanceEvent(summary, {
				id: 'open-overnight-in',
				kind: 'clock_in',
				occurredAt: `${previousDate}T22:00:00+09:00`,
				localDate: previousDate,
				localTime: '22:00:00',
			}),
		],
		absences: []
	};
}

export function buildWideTooltipSummary(month: string, todayDate: string): AttendanceSummary {
	const summary = buildAttendanceSummaryFixture(month);
	if (month !== todayDate.slice(0, 7)) return summary;
	const times = ['09:46:54', '10:46:54', '10:46:54', '19:35:48', '19:36:00'];
	let index = 0;
	return {
		...summary,
		locations: summary.locations.map((location) => {
			if (location.id !== 'office') return location;
			return { ...location, name: '사무실본관회의실A' };
		}),
		events: summary.events.map((event) => {
			if (event.email !== 'kim@example.com' || event.localDate !== todayDate) return event;
			const localTime = times[index] ?? event.localTime;
			index += 1;
			return {
				...event,
				occurredAt: `${todayDate}T${localTime}+09:00`,
				localTime,
				locationID: 'office',
				locationName: '사무실본관회의실A'
			};
		})
	};
}

export function parseUpdateAttendanceEventRequest(payload: string | null): UpdateAttendanceEventRequest {
	const parsed = JSON.parse(payload ?? '{}') as Partial<UpdateAttendanceEventRequest>;
	if (
		typeof parsed.localDate !== 'string' ||
		typeof parsed.localTime !== 'string' ||
		typeof parsed.locationID !== 'string' ||
		typeof parsed.reason !== 'string'
	) {
		throw new Error('Invalid update attendance event request');
	}
	return {
		localDate: parsed.localDate,
		localTime: parsed.localTime,
		locationID: parsed.locationID,
		reason: parsed.reason
	};
}

export function replaceSummaryEvent(
	summary: AttendanceSummary,
	eventID: string,
	request: UpdateAttendanceEventRequest
): AttendanceSummary {
	return {
		...summary,
		events: summary.events.map((event) => (event.id === eventID ? overrideEvent(summary, event, request) : event))
	};
}

function attendanceEvent(
	summary: AttendanceSummary,
	overrides: Pick<AttendanceEvent, 'id' | 'kind' | 'occurredAt' | 'localDate' | 'localTime'>
): AttendanceEvent {
	return {
		id: overrides.id,
		mattermostUserID: 'kim',
		mattermostUsername: 'kim',
		email: summary.currentUserEmail,
		displayName: '김철수',
		kind: overrides.kind,
		occurredAt: overrides.occurredAt,
		localDate: overrides.localDate,
		localTime: overrides.localTime,
		timeZoneAtEvent: summary.timeZone,
		source: 'test',
		resultPostID: `${overrides.id}-post`,
		locationID: 'office',
		locationName: '사무실'
	};
}

function overrideEvent(
	summary: AttendanceSummary,
	event: AttendanceEvent,
	request: UpdateAttendanceEventRequest
): AttendanceEvent {
	const location = summary.locations.find((candidate) => candidate.id === request.locationID);
	const locationName = location?.name ?? request.locationID;
	const editedAt = `${request.localDate}T17:20:00+09:00`;
	return {
		...event,
		occurredAt: `${request.localDate}T${request.localTime}:00+09:00`,
		localDate: request.localDate,
		localTime: request.localTime,
		locationID: request.locationID,
		locationName,
		overriddenBy: summary.currentUserEmail,
		overriddenAt: editedAt,
		overrideReason: request.reason,
		originalOccurredAt: event.occurredAt,
		originalLocalDate: event.localDate,
		originalLocalTime: event.localTime,
		originalLocationID: event.locationID,
		originalLocationName: event.locationName,
		overrideHistory: [
			{
				id: `override-${event.id}`,
				eventID: event.id,
				editedBy: summary.currentUserEmail,
				editedAt,
				reason: request.reason,
				originalOccurredAt: event.occurredAt,
				originalLocalDate: event.localDate,
				originalLocalTime: event.localTime,
				originalLocationID: event.locationID ?? '',
				originalLocationName: event.locationName ?? '',
				overrideOccurredAt: `${request.localDate}T${request.localTime}:00+09:00`,
				overrideLocalDate: request.localDate,
				overrideLocalTime: request.localTime,
				overrideLocationID: request.locationID,
				overrideLocationName: locationName
			},
			...(event.overrideHistory ?? [])
		]
	};
}
