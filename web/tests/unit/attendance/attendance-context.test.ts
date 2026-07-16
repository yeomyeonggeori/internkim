import { expect, test } from 'bun:test';
import { buildAttendanceSummaryFixture } from '../../../dev-attendance-summary-fixture';
import { currentMonthInTimeZone } from '../../../src/routes/attendance/shared/attendance-date';
import { attendanceText } from '../../../src/routes/attendance/text';

const currentMonth = currentMonthInTimeZone('Asia/Seoul');
const firstServerTime = `${currentMonth}-15T15:00:00+09:00`;
const secondServerTime = `${currentMonth}-15T16:00:00+09:00`;

test('loads a legacy attendance summary without a server clock', async () => {
	const originalState = Reflect.get(globalThis, '$state');
	const originalFetch = globalThis.fetch;
	Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
	globalThis.fetch = Object.assign(
		async () => {
			const { serverTime: _serverTime, ...legacySummary } = buildAttendanceSummaryFixture(currentMonth);
			return Response.json(legacySummary);
		},
		{ preconnect: originalFetch.preconnect }
	);

	try {
		const { AttendanceState } = await import('../../../src/routes/attendance/attendance-context.svelte');
		const attendance = new AttendanceState(attendanceText.ko.loadFailed);

		await attendance.load();

		expect(attendance.summary?.month).toBe(currentMonth);
		expect(attendance.currentMonthSummary?.month).toBe(currentMonth);
		expect(attendance.serverClock).toBe(null);
		expect(attendance.errorMessage).toBe('');
	} finally {
		globalThis.fetch = originalFetch;
		if (originalState === undefined) {
			Reflect.deleteProperty(globalThis, '$state');
		} else {
			Reflect.set(globalThis, '$state', originalState);
		}
	}
});

test('recovers the server clock after loading a legacy attendance summary', async () => {
	const originalState = Reflect.get(globalThis, '$state');
	const originalFetch = globalThis.fetch;
	let requestCount = 0;
	Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
	globalThis.fetch = Object.assign(
		async () => {
			requestCount += 1;
			const summary = buildAttendanceSummaryFixture(currentMonth);
			if (requestCount > 1) return Response.json({ ...summary, serverTime: secondServerTime });
			const { serverTime: _serverTime, ...legacySummary } = summary;
			return Response.json(legacySummary);
		},
		{ preconnect: originalFetch.preconnect }
	);

	try {
		const { AttendanceState } = await import('../../../src/routes/attendance/attendance-context.svelte');
		const attendance = new AttendanceState(attendanceText.ko.loadFailed);

		await attendance.load();
		expect(attendance.serverClock).toBe(null);
		expect(await attendance.refreshServerClock()).toBe(true);
		const recoveredClock = attendance.serverClock;
		if (!recoveredClock) throw new Error('Expected the attendance server clock to recover');

		expect(requestCount).toBe(2);
		expect(attendance.currentServerTime(recoveredClock.monotonicTimestampMilliseconds).toISOString()).toBe(
			new Date(secondServerTime).toISOString()
		);
	} finally {
		globalThis.fetch = originalFetch;
		if (originalState === undefined) {
			Reflect.deleteProperty(globalThis, '$state');
		} else {
			Reflect.set(globalThis, '$state', originalState);
		}
	}
});

