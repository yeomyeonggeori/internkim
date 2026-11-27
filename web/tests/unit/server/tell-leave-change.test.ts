import { describe, expect, test } from 'bun:test';
import { leaveChangeOf } from '../../../src/lib/server/tell-leave-change';

const sampleID = 'member-sample';

function answered(leave: Record<string, unknown>): { result: Record<string, unknown> } {
	return {
		result: { personID: sampleID, person: '이샘플', kind: '연차', startDate: '2027-03-09', endDate: '2027-03-09', ...leave }
	};
}

describe('what the administrators are told about a leave', () => {
	test('an approved leave taken back by its owner', () => {
		const taken = answered({ status: 'approved', startDate: '2027-03-02', endDate: '2027-03-05' });
		expect(leaveChangeOf('leave_delete', taken, sampleID)).toEqual({
			title: '휴가 취소: 이샘플',
			body: '연차 2027-03-02 ~ 2027-03-05'
		});
	});

	test('a request to move an approved leave', () => {
		const asked = answered({ status: 'requested', replacedLeave: { startDate: '2027-03-02', endDate: '2027-03-02' } });
		expect(leaveChangeOf('leave_request', asked, sampleID)).toEqual({
			title: '휴가 변경 요청: 이샘플',
			body: '연차 2027-03-02 → 2027-03-09'
		});
	});

	test('a move that needed no decision', () => {
		const moved = answered({ status: 'approved', replacedLeave: { startDate: '2027-03-02', endDate: '2027-03-02' } });
		expect(leaveChangeOf('leave_request', moved, sampleID)?.title).toBe('휴가 변경: 이샘플');
	});

	test('the person whose leave an administrator moved, with the reason', () => {
		const moved = answered({ status: 'approved', replacedLeave: { startDate: '2027-03-02', endDate: '2027-03-02' } });
		expect(leaveChangeOf('leave_update', moved, 'member-admin', '출장 일정 변경')).toEqual({
			title: '휴가 수정',
			body: '연차 2027-03-02 → 2027-03-09 · 사유: 출장 일정 변경',
			personID: sampleID
		});
		expect(leaveChangeOf('leave_update', moved, sampleID, '')).toBeNull();
		expect(leaveChangeOf('leave_update', answered({ status: 'approved' }), 'member-admin')).toBeNull();
	});

	test('nothing for a request withdrawn, a new leave, or somebody else\'s leave', () => {
		expect(leaveChangeOf('leave_delete', answered({ status: 'requested' }), sampleID)).toBeNull();
		expect(leaveChangeOf('leave_request', answered({ status: 'requested' }), sampleID)).toBeNull();
		expect(leaveChangeOf('leave_delete', answered({ status: 'approved' }), 'member-admin')).toBeNull();
	});
});
