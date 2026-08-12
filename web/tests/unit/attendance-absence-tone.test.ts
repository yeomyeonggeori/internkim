import { describe, expect, test } from 'bun:test';
import { ABSENCE_TONE, STATUS_TONE, absenceDisplayClass } from '../../src/routes/attendance/shared/color-tokens';
import { dayOffColor } from '../../src/lib/calendar/day-off-color';

describe('leave reads as a day off across attendance', () => {
	test('the monthly table draws a leave bar in the day-off colour', () => {
		expect(ABSENCE_TONE.leave.bar).toBe(dayOffColor);
	});

	test('leave labels and meters are red, and other absences stay muted', () => {
		expect(ABSENCE_TONE.leave.label).toContain('destructive');
		expect(ABSENCE_TONE.leave.meter).toContain('destructive');
		expect(ABSENCE_TONE.other.label).toContain('muted');
		expect(ABSENCE_TONE.other.meter).toContain('muted');
	});

	test('no absence surface is left on the info colour', () => {
		expect(STATUS_TONE.absence).not.toContain('info');
		expect(absenceDisplayClass('leave', true)).not.toContain('info');
	});

	test('an absence that is not leave keeps its own muted treatment', () => {
		expect(absenceDisplayClass('other')).toContain('muted');
	});
});
