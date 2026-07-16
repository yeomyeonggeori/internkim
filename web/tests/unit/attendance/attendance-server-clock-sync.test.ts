import { describe, expect, test } from 'bun:test';
import {
	AttendanceServerClockSync,
	type AttendanceServerClockSyncSource
} from '../../../src/routes/attendance/attendance-server-clock-sync';
import type { AttendanceServerClock } from '../../../src/routes/attendance/shared/attendance-server-clock';

type Summary = AttendanceServerClockSyncSource & {
	month: string;
	timeZone?: string;
	timeZoneAuthoritative?: boolean;
};

type LoadedSummaryState = {
	month: string;
	serverTime?: unknown;
	timeZone?: string;
	timeZoneAuthoritative?: boolean;
};

type Deferred<Value> = {
	promise: Promise<Value>;
	resolve: (value: Value) => void;
	reject: (error: Error) => void;
};

function createDeferred<Value>(): Deferred<Value> {
	let resolvePromise: ((value: Value) => void) | undefined;
	let rejectPromise: ((error: Error) => void) | undefined;
	const promise = new Promise<Value>((resolve, reject) => {
		resolvePromise = resolve;
		rejectPromise = reject;
	});
	if (!resolvePromise || !rejectPromise) throw new Error('Expected promise controls to initialize');
	return { promise, resolve: resolvePromise, reject: rejectPromise };
}

