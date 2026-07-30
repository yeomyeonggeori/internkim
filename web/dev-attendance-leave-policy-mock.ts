import type {
	AttendanceLeavePolicy,
	LeaveType
} from './src/routes/admin/admin-types';

export type DevAttendanceLeavePolicyMockState = {
	policy: AttendanceLeavePolicy;
	nextCustomLeaveTypeID: number;
};

type DevAttendanceLeavePolicyMockRequest = {
	method: string;
	pathname: string;
	body?: string;
};

type DevAttendanceLeavePolicyMockResponse = {
	status: number;
	body: unknown;
};

const partialLeaveUnits: LeaveType['allowedUnits'] = [
	'fullDay',
	'halfDay',
	'quarterDay'
];

function systemLeaveType(
	id: string,
	name: string,
	overrides: Partial<LeaveType> = {}
): LeaveType {
	return {
		id,
		systemKind: id,
		name,
		paid: false,
		balanceMode: 'none',
		grantCadence: 'none',
		grantAmountMilliDays: 0,
		expiryMode: 'none',
		carryoverEnabled: false,
		allowedUnits: ['fullDay'],
		includeInSummary: false,
		isActive: true,
		isSystem: true,
		sortOrder: 0,
		...overrides
	};
}

export function createDefaultAttendanceLeavePolicy(): AttendanceLeavePolicy {
	return {
		version: 2,
		balanceTrackingMode: 'managed',
		fiscalYearStartMonth: 1,
		fiscalYearStartDay: 1,
		leaveTypes: [
			systemLeaveType('annual', '연차', {
				paid: true,
				balanceMode: 'annual',
				grantCadence: 'annual',
				grantAmountMilliDays: 15000,
				expiryMode: 'fiscalYearEnd',
				allowedUnits: partialLeaveUnits,
				includeInSummary: true,
				sortOrder: 0
			}),
			systemLeaveType('sick', '병가', {
				allowedUnits: partialLeaveUnits,
				sortOrder: 1
			}),
			systemLeaveType('bereavement', '경조휴가', { paid: true, sortOrder: 2 }),
			systemLeaveType('public', '공가', { paid: true, sortOrder: 3 }),
			systemLeaveType('maternity', '출산휴가', { paid: true, sortOrder: 4 }),
			systemLeaveType('spouse-maternity', '배우자 출산휴가', { paid: true, sortOrder: 5 }),
			systemLeaveType('miscarriage-stillbirth', '유산·사산휴가', {
				paid: true,
				sortOrder: 6
			}),
			systemLeaveType('fertility-treatment', '난임치료휴가', {
				paid: true,
				allowedUnits: partialLeaveUnits,
				sortOrder: 7
			}),
			systemLeaveType('family-care', '가족돌봄휴가', {
				allowedUnits: partialLeaveUnits,
				sortOrder: 8
			}),
			systemLeaveType('reward', '포상휴가', {
				paid: true,
				balanceMode: 'separate',
				includeInSummary: true,
				sortOrder: 9
			}),
			systemLeaveType('compensatory', '보상휴가', {
				paid: true,
				balanceMode: 'separate',
				allowedUnits: partialLeaveUnits,
				includeInSummary: true,
				sortOrder: 10
			}),
			systemLeaveType('long-service', '장기근속휴가', {
				paid: true,
				balanceMode: 'separate',
				includeInSummary: true,
				sortOrder: 11
			}),
			systemLeaveType('refresh', '리프레시휴가', {
				paid: true,
				balanceMode: 'separate',
				includeInSummary: true,
				sortOrder: 12
			}),
			systemLeaveType('parental-leave', '육아휴직', { sortOrder: 13 }),
			systemLeaveType('unpaid', '무급휴가', {
				allowedUnits: partialLeaveUnits,
				sortOrder: 14
			}),
			systemLeaveType('other', '기타 휴가', { sortOrder: 15 })
		],
		updatedAt: ''
	};
}

export function createDevAttendanceLeavePolicyMockState(): DevAttendanceLeavePolicyMockState {
	return {
		policy: createDefaultAttendanceLeavePolicy(),
		nextCustomLeaveTypeID: 1
	};
}

export function createDevAttendanceLeavePolicyMockResponse(
	state: DevAttendanceLeavePolicyMockState,
	request: DevAttendanceLeavePolicyMockRequest
): DevAttendanceLeavePolicyMockResponse | undefined {
	if (request.pathname !== '/admin/api/attendance-leave-policy') return undefined;
	if (request.method === 'GET') {
		return { status: 200, body: structuredClone(state.policy) };
	}
	if (request.method !== 'PUT') return undefined;

	const requestedPolicy = attendanceLeavePolicyFromBody(request.body);
	state.policy = {
		...requestedPolicy,
		updatedAt: new Date().toISOString(),
		leaveTypes: requestedPolicy.leaveTypes.map((leaveType) => ({
			...leaveType,
			id: leaveType.id || `custom-${state.nextCustomLeaveTypeID++}`
		}))
	};
	return { status: 200, body: structuredClone(state.policy) };
}

function attendanceLeavePolicyFromBody(body: string | undefined): AttendanceLeavePolicy {
	const value: unknown = JSON.parse(body ?? '{}');
	if (
		!value ||
		typeof value !== 'object' ||
		Reflect.get(value, 'version') !== 2 ||
		!Array.isArray(Reflect.get(value, 'leaveTypes'))
	) {
		throw new Error('invalid attendance leave policy request');
	}
	return value as AttendanceLeavePolicy;
}
