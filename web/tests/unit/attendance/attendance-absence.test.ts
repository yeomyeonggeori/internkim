import { describe, expect, test } from 'bun:test';
import {
	absencesForDate,
	absenceLabelText,
	hasAbsenceDetails
} from '../../../src/routes/attendance/shared/attendance-absence';
import { computePeopleToday } from '../../../src/routes/attendance/shared/attendance-people';
import type { AttendanceAbsence } from '../../../src/routes/attendance/attendance-context.svelte';
import { attendanceText } from '../../../src/routes/attendance/text';

const absences: AttendanceAbsence[] = [
	{
		id: 'absence-1',
		email: 'kim@example.com',
		kind: 'leave',
		labelKey: 'leave',
		date: '2026-06-10',
		reason: 'family',
		createdBy: 'kim@example.com',
		createdAt: '2026-06-01T09:00:00Z'
	},
	{
		id: 'absence-2',
		email: 'lee@example.com',
		kind: 'business_trip',
		labelKey: 'business_trip',
		date: '2026-06-10',
		createdAt: '2026-06-01T09:00:00Z'
	},
	{
		id: 'absence-3',
		email: 'kim@example.com',
		kind: 'leave',
		labelKey: 'leave',
		date: '2026-06-11',
		createdAt: '2026-06-01T09:00:00Z'
	}
];

describe('attendance absence helpers', () => {
	test('finds absences for a specific date and email', () => {
		const result = absencesForDate(absences, '2026-06-10', 'kim@example.com');

		expect(result.map((absence) => absence.id)).toEqual(['absence-1']);
	});

	test('maps backend label keys through attendance localization text', () => {
		expect(absenceLabelText(absences[0], attendanceText.ko)).toBe('휴가');
		expect(absenceLabelText(absences[1], attendanceText.en)).toBe('Business trip');
	});

	test('treats sanitized absence records as having no private details', () => {
		expect(hasAbsenceDetails(absences[0])).toBe(true);
		expect(hasAbsenceDetails(absences[1])).toBe(false);
	});

	test('classifies a person with an absence separately from not clocked in', () => {
		const people = computePeopleToday('2026-06-10', [], undefined, '2026-06-10', absences);

		expect(people.find((person) => person.email === 'kim@example.com')?.status).toBe('absence');
		expect(people.find((person) => person.email === 'lee@example.com')?.status).toBe('absence');
	});

	test('does not add people to today from absences on another date', () => {
		const people = computePeopleToday('2026-06-10', [], undefined, '2026-06-10', [
			{
				id: 'future-absence',
				email: 'future@example.com',
				kind: 'leave',
				labelKey: 'leave',
				date: '2026-06-11',
				createdAt: '2026-06-01T09:00:00Z'
			}
		]);

		expect(people.some((person) => person.email === 'future@example.com')).toBe(false);
	});
});
