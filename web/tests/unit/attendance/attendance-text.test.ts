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
		expect(attendanceText.en.inProgress).toBe('In progress');
		expect(attendanceText.en.confirmClockOut).toBe('Confirm clock-out');
		expect(attendanceText.en.subscriptionDayTemplate).toBe('{count} days');
		expect(attendanceText.en.absenceNoWeekdays).toBe('No weekdays to register.');
		expect(attendanceText.en.moreLocationsTemplate).toBe('+{count} more');
		expect(attendanceText.en.collapseLocations).toBe('Collapse');
		expect(attendanceText.en.locationSegmentCountTemplate).toBe('{count} segments');
		expect(attendanceText.ko.finished).toBe('퇴근');
		expect(attendanceText.ko.absent).toBe('미출근');
		expect(attendanceText.ko.inProgress).toBe('진행 중');
		expect(attendanceText.ko.confirmClockOut).toBe('퇴근 확정');
		expect(attendanceText.ko.subscriptionDayTemplate).toBe('{count}일');
		expect(attendanceText.ko.absenceNoWeekdays).toBe('등록할 평일이 없습니다.');
		expect(attendanceText.ko.moreLocationsTemplate).toBe('+{count} 더보기');
		expect(attendanceText.ko.collapseLocations).toBe('접기');
		expect(attendanceText.ko.locationSegmentCountTemplate).toBe('{count}구간');
	});
});

function collectTextShape(node: TextNode, path: string[] = []): string[] {
	if (typeof node === 'string') return [`${path.join('.')}:string`];
	if (Array.isArray(node)) return [`${path.join('.')}:array:${node.length}`];

	return Object.entries(node).flatMap(([key, value]) => collectTextShape(value, [...path, key]));
}
