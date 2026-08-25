import { describe, expect, test } from 'bun:test';
import type { AttendanceMember } from '../../../src/routes/attendance/attendance-context.svelte';
import { leaveApprovalEmployeeName } from '../../../src/routes/attendance/approval/leave-approval-employee-name';

const members: AttendanceMember[] = [
	{ email: 'member1@example.com', displayName: '이샘플', mattermostUsername: 'lee' },
	{ email: 'Park@example.com', displayName: '박예시', mattermostUsername: 'park' },
	{ email: 'choi@example.com', displayName: '', mattermostUsername: 'choi' }
];

describe('leaveApprovalEmployeeName', () => {
	test('resolves the display name for a known address', () => {
		expect(leaveApprovalEmployeeName(members, 'member1@example.com')).toBe('이샘플');
	});

	test('matches the address regardless of case', () => {
		expect(leaveApprovalEmployeeName(members, 'park@EXAMPLE.com')).toBe('박예시');
	});

	test('falls back to the address when the member is unknown', () => {
		expect(leaveApprovalEmployeeName(members, 'nobody@example.com')).toBe('nobody@example.com');
	});

	test('falls back to the address when the member has no display name', () => {
		expect(leaveApprovalEmployeeName(members, 'choi@example.com')).toBe('choi@example.com');
	});
});