test('refreshes the server clock without replacing the loaded attendance summary', async () => {
	const originalState = Reflect.get(globalThis, '$state');
	const originalFetch = globalThis.fetch;
	let requestCount = 0;
	Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
	globalThis.fetch = Object.assign(
		async () => {
			requestCount += 1;
			return Response.json({
				...buildAttendanceSummaryFixture(currentMonth),
				serverTime: requestCount === 1 ? firstServerTime : secondServerTime,
				timeZone: requestCount === 1 ? 'Asia/Seoul' : 'America/Los_Angeles',
				timeZoneAuthoritative: requestCount === 1
			});
		},
		{ preconnect: originalFetch.preconnect }
	);

	try {
		const { AttendanceState } = await import('../../../src/routes/attendance/attendance-context.svelte');
		const attendance = new AttendanceState(attendanceText.ko.loadFailed);

		await attendance.load();
		const loadedSummary = attendance.summary;
		await attendance.refreshServerClock();
		const refreshedClock = attendance.serverClock;
		if (!refreshedClock) throw new Error('Expected the attendance server clock to be refreshed');

		expect(requestCount).toBe(2);
		expect(attendance.summary).toBe(loadedSummary);
		expect(attendance.summary?.timeZone).toBe('America/Los_Angeles');
		expect(attendance.summary?.timeZoneAuthoritative).toBe(false);
		expect(attendance.currentServerTime(refreshedClock.monotonicTimestampMilliseconds).toISOString()).toBe(
			new Date(secondServerTime).toISOString()
		);
	} finally {
		globalThis.fetch = originalFetch;
		if (originalState === undefined) {
			Reflect.deleteProperty(globalThis, '$state');
		} else {
			Reflect.set(globalThis, '$state', originalState);
		}
	}
});

test('keeps the existing server clock when a background refresh fails', async () => {
	const originalState = Reflect.get(globalThis, '$state');
	const originalFetch = globalThis.fetch;
	let requestCount = 0;
	Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
	globalThis.fetch = Object.assign(
		async () => {
			requestCount += 1;
			if (requestCount > 1) return new Response('unavailable', { status: 503 });
			return Response.json({
				...buildAttendanceSummaryFixture(currentMonth),
				serverTime: firstServerTime
			});
		},
		{ preconnect: originalFetch.preconnect }
	);

	try {
		const { AttendanceState } = await import('../../../src/routes/attendance/attendance-context.svelte');
		const attendance = new AttendanceState(attendanceText.ko.loadFailed);

		await attendance.load();
		const loadedSummary = attendance.summary;
		const loadedClock = attendance.serverClock;
		await attendance.refreshServerClock();

		expect(requestCount).toBe(2);
		expect(attendance.summary).toBe(loadedSummary);
		expect(attendance.serverClock).toBe(loadedClock);
	} finally {
		globalThis.fetch = originalFetch;
		if (originalState === undefined) {
			Reflect.deleteProperty(globalThis, '$state');
		} else {
			Reflect.set(globalThis, '$state', originalState);
		}
	}
});

test('clears invalid server clocks and recovers on the next valid response', async () => {
	const originalState = Reflect.get(globalThis, '$state');
	const originalFetch = globalThis.fetch;
	let requestCount = 0;
	Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
	globalThis.fetch = Object.assign(
		async () => {
			requestCount += 1;
			const summary = buildAttendanceSummaryFixture(currentMonth);
			if (requestCount === 1) return Response.json({ ...summary, serverTime: firstServerTime });
			if (requestCount === 2) {
				const { serverTime: _serverTime, ...legacySummary } = summary;
				return Response.json(legacySummary);
			}
			if (requestCount === 3) return Response.json({ ...summary, serverTime: secondServerTime });
			return Response.json({ ...summary, serverTime: 'invalid' });
		},
		{ preconnect: originalFetch.preconnect }
	);

	try {
		const { AttendanceState } = await import('../../../src/routes/attendance/attendance-context.svelte');
		const attendance = new AttendanceState(attendanceText.ko.loadFailed);

		await attendance.load();
		expect(attendance.serverClock).not.toBe(null);
		expect(await attendance.refreshServerClock()).toBe(false);
		expect(attendance.serverClock).toBe(null);

		expect(await attendance.refreshServerClock()).toBe(true);
		expect(attendance.serverClock).not.toBe(null);

		expect(await attendance.refreshServerClock()).toBe(false);
		expect(requestCount).toBe(4);
		expect(attendance.serverClock).toBe(null);
	} finally {
		globalThis.fetch = originalFetch;
		if (originalState === undefined) {
			Reflect.deleteProperty(globalThis, '$state');
		} else {
			Reflect.set(globalThis, '$state', originalState);
		}
	}
});

