import type { Plugin } from 'vite';
import type { IncomingMessage, ServerResponse } from 'node:http';
import type {
	AttendanceAbsence,
	AttendanceAbsenceKind,
	AttendanceSummary
} from './src/routes/attendance/attendance-context.svelte';
import { isWeekday, todayDateInTimeZone } from './src/routes/attendance/shared/attendance-date';
import { buildAttendanceSummaryFixture } from './dev-attendance-summary-fixture';
import {
	createDevEmployeeLeaveMockResponse,
	createDevEmployeeLeaveMockState,
	type DevEmployeeLeaveMockState
} from './dev-attendance-leave-mock';
import {
	createDevLeaveApprovalMockResponse,
	createDevLeaveApprovalMockState,
	type DevLeaveApprovalMockState
} from './dev-attendance-leave-approval-mock';
import {
	createDevLeaveManagementMockResponse,
	createDevLeaveManagementMockState,
	type DevLeaveManagementMockState
} from './dev-attendance-leave-management-mock';
import {
	createDevAttendanceLeavePolicyMockResponse,
	createDevAttendanceLeavePolicyMockState,
	type DevAttendanceLeavePolicyMockState
} from './dev-attendance-leave-policy-mock';
import {
	createDevAttendanceWorkPolicyMockResponse,
	createDevAttendanceWorkPolicyMockState,
	type DevAttendanceWorkPolicyMockState
} from './dev-attendance-work-policy-mock';
import { createDevAttendanceWorkStatus } from './dev-attendance-work-status-mock';
import { synchronizeUnlimitedEmployeeLeaveUsage } from './dev-attendance-leave-balance';

type DevAttendanceMockPluginOptions = {
	isEnabled: boolean;
	userEmail: string;
};

type DevAttendanceMockState = {
	userEmail: string;
	locale: 'ko' | 'en';
	createdAbsences: AttendanceAbsence[];
	canceledAbsenceIDs: Set<string>;
	eventOverrides: Record<string, DevAttendanceEventOverride[]>;
	nextAbsenceID: number;
	leave: DevEmployeeLeaveMockState;
	leaveApproval: DevLeaveApprovalMockState;
	leaveManagement: DevLeaveManagementMockState;
	leavePolicy: DevAttendanceLeavePolicyMockState;
	workPolicy: DevAttendanceWorkPolicyMockState;
};

type DevAttendanceMockRequest = {
	method: string;
	pathname: string;
	searchParams: URLSearchParams;
	body?: string;
	contentType?: string;
};

type DevAttendanceMockResponse = {
	status: number;
	body: unknown;
};

type AbsencePayload = {
	kind: AttendanceAbsenceKind;
	startDate: string;
	endDate: string;
	reason: string;
};

type EventOverridePayload = {
	localDate: string;
	localTime: string;
	locationID: string;
	reason: string;
};

type DevAttendanceEventOverride = EventOverridePayload & {
	eventID: string;
	editedAt: string;
	editedBy: string;
};