describe('attendance server clock synchronization', () => {
	test('applies only the newest successful response using its completion timestamp', async () => {
		const olderRequest = createDeferred<Summary>();
		const newerRequest = createDeferred<Summary>();
		const requests = new Map([
			['older', olderRequest],
			['newer', newerRequest]
		]);
		const appliedClocks: (AttendanceServerClock | null)[] = [];
		const appliedSummaries: Summary[] = [];
		let monotonicTimestampMilliseconds = 1_000;
		const synchronization = new AttendanceServerClockSync<Summary>({
			requestSummary: (month) => {
				const request = requests.get(month);
				if (!request) throw new Error(`Unexpected month ${month}`);
				return request.promise;
			},
			applySummarySnapshot: (summary, serverClock) => {
				appliedClocks.push(serverClock);
				appliedSummaries.push(summary);
			},
			monotonicNow: () => monotonicTimestampMilliseconds
		});

		const olderLoad = synchronization.loadSummary('older', () => undefined);
		const newerLoad = synchronization.loadSummary('newer', () => undefined);
		monotonicTimestampMilliseconds = 2_000;
		newerRequest.resolve({
			month: 'newer',
			serverTime: '2026-07-15T16:00:00+09:00',
			timeZone: 'Asia/Seoul',
			timeZoneAuthoritative: true
		});
		const newerSummary = await newerLoad;
		monotonicTimestampMilliseconds = 3_000;
		olderRequest.resolve({
			month: 'older',
			serverTime: '2026-07-15T15:00:00+09:00',
			timeZone: 'America/Los_Angeles',
			timeZoneAuthoritative: false
		});
		const olderSummary = await olderLoad;

		expect(appliedClocks).toEqual([
			{
				serverTimestampMilliseconds: Date.parse('2026-07-15T16:00:00+09:00'),
				monotonicTimestampMilliseconds: 2_000
			}
		]);
		expect(appliedSummaries).toEqual([
			{
				month: 'newer',
				serverTime: '2026-07-15T16:00:00+09:00',
				timeZone: 'Asia/Seoul',
				timeZoneAuthoritative: true
			}
		]);
		expect(newerSummary?.month).toBe('newer');
		expect(olderSummary).toBe(null);
	});

	test('applies a loaded summary inside the acceptance gate before a newer refresh', async () => {
		const olderRequest = createDeferred<Summary>();
		const newerRequest = createDeferred<Summary>();
		let loadedSummary: LoadedSummaryState | null = null;
		let newerRefresh: Promise<boolean> | null = null;
		let synchronization: AttendanceServerClockSync<Summary>;
		synchronization = new AttendanceServerClockSync<Summary>({
			requestSummary: (month) => (month === 'older' ? olderRequest.promise : newerRequest.promise),
			applySummarySnapshot: (summary) => {
				if (loadedSummary) {
					loadedSummary.serverTime = summary.serverTime;
					loadedSummary.timeZone = summary.timeZone;
					loadedSummary.timeZoneAuthoritative = summary.timeZoneAuthoritative;
				}
				if (summary.month === 'older') newerRefresh = synchronization.refresh('newer');
			},
			monotonicNow: () => 1_000
		});

		const olderLoad = synchronization.loadSummary('older', (summary) => {
			loadedSummary = { ...summary };
		});
		olderRequest.resolve({
			month: 'older',
			serverTime: '2026-07-15T15:00:00+09:00',
			timeZone: 'America/Los_Angeles',
			timeZoneAuthoritative: false
		});
		await olderLoad;
		if (!newerRefresh) throw new Error('Expected the newer refresh to start');
		newerRequest.resolve({
			month: 'newer',
			serverTime: '2026-07-15T16:00:00+09:00',
			timeZone: 'Asia/Seoul',
			timeZoneAuthoritative: true
		});
		await newerRefresh;

		expect(loadedSummary).toEqual({
			month: 'older',
			serverTime: '2026-07-15T16:00:00+09:00',
			timeZone: 'Asia/Seoul',
			timeZoneAuthoritative: true
		});
	});

	test('ignores an older request failure after a newer summary is applied', async () => {
		const olderRequest = createDeferred<Summary>();
		const newerRequest = createDeferred<Summary>();
		const appliedSummaries: Summary[] = [];
		const synchronization = new AttendanceServerClockSync<Summary>({
			requestSummary: (month) => (month === 'older' ? olderRequest.promise : newerRequest.promise),
			applySummarySnapshot: (summary) => appliedSummaries.push(summary),
			monotonicNow: () => 1_000
		});

		const olderLoad = synchronization.loadSummary('older', () => undefined);
		const newerLoad = synchronization.loadSummary('newer', () => undefined);
		newerRequest.resolve({ month: 'newer', serverTime: '2026-07-15T16:00:00+09:00' });
		await newerLoad;
		olderRequest.reject(new Error('older request failed'));

		expect(await olderLoad).toBe(null);
		expect(appliedSummaries.map((summary) => summary.month)).toEqual(['newer']);
	});

	test('keeps a full load valid when a later background refresh fails', async () => {
		const loadRequest = createDeferred<Summary>();
		const refreshRequest = createDeferred<Summary>();
		let loadedMonth = '';
		const synchronization = new AttendanceServerClockSync<Summary>({
			requestSummary: (month) => (month === 'load' ? loadRequest.promise : refreshRequest.promise),
			applySummarySnapshot: () => undefined,
			monotonicNow: () => 1_000
		});

		const load = synchronization.loadSummary('load', (summary) => {
			loadedMonth = summary.month;
		});
		const refresh = synchronization.refresh('refresh');
		refreshRequest.reject(new Error('refresh unavailable'));
		expect(await refresh).toBe(false);
		loadRequest.resolve({ month: 'load', serverTime: '2026-07-15T15:00:00+09:00' });

		expect((await load)?.month).toBe('load');
		expect(loadedMonth).toBe('load');
	});

	test('loads full data while preserving a newer successful refresh snapshot', async () => {
		const loadRequest = createDeferred<Summary>();
		const refreshRequest = createDeferred<Summary>();
		let loadedSummary: LoadedSummaryState | null = null;
		const synchronization = new AttendanceServerClockSync<Summary>({
			requestSummary: (month) => (month === 'load' ? loadRequest.promise : refreshRequest.promise),
			applySummarySnapshot: (summary) => {
				if (!loadedSummary) return;
				loadedSummary.serverTime = summary.serverTime;
				loadedSummary.timeZone = summary.timeZone;
				loadedSummary.timeZoneAuthoritative = summary.timeZoneAuthoritative;
			},
			monotonicNow: () => 1_000
		});

		const load = synchronization.loadSummary('load', (summary) => {
			loadedSummary = { ...summary };
		});
		const refresh = synchronization.refresh('refresh');
		refreshRequest.resolve({
			month: 'refresh',
			serverTime: '2026-07-15T16:00:00+09:00',
			timeZone: 'Asia/Seoul',
			timeZoneAuthoritative: true
		});
		expect(await refresh).toBe(true);
		loadRequest.resolve({
			month: 'load',
			serverTime: '2026-07-15T15:00:00+09:00',
			timeZone: 'America/Los_Angeles',
			timeZoneAuthoritative: false
		});

		expect((await load)?.month).toBe('load');
		expect(loadedSummary).toEqual({
			month: 'load',
			serverTime: '2026-07-15T16:00:00+09:00',
			timeZone: 'Asia/Seoul',
			timeZoneAuthoritative: true
		});
	});

	test('surfaces a new month load failure after a successful background snapshot', async () => {
		const augustLoadRequest = createDeferred<Summary>();
		const augustRefreshRequest = createDeferred<Summary>();
		let requestCount = 0;
		const synchronization = new AttendanceServerClockSync<Summary>({
			requestSummary: async () => {
				requestCount += 1;
				if (requestCount === 1) {
					return { month: '2026-07', serverTime: '2026-07-15T15:00:00+09:00' };
				}
				return requestCount === 2 ? augustLoadRequest.promise : augustRefreshRequest.promise;
			},
			applySummarySnapshot: () => undefined,
			monotonicNow: () => 1_000
		});

		await synchronization.loadSummary('2026-07', () => undefined);
		const augustLoad = synchronization.loadSummary('2026-08', () => undefined);
		const augustRefresh = synchronization.refresh('2026-08');
		augustRefreshRequest.resolve({
			month: '2026-08',
			serverTime: '2026-08-15T15:00:00+09:00'
		});
		await augustRefresh;
		const loadError = new Error('August summary unavailable');
		augustLoadRequest.reject(loadError);

		await expect(augustLoad).rejects.toBe(loadError);
	});

	test('ignores a pending load after its target is invalidated', async () => {
		const request = createDeferred<Summary>();
		const appliedMonths: string[] = [];
		const synchronization = new AttendanceServerClockSync<Summary>({
			requestSummary: () => request.promise,
			applySummarySnapshot: () => undefined,
			monotonicNow: () => 1_000
		});
		const load = synchronization.loadSummary(
			'2026-07',
			(summary) => appliedMonths.push(summary.month),
			'currentMonthSummary'
		);

		synchronization.invalidateLoadTarget('currentMonthSummary');
		request.resolve({ month: '2026-07', serverTime: '2026-07-15T15:00:00+09:00' });

		expect(await load).toBe(null);
		expect(appliedMonths).toEqual([]);
	});

	test('applies null after successful missing and invalid clock responses', async () => {
		const summaries: Summary[] = [
			{ month: 'missing' },
			{ month: 'invalid', serverTime: 'invalid' }
		];
		const appliedClocks: (AttendanceServerClock | null)[] = [];
		const synchronization = new AttendanceServerClockSync<Summary>({
			requestSummary: async () => {
				const summary = summaries.shift();
				if (!summary) throw new Error('Expected another summary');
				return summary;
			},
			applySummarySnapshot: (_summary, serverClock) => appliedClocks.push(serverClock),
			monotonicNow: () => 1_000
		});

		await synchronization.loadSummary('missing', () => undefined);
		await synchronization.loadSummary('invalid', () => undefined);

		expect(appliedClocks).toEqual([null, null]);
	});

	test('coalesces concurrent background refresh requests', async () => {
		const request = createDeferred<Summary>();
		let requestCount = 0;
		const appliedClocks: (AttendanceServerClock | null)[] = [];
		const synchronization = new AttendanceServerClockSync<Summary>({
			requestSummary: () => {
				requestCount += 1;
				return request.promise;
			},
			applySummarySnapshot: (_summary, serverClock) => appliedClocks.push(serverClock),
			monotonicNow: () => 1_000
		});

		const firstRefresh = synchronization.refresh('2026-07');
		const secondRefresh = synchronization.refresh('2026-07');
		request.resolve({ month: '2026-07', serverTime: '2026-07-15T15:00:00+09:00' });

		expect(await Promise.all([firstRefresh, secondRefresh])).toEqual([true, true]);
		expect(requestCount).toBe(1);
		expect(appliedClocks.length).toBe(1);

		expect(await synchronization.refresh('2026-07')).toBe(true);
		expect(requestCount).toBe(2);
		expect(appliedClocks.length).toBe(2);
	});

	test('returns false without clearing the applied clock when a background refresh fails', async () => {
		const initialSummary: Summary = {
			month: '2026-07',
			serverTime: '2026-07-15T15:00:00+09:00'
		};
		let shouldFail = false;
		let appliedClock: AttendanceServerClock | null = null;
		const synchronization = new AttendanceServerClockSync<Summary>({
			requestSummary: async () => {
				if (shouldFail) throw new Error('unavailable');
				return initialSummary;
			},
			applySummarySnapshot: (_summary, serverClock) => {
				appliedClock = serverClock;
			},
			monotonicNow: () => 1_000
		});

		await synchronization.loadSummary('2026-07', () => undefined);
		const initialClock = appliedClock;
		shouldFail = true;

		expect(await synchronization.refresh('2026-07')).toBe(false);
		expect(appliedClock).toBe(initialClock);
	});

	test('returns normal summary responses', async () => {
		const summary: Summary = { month: '2026-07' };
		const synchronization = new AttendanceServerClockSync<Summary>({
			requestSummary: async () => summary,
			applySummarySnapshot: () => undefined,
			monotonicNow: () => 1_000
		});

		expect(await synchronization.loadSummary('2026-07', () => undefined)).toBe(summary);
	});

	test('propagates normal summary request failures', async () => {
		const requestError = new Error('summary unavailable');
		const synchronization = new AttendanceServerClockSync<Summary>({
			requestSummary: async () => {
				throw requestError;
			},
			applySummarySnapshot: () => undefined,
			monotonicNow: () => 1_000
		});

		await expect(synchronization.loadSummary('2026-07', () => undefined)).rejects.toBe(requestError);
	});
});
