import type {
	AttendanceLeavePolicy,
	LeaveAllowedUnit,
	LeaveBalanceMode,
	LeaveExpiryMode,
	LeaveGrantCadence,
	LeaveType
} from '../../routes/admin/admin-types';

const partialUnits: LeaveAllowedUnit[] = ['fullDay', 'halfDay', 'quarterDay'];
const fullDayUnits: LeaveAllowedUnit[] = ['fullDay'];

export const annualLeaveTypeID = 'annual';
export const defaultAnnualGrantMilliDays = 15000;

type SystemLeaveType = {
	id: string;
	systemKind: string;
	name: string;
	paid: boolean;
	balanceMode: LeaveBalanceMode;
	grantCadence: LeaveGrantCadence;
	grantAmountMilliDays: number;
	expiryMode: LeaveExpiryMode;
	allowedUnits: LeaveAllowedUnit[];
	includeInSummary: boolean;
};

const systemLeaveTypes: SystemLeaveType[] = [
	{ id: 'annual', systemKind: 'annual', name: '연차', paid: true, balanceMode: 'annual', grantCadence: 'annual', grantAmountMilliDays: defaultAnnualGrantMilliDays, expiryMode: 'fiscalYearEnd', allowedUnits: partialUnits, includeInSummary: true },
	{ id: 'sick', systemKind: 'sick', name: '병가', paid: false, balanceMode: 'none', grantCadence: 'none', grantAmountMilliDays: 0, expiryMode: 'none', allowedUnits: partialUnits, includeInSummary: false },
	{ id: 'bereavement', systemKind: 'bereavement', name: '경조휴가', paid: true, balanceMode: 'none', grantCadence: 'none', grantAmountMilliDays: 0, expiryMode: 'none', allowedUnits: fullDayUnits, includeInSummary: false },
	{ id: 'public', systemKind: 'public', name: '공가', paid: true, balanceMode: 'none', grantCadence: 'none', grantAmountMilliDays: 0, expiryMode: 'none', allowedUnits: fullDayUnits, includeInSummary: false },
	{ id: 'maternity', systemKind: 'maternity', name: '출산휴가', paid: true, balanceMode: 'none', grantCadence: 'none', grantAmountMilliDays: 0, expiryMode: 'none', allowedUnits: fullDayUnits, includeInSummary: false },
	{ id: 'spouse-maternity', systemKind: 'spouseMaternity', name: '배우자 출산휴가', paid: true, balanceMode: 'none', grantCadence: 'none', grantAmountMilliDays: 0, expiryMode: 'none', allowedUnits: fullDayUnits, includeInSummary: false },
	{ id: 'miscarriage-stillbirth', systemKind: 'miscarriageStillbirth', name: '유산·사산휴가', paid: true, balanceMode: 'none', grantCadence: 'none', grantAmountMilliDays: 0, expiryMode: 'none', allowedUnits: fullDayUnits, includeInSummary: false },
	{ id: 'fertility-treatment', systemKind: 'fertilityTreatment', name: '난임치료휴가', paid: true, balanceMode: 'none', grantCadence: 'none', grantAmountMilliDays: 0, expiryMode: 'none', allowedUnits: partialUnits, includeInSummary: false },
	{ id: 'family-care', systemKind: 'familyCare', name: '가족돌봄휴가', paid: false, balanceMode: 'none', grantCadence: 'none', grantAmountMilliDays: 0, expiryMode: 'none', allowedUnits: partialUnits, includeInSummary: false },
	{ id: 'reward', systemKind: 'reward', name: '포상휴가', paid: true, balanceMode: 'separate', grantCadence: 'none', grantAmountMilliDays: 0, expiryMode: 'none', allowedUnits: fullDayUnits, includeInSummary: true },
	{ id: 'compensatory', systemKind: 'compensatory', name: '보상휴가', paid: true, balanceMode: 'separate', grantCadence: 'none', grantAmountMilliDays: 0, expiryMode: 'none', allowedUnits: partialUnits, includeInSummary: true },
	{ id: 'long-service', systemKind: 'longService', name: '장기근속휴가', paid: true, balanceMode: 'separate', grantCadence: 'none', grantAmountMilliDays: 0, expiryMode: 'none', allowedUnits: fullDayUnits, includeInSummary: true },
	{ id: 'refresh', systemKind: 'refresh', name: '리프레시휴가', paid: true, balanceMode: 'separate', grantCadence: 'none', grantAmountMilliDays: 0, expiryMode: 'none', allowedUnits: fullDayUnits, includeInSummary: true },
	{ id: 'parental-leave', systemKind: 'parentalLeave', name: '육아휴직', paid: false, balanceMode: 'none', grantCadence: 'none', grantAmountMilliDays: 0, expiryMode: 'none', allowedUnits: fullDayUnits, includeInSummary: false },
	{ id: 'unpaid', systemKind: 'unpaid', name: '무급휴가', paid: false, balanceMode: 'none', grantCadence: 'none', grantAmountMilliDays: 0, expiryMode: 'none', allowedUnits: partialUnits, includeInSummary: false },
	{ id: 'other', systemKind: 'other', name: '기타 휴가', paid: false, balanceMode: 'none', grantCadence: 'none', grantAmountMilliDays: 0, expiryMode: 'none', allowedUnits: fullDayUnits, includeInSummary: false }
];

export const systemLeaveTypeIDs: ReadonlySet<string> = new Set(
	systemLeaveTypes.map((leaveType) => leaveType.id)
);

export function systemLeaveTypeKind(leaveTypeID: string): string | undefined {
	return systemLeaveTypes.find((leaveType) => leaveType.id === leaveTypeID)?.systemKind;
}

export function defaultLeaveTypes(): LeaveType[] {
	return systemLeaveTypes.map((leaveType, sortOrder) => ({
		...leaveType,
		allowedUnits: [...leaveType.allowedUnits],
		carryoverEnabled: false,
		isActive: true,
		isSystem: true,
		sortOrder
	}));
}

export function defaultLeavePolicy(): AttendanceLeavePolicy {
	return {
		version: 2,
		balanceTrackingMode: 'managed',
		fiscalYearStartMonth: 1,
		fiscalYearStartDay: 1,
		leaveTypes: defaultLeaveTypes(),
		updatedAt: ''
	};
}
