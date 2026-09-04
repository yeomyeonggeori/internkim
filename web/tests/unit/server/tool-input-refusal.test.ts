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

// A model gets one call to recover with, and it spends it on whatever the
// refusal named. zod's own wording says what the field is not without saying
// what it holds, so the refusal is rewritten to name the field, what it takes,
// and what came in. The capability runtime writes the same sentences in Go.
describe('a refusal says what the field takes and what came in', () => {
	test('names the choices and the value that is not one of them', () => {
		expect(refusalOfToolInput('attendance_add', { kind: 'clocking_in' })).toBe(
			'input.kind must be one of "clock_in", "clock_out", and it is "clocking_in"'
		);
	});

	test('names the type it takes and the value that is not it', () => {
		expect(refusalOfToolInput('task_list', { limit: 'ten' })).toBe(
			'input.limit must be a number, and it is "ten"'
		);
	});

	test('says a required field is missing rather than that undefined is the wrong type', () => {
		expect(refusalOfToolInput('attendance_add', {})).toBe('input.kind is required and is missing');
	});

	test('names a field the tool does not take as that, not as an unrecognized key', () => {
		expect(refusalOfToolInput('attendance_add', { kind: 'clock_in', mood: 'cheerful' })).toBe(
			'input.mood is not a field this tool takes'
		);
	});

	test('names every fault at once so one recovery answers all of them', () => {
		const refused = refusalOfToolInput('task_list', { limit: 'ten', scope: 'everyone' });
		expect(refused).toContain('input.limit must be a number, and it is "ten"');
		expect(refused).toContain('input.scope must be one of');
	});
});