export function devAttendanceMockPlugin(options: DevAttendanceMockPluginOptions): Plugin {
	const state = createDevAttendanceMockState(options.userEmail);

	return {
		name: 'internkim-dev-attendance-mock',
		configureServer(server) {
			if (!options.isEnabled) return;

			server.middlewares.use((request, response, next) => {
				const requestURL = new URL(request.url ?? '/', 'http://localhost');
				readRequestBody(request, async (body) => {
					const mockResponse = await createDevAttendanceMockResponse(state, {
						method: request.method ?? 'GET',
						pathname: requestURL.pathname,
						searchParams: requestURL.searchParams,
						body,
						contentType: request.headers['content-type']
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

export function createDevAttendanceMockState(userEmail: string): DevAttendanceMockState {
	const leave = createDevEmployeeLeaveMockState();
	return {
		userEmail,
		locale: 'ko',
		createdAbsences: [],
		canceledAbsenceIDs: new Set(),
		eventOverrides: {},
		nextAbsenceID: 1,
		leave,
		leaveApproval: createDevLeaveApprovalMockState(leave),
		leaveManagement: createDevLeaveManagementMockState(leave),
		leavePolicy: createDevAttendanceLeavePolicyMockState(),
		workPolicy: createDevAttendanceWorkPolicyMockState()
	};
}

export async function createDevAttendanceMockResponse(
	state: DevAttendanceMockState,
	request: DevAttendanceMockRequest
): Promise<DevAttendanceMockResponse | undefined> {
	if (request.method === 'GET' && request.pathname === '/auth/session') {
		return {
			status: 200,
			body: { authenticated: true, email: state.userEmail, isAdmin: true }
		};
	}
	if (request.method === 'GET' && request.pathname === '/admin/api/session') {
		return {
			status: 200,
			body: {
				email: state.userEmail,
				claimedAdminEmail: state.userEmail,
				isAdmin: true,
				isClaimed: true,
				bootstrapStatus: 'claimed'
			}
		};
	}
	if (request.method === 'GET' && request.pathname === '/admin/api/locale') {
		return { status: 200, body: { locale: state.locale } };
	}
	if (request.method === 'PUT' && request.pathname === '/admin/api/locale') {
		state.locale = localeFromBody(request.body);
		return { status: 200, body: { locale: state.locale } };
	}
	const leavePolicyResponse = createDevAttendanceLeavePolicyMockResponse(
		state.leavePolicy,
		request
	);
	if (leavePolicyResponse) {
		if (request.method === 'PUT' && request.pathname === '/admin/api/attendance-leave-policy') {
			const nextMode = state.leavePolicy.policy.balanceTrackingMode;
			if (state.leave.payload.balanceTrackingMode !== nextMode && nextMode === 'unlimited') {
				state.leave.managedBalancesByLeaveType = Object.fromEntries(
					state.leave.payload.leaveTypes.map((leaveType) => [
						leaveType.id,
						leaveType.balance ? structuredClone(leaveType.balance) : undefined
					])
				);
				state.leave.payload.balanceTrackingMode = nextMode;
				synchronizeUnlimitedEmployeeLeaveUsage(state.leave.payload);
			} else if (state.leave.payload.balanceTrackingMode !== nextMode) {
				state.leave.payload.balanceTrackingMode = nextMode;
				for (const leaveType of state.leave.payload.leaveTypes) {
					const balance = state.leave.managedBalancesByLeaveType[leaveType.id];
					leaveType.balance = balance ? structuredClone(balance) : undefined;
				}
			}
		}
		return leavePolicyResponse;
	}
	const workPolicyResponse = createDevAttendanceWorkPolicyMockResponse(
		state.workPolicy,
		request
	);
	if (workPolicyResponse) return workPolicyResponse;
	const leaveResponse = createDevEmployeeLeaveMockResponse(state.leave, request);
	if (leaveResponse) return leaveResponse;
	const leaveApprovalResponse = createDevLeaveApprovalMockResponse(state.leaveApproval, request);
	if (leaveApprovalResponse) return leaveApprovalResponse;
	const leaveManagementResponse = createDevLeaveManagementMockResponse(
		state.leaveManagement,
		request
	);
	if (leaveManagementResponse) return leaveManagementResponse;
	if (request.method === 'GET' && request.pathname === '/attendance/api/work-status') {
		return {
			status: 200,
			body: createDevAttendanceWorkStatus(
				state.userEmail,
				request.searchParams.get('period'),
				request.searchParams.get('anchor')
			)
		};
	}
	if (request.method === 'GET' && request.pathname === '/attendance/api/summary') {
		const month = request.searchParams.get('month') || currentMonth();
		const summary = buildAttendanceSummaryFixture(month);
		const events = summary.events.map((event) =>
			projectEventOverrides(event, state.eventOverrides[event.id] ?? [], summary.locations)
		);
		return {
			status: 200,
			body: {
				...summary,
				currentUserEmail: state.userEmail,
				events,
				absences: [...summary.absences, ...state.createdAbsences].filter(
					(absence) => !state.canceledAbsenceIDs.has(absence.id)
				)
			}
		};
	}
	if (request.method === 'PATCH' && request.pathname.startsWith('/attendance/api/events/')) {
		const eventID = decodeURIComponent(request.pathname.replace('/attendance/api/events/', ''));
		const payload = eventOverridePayloadFromBody(request.body);
		const nextOverride = {
			...payload,
			eventID,
			editedAt: new Date().toISOString(),
			editedBy: state.userEmail
		};
		state.eventOverrides[eventID] = [...(state.eventOverrides[eventID] ?? []), nextOverride];
		return { status: 200, body: { ok: true } };
	}
	if (request.method === 'POST' && request.pathname === '/attendance/api/absences') {
		const payload = absencePayloadFromBody(request.body);
		const rangeID = `dev-absence-range-${state.nextAbsenceID++}`;
		const dates = datesBetween(payload.startDate, payload.endDate);
		const dateSet = new Set(dates);
		const absences = dates.map((date) => ({
			id: `${rangeID}__date_${date}`,
			rangeID,
			email: state.userEmail,
			kind: payload.kind,
			labelKey: payload.kind,
			date,
			startDate: payload.startDate,
			endDate: payload.endDate,
			reason: payload.reason,
			createdBy: state.userEmail,
			createdAt: `${date}T09:00:00+09:00`,
			isRangeStart: date === dates[0],
			isRangeEnd: date === dates[dates.length - 1],
			isChunkStart: !dateSet.has(addDays(date, -1)),
			isChunkEnd: !dateSet.has(addDays(date, 1))
		}));
		state.createdAbsences.push(...absences);
		return { status: 200, body: { absences } };
	}
	if (request.method === 'DELETE' && request.pathname.startsWith('/attendance/api/absences/')) {
		const absenceID = decodeURIComponent(request.pathname.replace('/attendance/api/absences/', ''));
		const summary = buildAttendanceSummaryFixture(currentMonth());
		const absence =
			state.createdAbsences.find((candidate) => candidate.id === absenceID) ??
			summary.absences.find((candidate) => candidate.id === absenceID);
		if (!absence) return { status: 404, body: 'attendance absence not found' };
		state.canceledAbsenceIDs.add(absenceID);
		state.createdAbsences = state.createdAbsences.filter((candidate) => candidate.id !== absenceID);
		return { status: 200, body: { ok: true } };
	}
	return undefined;
}

function projectEventOverrides(
	event: AttendanceSummary['events'][number],
	overrides: DevAttendanceEventOverride[],
	locations: AttendanceSummary['locations']
): AttendanceSummary['events'][number] {
	if (overrides.length === 0) return event;
	let updatedEvent = event;
	const overrideHistory: NonNullable<AttendanceSummary['events'][number]['overrideHistory']> = [];
	for (const [index, override] of overrides.entries()) {
		const projection = projectSingleEventOverride(updatedEvent, override, locations, index);
		updatedEvent = projection.event;
		overrideHistory.unshift(projection.history);
	}
	const latestHistory = overrideHistory[0];
	return {
		...updatedEvent,
		originalOccurredAt: latestHistory.originalOccurredAt,
		originalLocalDate: latestHistory.originalLocalDate,
		originalLocalTime: latestHistory.originalLocalTime,
		originalLocationID: latestHistory.originalLocationID,
		originalLocationName: latestHistory.originalLocationName,
		overrideReason: latestHistory.reason,
		overriddenBy: latestHistory.editedBy,
		overriddenAt: latestHistory.editedAt,
		overrideHistory
	};
}

function projectSingleEventOverride(
	event: AttendanceSummary['events'][number],
	override: DevAttendanceEventOverride,
	locations: AttendanceSummary['locations'],
	index: number
): {
	event: AttendanceSummary['events'][number];
	history: NonNullable<AttendanceSummary['events'][number]['overrideHistory']>[number];
} {
	const location = locations.find((candidate) => candidate.id === override.locationID);
	const locationID = location?.id ?? event.locationID;
	const locationName = location?.name ?? event.locationName;
	const overrideLocalTime = normalizeLocalTime(override.localTime);
	const overrideOccurredAt = `${override.localDate}T${overrideLocalTime}:00+09:00`;
	return {
		event: {
			...event,
			occurredAt: overrideOccurredAt,
			localDate: override.localDate,
			localTime: overrideLocalTime,
			locationID,
			locationName
		},
		history: {
			id: `dev-override-${event.id}-${index}`,
			eventID: event.id,
			editedBy: override.editedBy,
			editedAt: override.editedAt,
			reason: override.reason,
			originalOccurredAt: event.occurredAt,
			originalLocalDate: event.localDate,
			originalLocalTime: event.localTime,
			originalLocationID: event.locationID ?? '',
			originalLocationName: event.locationName ?? '',
			overrideOccurredAt,
			overrideLocalDate: override.localDate,
			overrideLocalTime,
			overrideLocationID: locationID ?? '',
			overrideLocationName: locationName ?? ''
		}
	};
}

function absencePayloadFromBody(body: string | undefined): AbsencePayload {
	const parsed = parseJSONRecord(body);
	const kind = absenceKindFromValue(parsed.kind);
	const startDate = dateFromValue(parsed.startDate);
	const endDate = dateFromValue(parsed.endDate) || startDate;
	const reason = typeof parsed.reason === 'string' ? parsed.reason.trim() : '';
	return { kind, startDate, endDate, reason };
}

function eventOverridePayloadFromBody(body: string | undefined): EventOverridePayload {
	const parsed = parseJSONRecord(body);
	return {
		localDate: dateFromValue(parsed.localDate),
		localTime: timeFromValue(parsed.localTime),
		locationID: typeof parsed.locationID === 'string' ? parsed.locationID.trim() : '',
		reason: typeof parsed.reason === 'string' ? parsed.reason.trim() : ''
	};
}

function localeFromBody(body: string | undefined): 'ko' | 'en' {
	const parsed = parseJSONRecord(body);
	return parsed.locale === 'en' ? 'en' : 'ko';
}

function parseJSONRecord(body: string | undefined): Record<string, unknown> {
	if (!body) return {};
	try {
		const parsed: unknown = JSON.parse(body);
		if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
			return parsed as Record<string, unknown>;
		}
	} catch {
		return {};
	}
	return {};
}

function absenceKindFromValue(value: unknown): AttendanceAbsenceKind {
	if (value === 'other') return value;
	return 'leave';
}

function dateFromValue(value: unknown): string {
	if (typeof value !== 'string') return todayDate();
	if (/^\d{4}-\d{2}-\d{2}$/.test(value)) return value;
	return todayDate();
}

function timeFromValue(value: unknown): string {
	if (typeof value !== 'string') return '09:00';
	const trimmedValue = value.trim();
	if (/^\d{2}:\d{2}$/.test(trimmedValue)) return trimmedValue;
	if (/^\d{2}:\d{2}:\d{2}$/.test(trimmedValue)) return trimmedValue.slice(0, 5);
	return '09:00';
}

function normalizeLocalTime(localTime: string): string {
	return timeFromValue(localTime);
}

function datesBetween(startDate: string, endDate: string): string[] {
	const start = new Date(`${startDate}T00:00:00Z`);
	const end = new Date(`${endDate}T00:00:00Z`);
	if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime()) || end < start)
		return [startDate];
	const dates: string[] = [];
	const cursor = new Date(start);
	while (cursor <= end && dates.length < 31) {
		const date = cursor.toISOString().slice(0, 10);
		if (isWeekday(date)) {
			dates.push(date);
		}
		cursor.setUTCDate(cursor.getUTCDate() + 1);
	}
	return dates;
}

function addDays(date: string, days: number): string {
	const parsedDate = new Date(`${date}T00:00:00Z`);
	parsedDate.setUTCDate(parsedDate.getUTCDate() + days);
	return parsedDate.toISOString().slice(0, 10);
}

function currentMonth(): string {
	return todayDate().slice(0, 7);
}

function todayDate(): string {
	return todayDateInTimeZone('Asia/Seoul');
}

function readRequestBody(request: IncomingMessage, callback: (body: string) => void) {
	if (request.method === 'GET') {
		callback('');
		return;
	}
	let body = '';
	request.on('data', (chunk: Buffer) => {
		body += chunk.toString('utf8');
	});
	request.on('end', () => callback(body));
	request.on('error', () => callback(''));
}

function writeJSON(response: ServerResponse, status: number, body: unknown) {
	response.statusCode = status;
	response.setHeader('Content-Type', 'application/json');
	response.end(JSON.stringify(body));
}
