import { describe, expect, test } from 'bun:test';
import {
	leaveTypeIsValid,
	leaveTypeWithBalanceMode
} from '../../../src/routes/admin/attendance-leave-policy-model';
import type { LeaveType } from '../../../src/routes/admin/admin-types';

function annualLeaveType(): LeaveType {
	return {
		id: 'annual',
		systemKind: 'annual',
		name: '연차',
		paid: true,
		balanceMode: 'annual',
		grantCadence: 'annual',
		grantAmountMilliDays: 15000,
		expiryMode: 'fiscalYearEnd',
		carryoverEnabled: false,
		allowedUnits: ['fullDay', 'halfDay', 'quarterDay'],
		includeInSummary: true,
		isActive: true,
		isSystem: true,
		sortOrder: 0
	};
}

describe('attendance leave policy model', () => {
	test('allows administrators to configure annual grant days without a legal floor', () => {
		const leaveType = annualLeaveType();

		expect(leaveTypeIsValid(leaveType)).toBe(true);
		expect(leaveTypeIsValid({ ...leaveType, grantAmountMilliDays: 20000 })).toBe(true);
		expect(leaveTypeIsValid({ ...leaveType, grantAmountMilliDays: 250 })).toBe(true);
	});

	test('clears separate policy fields when using the annual balance', () => {
		const leaveType = {
			...annualLeaveType(),
			id: 'custom-family',
			systemKind: '',
			name: '가족돌봄 휴가',
			balanceMode: 'separate' as const,
			grantCadence: 'annual' as const,
			grantAmountMilliDays: 3000,
			expiryMode: 'monthsAfterGrant' as const,
			expiryMonths: 12,
			carryoverEnabled: true,
			carryoverLimitMilliDays: 1000,
			isSystem: false
		};

		expect(leaveTypeWithBalanceMode(leaveType, 'annual')).toEqual({
			...leaveType,
			balanceMode: 'annual',
			grantCadence: 'none',
			grantAmountMilliDays: 0,
			expiryMode: 'none',
			expiryMonths: undefined,
			carryoverEnabled: false,
			carryoverLimitMilliDays: undefined,
			includeInSummary: false
		});
	});
});
