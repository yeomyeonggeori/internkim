import { describe, expect, test } from 'bun:test';
import {
	leaveKindOf,
	leaveKindsOfPolicy,
	NoSuchLeaveKind,
	type LeaveKind
} from '$lib/server/public-api/record/leave';

const registered: LeaveKind[] = [
	{ id: 'annual', name: '연차', isPaid: true, isDeducted: true },
	{ id: 'condolence', name: '경조사', isPaid: true, isDeducted: false },
	{ id: 'reserve', name: '예비군', isPaid: false, isDeducted: false }
];

describe('naming a kind of leave', () => {
	test('takes the id, then the registered name, then a unique part', () => {
		expect(leaveKindOf(registered, 'annual').name).toBe('연차');
		expect(leaveKindOf(registered, '경조사').id).toBe('condolence');
		expect(leaveKindOf(registered, '예비').id).toBe('reserve');
	});

	test('refuses a kind the company does not register, and says which it has', () => {
		const refusal = (() => {
			try {
				leaveKindOf(registered, '병가');
			} catch (thrown) {
				return thrown;
			}
		})();

		expect(refusal).toBeInstanceOf(NoSuchLeaveKind);
		expect((refusal as NoSuchLeaveKind).registered).toEqual(['연차', '경조사', '예비군']);
	});
});

describe('the kinds a company offers', () => {
	test('come from its own policy, paid and deducted as it set them', () => {
		const kinds = leaveKindsOfPolicy({
			attendanceLeavePolicy: {
				version: 2,
				balanceTrackingMode: 'managed',
				fiscalYearStartMonth: 1,
				fiscalYearStartDay: 1,
				updatedAt: '2026-08-01T00:00:00.000Z',
				leaveTypes: [
					{ id: 'annual', name: '연차', paid: true, balanceMode: 'annual', isActive: true },
					{ id: 'unpaid', name: '무급휴가', paid: false, balanceMode: 'none', isActive: true },
					{ id: 'retired', name: '옛 휴가', paid: true, balanceMode: 'none', isActive: false }
				]
			}
		});

		expect(kinds.map((kind) => kind.id)).toEqual(['annual', 'unpaid']);
		expect(kinds[0]).toEqual({ id: 'annual', name: '연차', isPaid: true, isDeducted: true });
		expect(kinds[1].isPaid).toBe(false);
		expect(kinds[1].isDeducted).toBe(false);
	});

	test('fall back to the default policy when a company has set none', () => {
		expect(leaveKindsOfPolicy({}).length === 0).toBe(false);
		expect(leaveKindsOfPolicy(null).some((kind) => kind.id === 'annual')).toBe(true);
	});
});
