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

test('schedules the next minute from the provided clock', () => {
	jest.useFakeTimers();
	jest.setSystemTime(new Date('2026-07-15T06:00:01.750Z'));
	const emittedTimes: Date[] = [];
	let providedTimeIndex = 0;
	const getCurrentTime = () =>
		providedTimeIndex++ === 0
			? new Date('2026-07-15T06:00:00.000Z')
			: new Date('2026-07-15T06:01:00.000Z');
	const stopClock = startAttendanceMinuteClock(
		(currentTime) => emittedTimes.push(currentTime),
		getCurrentTime
	);

	expect(emittedTimes.map((time) => time.toISOString())).toEqual(['2026-07-15T06:00:00.000Z']);
	jest.advanceTimersByTime(59_999);
	expect(emittedTimes.length).toBe(1);
	jest.advanceTimersByTime(1);
	expect(emittedTimes.map((time) => time.toISOString())).toEqual([
		'2026-07-15T06:00:00.000Z',
		'2026-07-15T06:01:00.000Z'
	]);

	stopClock();
});

test('does not emit or schedule a tick when the provided clock is invalid', () => {
	jest.useFakeTimers();
	const emittedTimes: Date[] = [];
	let currentTimeRequestCount = 0;
	const stopClock = startAttendanceMinuteClock(
		(currentTime) => emittedTimes.push(currentTime),
		() => {
			currentTimeRequestCount += 1;
			return new Date(Number.NaN);
		}
	);

	expect(emittedTimes).toEqual([]);
	expect(currentTimeRequestCount).toBe(1);
	jest.advanceTimersByTime(60_000);
	expect(currentTimeRequestCount).toBe(1);

	stopClock();
});

test('stops when the provided clock becomes invalid', () => {
	jest.useFakeTimers();
	const emittedTimes: Date[] = [];
	let currentTimeRequestCount = 0;
	const stopClock = startAttendanceMinuteClock(
		(currentTime) => emittedTimes.push(currentTime),
		() => {
			currentTimeRequestCount += 1;
			return currentTimeRequestCount === 1
				? new Date('2026-07-15T06:00:30.000Z')
				: new Date(Number.NaN);
		}
	);

	expect(emittedTimes.map((time) => time.toISOString())).toEqual(['2026-07-15T06:00:30.000Z']);
	jest.advanceTimersByTime(30_000);
	expect(emittedTimes.map((time) => time.toISOString())).toEqual(['2026-07-15T06:00:30.000Z']);
	expect(currentTimeRequestCount).toBe(2);
	jest.advanceTimersByTime(60_000);
	expect(currentTimeRequestCount).toBe(2);

	stopClock();
});
