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
	return {
		id: '',
		systemKind: '',
		name: '',
		paid: false,
		balanceMode: 'separate',
		grantCadence: 'manual',
		grantAmountMilliDays: 0,
		expiryMode: 'none',
		carryoverEnabled: false,
		allowedUnits: ['fullDay'],
		isActive: true,
		isSystem: false,
		sortOrder
	};
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

export function isLeaveBalanceMode(value: string | undefined): value is LeaveBalanceMode {
	return value === 'annual' || value === 'separate' || value === 'none';
}

export function isLeaveGrantCadence(value: string | undefined): value is LeaveGrantCadence {
	return (
		value === 'statutory' ||
		value === 'annual' ||
		value === 'monthly' ||
		value === 'manual' ||
		value === 'none'
	);
}

export function isLeaveExpiryMode(value: string | undefined): value is LeaveExpiryMode {
	return value === 'fiscalYearEnd' || value === 'monthsAfterGrant' || value === 'none';
}

export function daysFromMilliDays(value: number | undefined): string {
	return ((value ?? 0) / 1000).toString();
}

export function milliDaysFromDays(value: string): number {
	const parsed = Number(value);
	return Number.isFinite(parsed) ? Math.round(parsed * 1000) : 0;
}

export function optionalMilliDaysFromDays(value: string): number | undefined {
	return value.trim() ? milliDaysFromDays(value) : undefined;
}

export const leaveAllowedUnits: LeaveAllowedUnit[] = ['fullDay', 'halfDay', 'quarterDay'];
