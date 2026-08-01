import type {
	AttendanceWorkPolicy,
	AttendanceWorkPolicyResponse,
	AttendanceWorkPolicyRevision
} from './src/routes/admin/admin-types';

type DevAttendanceWorkPolicyRequest = {
	method: string;
	pathname: string;
	body?: string;
};

type DevAttendanceWorkPolicyResponse = {
	status: number;
	body: unknown;
};

export type DevAttendanceWorkPolicyMockState = {
	policy: AttendanceWorkPolicy;
};

export function createDevAttendanceWorkPolicyMockState(): DevAttendanceWorkPolicyMockState {
	return {
		policy: {
			version: 1,
			updatedAt: '1970-01-01T00:00:00Z',
			revisions: [defaultRevision()]
		}
	};
}

export function createDevAttendanceWorkPolicyMockResponse(
	state: DevAttendanceWorkPolicyMockState,
	request: DevAttendanceWorkPolicyRequest
): DevAttendanceWorkPolicyResponse | undefined {
	if (request.pathname !== '/admin/api/attendance-work-policy') return undefined;
	if (request.method === 'GET') return { status: 200, body: workPolicyResponse(state.policy) };
	if (request.method !== 'PUT') return { status: 405, body: 'method not allowed' };
	const revision = parseRevision(request.body);
	const effectiveDate = new Date().toLocaleDateString('en-CA', { timeZone: 'Asia/Seoul' });
	const savedRevision = { ...revision, effectiveDate };
	const revisions = state.policy.revisions.filter(
		(candidate) => candidate.effectiveDate !== effectiveDate
	);
	state.policy = {
		...state.policy,
		updatedAt: new Date().toISOString(),
		revisions: [...revisions, savedRevision].sort((left, right) =>
			left.effectiveDate.localeCompare(right.effectiveDate)
		)
	};
	return { status: 200, body: workPolicyResponse(state.policy) };
}

function workPolicyResponse(policy: AttendanceWorkPolicy): AttendanceWorkPolicyResponse {
	const currentDate = new Date().toLocaleDateString('en-CA', { timeZone: 'Asia/Seoul' });
	return {
		policy,
		currentMonth: currentDate.slice(0, 7),
		holidayDates: [],
		timeZone: 'Asia/Seoul'
	};
}

function parseRevision(body: string | undefined): AttendanceWorkPolicyRevision {
	if (!body) return defaultRevision();
	const parsed: unknown = JSON.parse(body);
	if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return defaultRevision();
	return parsed as AttendanceWorkPolicyRevision;
}

function defaultRevision(): AttendanceWorkPolicyRevision {
	return {
		effectiveDate: '1970-01-01',
		workMode: 'flexible',
		workingWeekdays: [1, 2, 3, 4, 5],
		dailyTargetMinutes: 480,
		weeklyTargetMinutes: 2400,
		referenceStartTime: '09:00',
		fixedStartTime: '',
		fixedEndTime: '',
		coreTimeEnabled: true,
		coreStartTime: '11:00',
		coreEndTime: '16:00',
		breakPeriods: [{ startTime: '12:00', endTime: '13:00' }],
		nightStartTime: '22:00',
		nightEndTime: '06:00'
	};
}
