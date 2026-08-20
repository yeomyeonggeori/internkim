import { describe, expect, test } from 'bun:test';
import {
	calculateCalendarCapacitySeconds,
	calculateWorkStandardCapacity
} from '../../../src/routes/attendance/personal/work-standard-capacity';

const hour = 60 * 60;

describe('calculateWorkStandardCapacity', () => {
	test('derives inclusive day, week, and month calendar capacities', () => {
		expect(calculateCalendarCapacitySeconds('2026-08-10', '2026-08-10')).toBe(24 * hour);
		expect(calculateCalendarCapacitySeconds('2026-08-10', '2026-08-16')).toBe(7 * 24 * hour);
		expect(calculateCalendarCapacitySeconds('2026-08-01', '2026-08-31')).toBe(31 * 24 * hour);
	});

	test('keeps the baseline stage through the exact 125 percent boundary', () => {
		const capacity = calculateWorkStandardCapacity({
			actualSeconds: 10 * hour,
			provisionalSeconds: 0,
			targetSeconds: 8 * hour,
			workingCapacitySeconds: 24 * hour,
			calendarCapacitySeconds: 24 * hour,
			referenceCapacitySeconds: 8 * hour,
			hasBaseline: true
		});

		expect(capacity.stage).toBe('baseline-buffer');
		expect(capacity.capacitySeconds).toBe(10 * hour);
		expect(capacity.targetPositionPercent).toBe(80);
		expect(capacity.actualWidthPercent).toBe(100);
	});

	test('expands to working days after one second over the baseline buffer', () => {
		const capacity = calculateWorkStandardCapacity({
			actualSeconds: 10 * hour + 1,
			provisionalSeconds: 0,
			targetSeconds: 8 * hour,
			workingCapacitySeconds: 5 * 24 * hour,
			calendarCapacitySeconds: 7 * 24 * hour,
			referenceCapacitySeconds: 8 * hour,
			hasBaseline: true
		});

		expect(capacity.stage).toBe('working-days');
		expect(capacity.capacitySeconds).toBe(5 * 24 * hour);
	});

	test('expands to all calendar days after one second over working-day capacity', () => {
		const capacity = calculateWorkStandardCapacity({
			actualSeconds: 5 * 24 * hour + 1,
			provisionalSeconds: 0,
			targetSeconds: 40 * hour,
			workingCapacitySeconds: 5 * 24 * hour,
			calendarCapacitySeconds: 7 * 24 * hour,
			referenceCapacitySeconds: 40 * hour,
			hasBaseline: true
		});

		expect(capacity.stage).toBe('calendar-days');
		expect(capacity.capacitySeconds).toBe(7 * 24 * hour);
	});

	test('starts autonomous work at reference-hour capacity and has no target marker', () => {
		const capacity = calculateWorkStandardCapacity({
			actualSeconds: hour,
			provisionalSeconds: 30 * 60,
			targetSeconds: 0,
			workingCapacitySeconds: 5 * 24 * hour,
			calendarCapacitySeconds: 7 * 24 * hour,
			referenceCapacitySeconds: 40 * hour,
			hasBaseline: false
		});

		expect(capacity.stage).toBe('reference-hours');
		expect(capacity.capacitySeconds).toBe(40 * hour);
		expect(capacity.targetPositionPercent).toBe(undefined);
		expect(capacity.actualWidthPercent).toBe(3.75);
	});

	test('selects reference-hours for 1h48m of work against a 5-working-day period at 480 reference minutes', () => {
		const capacity = calculateWorkStandardCapacity({
			actualSeconds: hour + 48 * 60,
			provisionalSeconds: 0,
			targetSeconds: 0,
			workingCapacitySeconds: 5 * 24 * hour,
			calendarCapacitySeconds: 7 * 24 * hour,
			referenceCapacitySeconds: 40 * hour,
			hasBaseline: false
		});

		expect(capacity.stage).toBe('reference-hours');
		expect(capacity.capacitySeconds).toBe(40 * hour);
		expect(capacity.actualWidthPercent).toBe(4.5);
	});

	test('autonomous escalates to working-days once actual exceeds reference capacity by one second', () => {
		const capacity = calculateWorkStandardCapacity({
			actualSeconds: 40 * hour + 1,
			provisionalSeconds: 0,
			targetSeconds: 0,
			workingCapacitySeconds: 5 * 24 * hour,
			calendarCapacitySeconds: 7 * 24 * hour,
			referenceCapacitySeconds: 40 * hour,
			hasBaseline: false
		});

		expect(capacity.stage).toBe('working-days');
		expect(capacity.capacitySeconds).toBe(5 * 24 * hour);
	});

	test('autonomous with zero reference capacity falls through to working-days', () => {
		const capacity = calculateWorkStandardCapacity({
			actualSeconds: hour,
			provisionalSeconds: 0,
			targetSeconds: 0,
			workingCapacitySeconds: 5 * 24 * hour,
			calendarCapacitySeconds: 7 * 24 * hour,
			referenceCapacitySeconds: 0,
			hasBaseline: false
		});

		expect(capacity.stage).toBe('working-days');
		expect(capacity.capacitySeconds).toBe(5 * 24 * hour);
	});

	test('autonomous with zero reference and zero working capacity falls through to calendar-days', () => {
		const capacity = calculateWorkStandardCapacity({
			actualSeconds: 1,
			provisionalSeconds: 0,
			targetSeconds: 0,
			workingCapacitySeconds: 0,
			calendarCapacitySeconds: 24 * hour,
			referenceCapacitySeconds: 0,
			hasBaseline: false
		});

		expect(capacity.stage).toBe('calendar-days');
		expect(capacity.capacitySeconds).toBe(24 * hour);
	});

	test('hasBaseline true never selects reference-hours even when reference capacity is supplied', () => {
		const capacity = calculateWorkStandardCapacity({
			actualSeconds: hour,
			provisionalSeconds: 0,
			targetSeconds: 0,
			workingCapacitySeconds: 5 * 24 * hour,
			calendarCapacitySeconds: 7 * 24 * hour,
			referenceCapacitySeconds: 40 * hour,
			hasBaseline: true
		});

		expect(capacity.stage).not.toBe('reference-hours');
	});

	test('uses calendar capacity when the period has no working date', () => {
		const capacity = calculateWorkStandardCapacity({
			actualSeconds: 1,
			provisionalSeconds: 0,
			targetSeconds: 0,
			workingCapacitySeconds: 0,
			calendarCapacitySeconds: 24 * hour,
			referenceCapacitySeconds: 0,
			hasBaseline: false
		});

		expect(capacity.stage).toBe('calendar-days');
		expect(capacity.capacitySeconds).toBe(24 * hour);
	});
});
