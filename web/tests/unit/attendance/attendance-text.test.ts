import { describe, expect, test } from 'bun:test';
import { attendanceText } from '../../../src/routes/attendance/text';

describe('attendance text', () => {
	test('provides localized labels for component status copy', () => {
		expect(attendanceText.en.finished).toBe('Clocked out');
		expect(attendanceText.en.absent).toBe('Not clocked in');
		expect(attendanceText.en.inProgress).toBe('In progress');
		expect(attendanceText.en.locationOverride).toBe('Location override');
		expect(attendanceText.en.subscriptionDayTemplate).toBe('{count} days');
		expect(attendanceText.ko.finished).toBe('퇴근');
		expect(attendanceText.ko.absent).toBe('미출근');
		expect(attendanceText.ko.inProgress).toBe('진행 중');
		expect(attendanceText.ko.locationOverride).toBe('장소 수정');
		expect(attendanceText.ko.subscriptionDayTemplate).toBe('{count}일');
	});
});
