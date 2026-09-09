import type {
	AttendanceLeavePolicy,
	LeaveAllowedUnit,
	LeaveBalanceMode,
	LeaveExpiryMode,
	LeaveGrantCadence,
	LeaveType
} from '../../routes/admin/admin-types';

const partialUnits: LeaveAllowedUnit[] = ['fullDay', 'halfDay', 'quarterDay'];

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
	{ id: 'paid', systemKind: 'paid', name: '유급 휴가', paid: true, balanceMode: 'none', grantCadence: 'none', grantAmountMilliDays: 0, expiryMode: 'none', allowedUnits: partialUnits, includeInSummary: false },
	{ id: 'unpaid', systemKind: 'unpaid', name: '무급휴가', paid: false, balanceMode: 'none', grantCadence: 'none', grantAmountMilliDays: 0, expiryMode: 'none', allowedUnits: partialUnits, includeInSummary: false }
];

const knownSystemLeaveTypeKinds: Record<string, string> = {
	annual: 'annual',
	paid: 'paid',
	sick: 'sick',
	unpaid: 'unpaid',
	bereavement: 'bereavement',
	public: 'public',
	maternity: 'maternity',
	'spouse-maternity': 'spouseMaternity',
	'miscarriage-stillbirth': 'miscarriageStillbirth',
	'fertility-treatment': 'fertilityTreatment',
	'family-care': 'familyCare',
	reward: 'reward',
	compensatory: 'compensatory',
	'long-service': 'longService',
	refresh: 'refresh',
	'parental-leave': 'parentalLeave',
	other: 'other'
};

export function systemLeaveTypeKind(leaveTypeID: string): string | undefined {
	return knownSystemLeaveTypeKinds[leaveTypeID];
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
