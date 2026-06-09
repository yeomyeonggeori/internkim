import type { Plugin } from 'vite';
import type { IncomingMessage, ServerResponse } from 'node:http';
import type { AttendanceAbsence, AttendanceAbsenceKind } from './src/routes/attendance/attendance-context.svelte';
import { buildAttendanceSummaryFixture } from './tests/fixtures/attendance-summary';

type DevAttendanceMockPluginOptions = {
	isEnabled: boolean;
	userEmail: string;
};

type DevAttendanceMockState = {
	userEmail: string;
	locale: 'ko' | 'en';
	createdAbsences: AttendanceAbsence[];
	nextAbsenceID: number;
};

type DevAttendanceMockRequest = {
	method: string;
	pathname: string;
	searchParams: URLSearchParams;
	body?: string;
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

export function createDevAttendanceMockState(userEmail: string): DevAttendanceMockState {
	return {
		userEmail,
		locale: 'ko',
		createdAbsences: [],
		nextAbsenceID: 1
	};
}

export async function createDevAttendanceMockResponse(
	state: DevAttendanceMockState,
	request: DevAttendanceMockRequest
): Promise<DevAttendanceMockResponse | undefined> {
	if (request.method === 'GET' && request.pathname === '/auth/session') {
		return { status: 200, body: { authenticated: true, email: state.userEmail, isAdmin: true } };
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
	if (request.method === 'GET' && request.pathname === '/attendance/api/summary') {
		const month = request.searchParams.get('month') || currentMonth();
		const summary = buildAttendanceSummaryFixture(month);
		return {
			status: 200,
			body: {
				...summary,
				currentUserEmail: state.userEmail,
				absences: [...summary.absences, ...state.createdAbsences]
			}
		};
	}
	if (request.method === 'POST' && request.pathname === '/attendance/api/absences') {
		const payload = absencePayloadFromBody(request.body);
		const absences = datesBetween(payload.startDate, payload.endDate).map((date) => ({
			id: `dev-absence-${state.nextAbsenceID++}`,
			email: state.userEmail,
			kind: payload.kind,
			labelKey: payload.kind,
			date,
			reason: payload.reason,
			createdBy: state.userEmail,
			createdAt: `${date}T09:00:00+09:00`
		}));
		state.createdAbsences.push(...absences);
		return { status: 200, body: { absences } };
	}
	return undefined;
}

function absencePayloadFromBody(body: string | undefined): AbsencePayload {
	const parsed = parseJSONRecord(body);
	const kind = absenceKindFromValue(parsed.kind);
	const startDate = dateFromValue(parsed.startDate);
	const endDate = dateFromValue(parsed.endDate) || startDate;
	const reason = typeof parsed.reason === 'string' ? parsed.reason.trim() : '';
	return { kind, startDate, endDate, reason };
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
	if (value === 'business_trip' || value === 'day_off' || value === 'other') return value;
	return 'leave';
}

function dateFromValue(value: unknown): string {
	if (typeof value !== 'string') return todayDate();
	if (/^\d{4}-\d{2}-\d{2}$/.test(value)) return value;
	return todayDate();
}

function datesBetween(startDate: string, endDate: string): string[] {
	const start = new Date(`${startDate}T00:00:00Z`);
	const end = new Date(`${endDate}T00:00:00Z`);
	if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime()) || end < start) return [startDate];
	const dates: string[] = [];
	const cursor = new Date(start);
	while (cursor <= end && dates.length < 31) {
		dates.push(cursor.toISOString().slice(0, 10));
		cursor.setUTCDate(cursor.getUTCDate() + 1);
	}
	return dates;
}

function currentMonth(): string {
	return todayDate().slice(0, 7);
}

function todayDate(): string {
	return new Date().toISOString().slice(0, 10);
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
