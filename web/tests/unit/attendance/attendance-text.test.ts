import { describe, expect, test } from 'bun:test';
import { attendanceText } from '../../../src/routes/attendance/text';

type TextTree = { readonly [key: string]: TextNode };
type TextNode = string | readonly string[] | TextTree;

describe('attendance text', () => {
	test('keeps Korean and English key shapes aligned', () => {
		expect(collectTextShape(attendanceText.en).sort()).toEqual(collectTextShape(attendanceText.ko).sort());
	});

	test('provides localized labels for component status copy', () => {
		expect(attendanceText.en.finished).toBe('Clocked out');
		expect(attendanceText.en.absent).toBe('Not clocked in');
		expect(attendanceText.en.subscriptionDayTemplate).toBe('{count} days');
		expect(attendanceText.en.absenceNoWeekdays).toBe('No weekdays to register.');
		expect(attendanceText.en.absenceKindOther).toBe('Other');
		expect(attendanceText.en.locationSegmentCountTemplate).toBe('{count} segments');
		expect(attendanceText.en.calendarEvents).toBe('Schedule');
		expect(attendanceText.en.teamMonthlyStatus).toBe('Monthly work status table');
		expect(attendanceText.en.teamMemberSearchPlaceholder).toBe('Search employees');
		expect(attendanceText.en.presentCountTemplate).toBe('{present}/{total}');
		expect(attendanceText.en.mobileStatusView).toBe('Team status');
		expect(attendanceText.en.mobileToolsView).toBe('My records');
		expect(attendanceText.ko.finished).toBe('퇴근');
		expect(attendanceText.ko.absent).toBe('미출근');
		expect(attendanceText.ko.subscriptionDayTemplate).toBe('{count}일');
		expect(attendanceText.ko.absenceNoWeekdays).toBe('등록할 평일이 없습니다.');
		expect(attendanceText.ko.absenceKindOther).toBe('기타');
		expect(attendanceText.ko.locationSegmentCountTemplate).toBe('{count}구간');
		expect(attendanceText.ko.calendarEvents).toBe('일정');
		expect(attendanceText.ko.teamMonthlyStatus).toBe('월간 근무 현황표');
		expect(attendanceText.ko.teamMemberSearchPlaceholder).toBe('직원 검색');
		expect(attendanceText.ko.presentCountTemplate).toBe('{present}/{total}');
		expect(attendanceText.ko.mobileStatusView).toBe('팀 현황');
		expect(attendanceText.ko.mobileToolsView).toBe('내 기록');
	});
});

function collectTextShape(node: TextNode, path: string[] = []): string[] {
	if (typeof node === 'string') return [`${path.join('.')}:string`];
	if (Array.isArray(node)) return [`${path.join('.')}:array:${node.length}`];

	return Object.entries(node).flatMap(([key, value]) => collectTextShape(value, [...path, key]));
}
