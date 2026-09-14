import type {
	LeaveAllowedUnit,
	LeaveBalanceMode,
	LeaveExpiryMode,
	LeaveGrantCadence,
	LeaveType
} from './admin-types';

export function copyLeaveType(leaveType: LeaveType): LeaveType {
	return { ...leaveType, allowedUnits: [...leaveType.allowedUnits] };
}

export function createLeaveType(sortOrder: number): LeaveType {
	return leaveTypeWithBalanceMode(
		{
			id: '',
			systemKind: '',
			name: '',
			paid: false,
			balanceMode: 'none',
			grantCadence: 'none',
			grantAmountMilliDays: 0,
			expiryMode: 'none',
			carryoverEnabled: false,
			allowedUnits: ['fullDay'],
			includeInSummary: false,
			isActive: true,
			isSystem: false,
			sortOrder
		},
		'none'
	);
}

export function leaveTypeIsValid(leaveType: LeaveType): boolean {
	if (!leaveType.name.trim() || leaveType.allowedUnits.length === 0) return false;
	if (leaveType.grantAmountMilliDays < 0) return false;
	if (
		leaveType.expiryMode === 'monthsAfterGrant' &&
		(!leaveType.expiryMonths || leaveType.expiryMonths < 1)
	) {
		return false;
	}
	return (
		leaveType.carryoverLimitMilliDays === undefined ||
		leaveType.carryoverLimitMilliDays >= 0
	);
}

export function leaveTypeWithBalanceMode(
	leaveType: LeaveType,
	balanceMode: LeaveBalanceMode
): LeaveType {
	if (balanceMode === 'separate' || (balanceMode === 'annual' && leaveType.id === 'annual')) {
		return {
			...leaveType,
			balanceMode,
			grantCadence: leaveType.grantCadence,
			includeInSummary: leaveType.includeInSummary
		};
	}
	return {
		...leaveType,
		balanceMode,
		grantCadence: 'none',
		grantAmountMilliDays: 0,
		expiryMode: 'none',
		expiryMonths: undefined,
		carryoverEnabled: false,
		carryoverLimitMilliDays: undefined,
		includeInSummary: false
	};
}

export function leaveTypeAccrues(leaveType: LeaveType): boolean {
	return leaveType.balanceMode === 'annual' && leaveType.id === 'annual';
}

export function isLeaveBalanceMode(value: string | undefined): value is LeaveBalanceMode {
	return value === 'annual' || value === 'separate' || value === 'none';
}

export function isLeaveGrantCadence(value: string | undefined): value is LeaveGrantCadence {
	return value === 'annual' || value === 'monthly' || value === 'none';
}

export function isLeaveExpiryMode(value: string | undefined): value is LeaveExpiryMode {
	return value === 'fiscalYearEnd' || value === 'monthsAfterGrant' || value === 'none';
}

export function daysFromMilliDays(value: number | undefined): number {
	return (value ?? 0) / 1000;
}

export function optionalDaysFromMilliDays(value: number | undefined): number | null {
	return value === undefined ? null : value / 1000;
}

export function milliDaysFromDays(value: number | null): number {
	return value === null || !Number.isFinite(value) ? 0 : Math.round(value * 1000);
}

export function optionalMilliDaysFromDays(value: number | null): number | undefined {
	return value === null ? undefined : milliDaysFromDays(value);
}

export const leaveAllowedUnits: LeaveAllowedUnit[] = ['fullDay', 'halfDay', 'quarterDay'];

function leaveUnitsThrough(unit: LeaveAllowedUnit): LeaveAllowedUnit[] {
	return leaveAllowedUnits.slice(0, leaveAllowedUnits.indexOf(unit) + 1);
}

export function leaveUnitsWith(
	units: LeaveAllowedUnit[],
	unit: LeaveAllowedUnit,
	offered: boolean
): LeaveAllowedUnit[] {
	if (offered) return leaveUnitsThrough(unit);
	return leaveAllowedUnits.slice(0, leaveAllowedUnits.indexOf(unit)).filter((held) => units.includes(held));
}
