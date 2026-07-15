import { afterEach, expect, jest, test } from 'bun:test';
import { startAttendanceMinuteClock } from '../../../src/routes/attendance/shared/attendance-minute-clock';

afterEach(() => {
	jest.useRealTimers();
});

test('emits immediately and again at the next minute', () => {
	jest.useFakeTimers();
	jest.setSystemTime(new Date('2026-07-15T06:00:30.000Z'));
	const emittedTimes: Date[] = [];
	const stopClock = startAttendanceMinuteClock((currentTime) => emittedTimes.push(currentTime));

	expect(emittedTimes.map((time) => time.toISOString())).toEqual(['2026-07-15T06:00:30.000Z']);
	jest.advanceTimersByTime(29_999);
	expect(emittedTimes.length).toBe(1);
	jest.advanceTimersByTime(1);
	expect(emittedTimes.length).toBe(2);

	stopClock();
});

test('stops emitting after cleanup', () => {
	jest.useFakeTimers();
	jest.setSystemTime(new Date('2026-07-15T06:00:30.000Z'));
	const emittedTimes: Date[] = [];
	const stopClock = startAttendanceMinuteClock((currentTime) => emittedTimes.push(currentTime));

	stopClock();
	jest.advanceTimersByTime(90_000);
	expect(emittedTimes.map((time) => time.toISOString())).toEqual(['2026-07-15T06:00:30.000Z']);
});

test('stops emitting after cleanup following a rescheduled tick', () => {
	jest.useFakeTimers();
	jest.setSystemTime(new Date('2026-07-15T06:00:30.000Z'));
	const emittedTimes: Date[] = [];
	const stopClock = startAttendanceMinuteClock((currentTime) => emittedTimes.push(currentTime));

	jest.advanceTimersByTime(30_000);
	expect(emittedTimes.length).toBe(2);

	stopClock();
	jest.advanceTimersByTime(180_000);
	expect(emittedTimes.length).toBe(2);
});
