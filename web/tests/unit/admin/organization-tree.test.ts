import { describe, expect, test } from 'bun:test';
import type { UserRecord } from '../../../src/routes/admin/admin-types';
import {
	isSupervisorCandidateForRecord,
	supervisorCandidatesForRecord
} from '../../../src/routes/admin/organization-tree';

function userRecord(overrides: Partial<UserRecord>): UserRecord {
	return {
		userID: '',
		handle: '',
		name: '',
		email: '',
		hireDate: '',
		role: 'member',
		...overrides
	};
}

describe('organization tree', () => {
	test('excludes the record and descendants from supervisor candidates', () => {
		const founder = userRecord({
			userID: 'founder',
			name: 'Founder',
			email: 'founder@example.com',
			hireDate: '2026-01-01'
		});
		const manager = userRecord({
			userID: 'manager',
			name: 'Manager',
			email: 'manager@example.com',
			hireDate: '2026-02-01',
			supervisorID: 'founder'
		});
		const report = userRecord({
			userID: 'report',
			name: 'Report',
			email: 'report@example.com',
			hireDate: '2026-03-01',
			supervisorID: 'manager'
		});
		const otherRoot = userRecord({
			userID: 'other-root',
			name: 'Other Root',
			email: 'other-root@example.com',
			hireDate: '2026-01-15'
		});

		const candidates = supervisorCandidatesForRecord([founder, manager, report, otherRoot], manager);

		expect(candidates.map((candidate) => candidate.userID)).toEqual(['founder', 'other-root']);
	});

	test('detects invalid supervisor selections', () => {
		const founder = userRecord({
			userID: 'founder',
			name: 'Founder',
			email: 'founder@example.com',
			hireDate: '2026-01-01'
		});
		const manager = userRecord({
			userID: 'manager',
			name: 'Manager',
			email: 'manager@example.com',
			hireDate: '2026-02-01',
			supervisorID: 'founder'
		});
		const report = userRecord({
			userID: 'report',
			name: 'Report',
			email: 'report@example.com',
			hireDate: '2026-03-01',
			supervisorID: 'manager'
		});

		expect(isSupervisorCandidateForRecord([founder, manager, report], { ...manager, supervisorID: 'report' })).toBe(false);
		expect(isSupervisorCandidateForRecord([founder, manager, report], { ...manager, supervisorID: 'founder' })).toBe(true);
		expect(isSupervisorCandidateForRecord([founder, manager, report], { ...manager, supervisorID: '' })).toBe(true);
	});
});
