import { describe, expect, test } from 'bun:test';
import { refusalOfToolInput } from '../../../src/lib/server/public-api/tool-input';

// The rules these hold the gate to are public.attendance_add's, in
// supabase/migrations/20260902000005_an_old_record_is_an_administrators_to_write.sql.
describe('the input gate refuses only what the record would refuse', () => {
	test('clocking in right now is taken without a reason', () => {
		expect(refusalOfToolInput('attendance_add', { kind: 'clock_in', location: '재택' })).toBeNull();
	});

	test('clocking out right now is taken without a reason', () => {
		expect(refusalOfToolInput('attendance_add', { kind: 'clock_out' })).toBeNull();
	});

	test('a record written by hand is taken by the gate and left to the record to judge', () => {
		expect(
			refusalOfToolInput('attendance_add', { kind: 'clock_out', date: '2026-09-01', time: '18:00' })
		).toBeNull();
	});

	test('a correction is taken by the gate and left to the record to ask for a reason', () => {
		expect(refusalOfToolInput('attendance_update', { corrections: [{ eventHint: 'an-event' }] })).toBeNull();
	});

	test('a removal is taken by the gate and left to the record to ask for a reason', () => {
		expect(refusalOfToolInput('attendance_delete', { eventHint: 'an-event' })).toBeNull();
	});

	test('a field the tool does not take is named back', () => {
		expect(refusalOfToolInput('attendance_add', { kind: 'clock_in', mood: 'cheerful' })).toContain('input.mood');
	});
});
