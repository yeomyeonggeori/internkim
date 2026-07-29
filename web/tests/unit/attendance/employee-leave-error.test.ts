import { describe, expect, test } from 'bun:test';
import { EmployeeLeaveAPIError } from '../../../src/routes/attendance/leave/employee-leave-api';
import { employeeLeaveErrorMessage } from '../../../src/routes/attendance/leave/employee-leave-error';
import { attendanceText } from '../../../src/routes/attendance/text';

describe('employee leave error localization', () => {
	test('maps recognized codes in Korean and English', () => {
		expect(
			employeeLeaveErrorMessage(
				new EmployeeLeaveAPIError('leaveConflict', 409),
				attendanceText.ko.leave,
				attendanceText.ko.leave.previewFailed
			)
		).toBe('이미 신청된 휴가와 일정이 겹칩니다.');
		expect(
			employeeLeaveErrorMessage(
				new EmployeeLeaveAPIError('insufficientBalance', 409),
				attendanceText.en.leave,
				attendanceText.en.leave.mutationFailed
			)
		).toBe('There is not enough leave available.');
		expect(
			employeeLeaveErrorMessage(
				new EmployeeLeaveAPIError('hireDateRequired', 409),
				attendanceText.ko.leave,
				attendanceText.ko.leave.mutationFailed
			)
		).toBe('이 휴가를 신청하려면 입사일을 먼저 등록해야 합니다.');
	});

	test('uses a localized generic fallback for unknown response shapes', () => {
		expect(
			employeeLeaveErrorMessage(
				new EmployeeLeaveAPIError(null, 502),
				attendanceText.ko.leave,
				attendanceText.ko.leave.previewFailed
			)
		).toBe('신청 내용을 계산하지 못했습니다.');
		expect(
			employeeLeaveErrorMessage(
				new Error('raw upstream response'),
				attendanceText.en.leave,
				attendanceText.en.leave.mutationFailed
			)
		).toBe('Could not process the leave request.');
	});
});