test('coalesces concurrent server clock refresh requests', async () => {
	const originalState = Reflect.get(globalThis, '$state');
	const originalFetch = globalThis.fetch;
	let requestCount = 0;
	let releaseRefresh: (() => void) | undefined;
	Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
	globalThis.fetch = Object.assign(
		async () => {
			requestCount += 1;
			if (requestCount > 1) {
				await new Promise<void>((resolve) => {
					releaseRefresh = resolve;
				});
			}
			return Response.json({
				...buildAttendanceSummaryFixture(currentMonth),
				serverTime: firstServerTime
			});
		},
		{ preconnect: originalFetch.preconnect }
	);

	try {
		const { AttendanceState } = await import('../../../src/routes/attendance/attendance-context.svelte');
		const attendance = new AttendanceState(attendanceText.ko.loadFailed);

		await attendance.load();
		const firstRefresh = attendance.refreshServerClock();
		const secondRefresh = attendance.refreshServerClock();

		expect(requestCount).toBe(2);
		if (!releaseRefresh) throw new Error('Expected the background refresh request to start');
		releaseRefresh();
		await Promise.all([firstRefresh, secondRefresh]);
	} finally {
		globalThis.fetch = originalFetch;
		if (originalState === undefined) {
			Reflect.deleteProperty(globalThis, '$state');
		} else {
			Reflect.set(globalThis, '$state', originalState);
		}
	}
});

test('orders legacy and valid server clocks by request sequence', async () => {
	const originalState = Reflect.get(globalThis, '$state');
	const originalFetch = globalThis.fetch;
	let requestCount = 0;
	let releaseDelayedLoad: (() => void) | undefined;
	let releaseSecondDelayedLoad: (() => void) | undefined;
	Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
	globalThis.fetch = Object.assign(
		async () => {
			const requestNumber = ++requestCount;
			if (requestNumber === 2) {
				await new Promise<void>((resolve) => {
					releaseDelayedLoad = resolve;
				});
			}
			if (requestNumber === 4) {
				await new Promise<void>((resolve) => {
					releaseSecondDelayedLoad = resolve;
				});
				return new Response('unavailable', { status: 503 });
			}
			const summary = buildAttendanceSummaryFixture(currentMonth);
			if (requestNumber === 2) {
				const { serverTime: _serverTime, ...legacySummary } = summary;
				return Response.json({
					...legacySummary,
					timeZone: 'America/Los_Angeles',
					timeZoneAuthoritative: false
				});
			}
			const serverTime =
				requestNumber === 1
					? firstServerTime
					: requestNumber === 5
						? 'invalid'
						: secondServerTime;
			return Response.json({ ...summary, serverTime });
		},
		{ preconnect: originalFetch.preconnect }
	);

	try {
		const { AttendanceState } = await import('../../../src/routes/attendance/attendance-context.svelte');
		const attendance = new AttendanceState(attendanceText.ko.loadFailed);

		await attendance.load();
		const delayedLoad = attendance.load();
		if (!releaseDelayedLoad) throw new Error('Expected the delayed summary request to start');
		expect(await attendance.refreshServerClock()).toBe(true);
		releaseDelayedLoad();
		await delayedLoad;
		const finalClock = attendance.serverClock;
		if (!finalClock) throw new Error('Expected the attendance server clock to remain available');

		expect(requestCount).toBe(3);
		expect(attendance.currentServerTime(finalClock.monotonicTimestampMilliseconds).toISOString()).toBe(
			new Date(secondServerTime).toISOString()
		);
		expect(attendance.summary?.timeZone).toBe('Asia/Seoul');
		expect(attendance.summary?.timeZoneAuthoritative).toBe(true);

		const secondDelayedLoad = attendance.load();
		if (!releaseSecondDelayedLoad) throw new Error('Expected the second delayed summary request to start');
		expect(await attendance.refreshServerClock()).toBe(false);
		releaseSecondDelayedLoad();
		await secondDelayedLoad;

		expect(requestCount).toBe(5);
		expect(attendance.serverClock).toBe(null);
		expect(attendance.summary?.timeZone).toBe('Asia/Seoul');
		expect(attendance.errorMessage).toBe('');
	} finally {
		globalThis.fetch = originalFetch;
		if (originalState === undefined) {
			Reflect.deleteProperty(globalThis, '$state');
		} else {
			Reflect.set(globalThis, '$state', originalState);
		}
	}
});
