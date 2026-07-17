import { expect, test } from 'bun:test';
import { buildAttendanceSummaryFixture } from '../../../dev-attendance-summary-fixture';
import { currentMonthInTimeZone } from '../../../src/routes/attendance/shared/attendance-date';
import { attendanceText } from '../../../src/routes/attendance/text';

type DeferredResponse = {
	promise: Promise<Response>;
	resolve: (response: Response) => void;
};

function createDeferredResponse(): DeferredResponse {
	let resolvePromise: ((response: Response) => void) | undefined;
	const promise = new Promise<Response>((resolve) => {
		resolvePromise = resolve;
	});
	if (!resolvePromise) throw new Error('Expected response controls to initialize');
	return { promise, resolve: resolvePromise };
}

test('keeps loading while a newer concurrent summary request is pending', async () => {
	const originalState = Reflect.get(globalThis, '$state');
	const originalFetch = globalThis.fetch;
	const currentMonth = currentMonthInTimeZone('Asia/Seoul');
	const olderRequest = createDeferredResponse();
	const newerRequest = createDeferredResponse();
	let requestCount = 0;
	Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
	globalThis.fetch = Object.assign(
		async () => {
			requestCount += 1;
			return requestCount === 1 ? olderRequest.promise : newerRequest.promise;
		},
		{ preconnect: originalFetch.preconnect }
	);

	try {
		const { AttendanceState } = await import('../../../src/routes/attendance/attendance-context.svelte');
		const attendance = new AttendanceState(attendanceText.ko.loadFailed);
		const olderLoad = attendance.load();
		const newerLoad = attendance.load();
		const summary = buildAttendanceSummaryFixture(currentMonth);

		olderRequest.resolve(Response.json(summary));
		await olderLoad;
		expect(attendance.isLoading).toBe(true);

		newerRequest.resolve(Response.json(summary));
		await newerLoad;
		expect(attendance.isLoading).toBe(false);
	} finally {
		globalThis.fetch = originalFetch;
		if (originalState === undefined) {
			Reflect.deleteProperty(globalThis, '$state');
		} else {
			Reflect.set(globalThis, '$state', originalState);
		}
	}
});

for (const scenario of [
	{
		name: 'successful',
		createRefreshResponse: (month: string) =>
			Response.json({
				...buildAttendanceSummaryFixture(month),
				serverTime: `${month}-15T16:00:00+09:00`,
				timeZone: 'Asia/Seoul',
				timeZoneAuthoritative: true
			}),
		expectedTimeZone: 'Asia/Seoul'
	},
	{
		name: 'failed',
		createRefreshResponse: () => new Response('unavailable', { status: 503 }),
		expectedTimeZone: 'America/Los_Angeles'
	}
]) {
	test(`keeps the initial summary when a later background refresh is ${scenario.name}`, async () => {
		const originalState = Reflect.get(globalThis, '$state');
		const originalFetch = globalThis.fetch;
		const currentMonth = currentMonthInTimeZone('Asia/Seoul');
		const loadRequest = createDeferredResponse();
		const refreshRequest = createDeferredResponse();
		let requestCount = 0;
		Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
		globalThis.fetch = Object.assign(
			async () => {
				requestCount += 1;
				return requestCount === 1 ? loadRequest.promise : refreshRequest.promise;
			},
			{ preconnect: originalFetch.preconnect }
		);

		try {
			const { AttendanceState } = await import(
				'../../../src/routes/attendance/attendance-context.svelte'
			);
			const attendance = new AttendanceState(attendanceText.ko.loadFailed);
			const load = attendance.load();
			const refresh = attendance.refreshServerClock();
			refreshRequest.resolve(scenario.createRefreshResponse(currentMonth));
			await refresh;
			loadRequest.resolve(
				Response.json({
					...buildAttendanceSummaryFixture(currentMonth),
					serverTime: `${currentMonth}-15T15:00:00-07:00`,
					timeZone: 'America/Los_Angeles',
					timeZoneAuthoritative: true
				})
			);
			await load;

			expect(attendance.summary?.month).toBe(currentMonth);
			expect(attendance.summary?.timeZone).toBe(scenario.expectedTimeZone);
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
}

test('surfaces a month-change load failure after a successful background refresh', async () => {
	const originalState = Reflect.get(globalThis, '$state');
	const originalFetch = globalThis.fetch;
	const currentMonth = currentMonthInTimeZone('Asia/Seoul');
	const nextMonth = currentMonth.endsWith('-12')
		? `${Number(currentMonth.slice(0, 4)) + 1}-01`
		: `${currentMonth.slice(0, 5)}${String(Number(currentMonth.slice(5)) + 1).padStart(2, '0')}`;
	const nextMonthLoadRequest = createDeferredResponse();
	const nextMonthRefreshRequest = createDeferredResponse();
	let requestCount = 0;
	Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
	globalThis.fetch = Object.assign(
		async () => {
			requestCount += 1;
			if (requestCount === 1) {
				return Response.json(buildAttendanceSummaryFixture(currentMonth));
			}
			return requestCount === 2 ? nextMonthLoadRequest.promise : nextMonthRefreshRequest.promise;
		},
		{ preconnect: originalFetch.preconnect }
	);

	try {
		const { AttendanceState } = await import('../../../src/routes/attendance/attendance-context.svelte');
		const attendance = new AttendanceState(attendanceText.ko.loadFailed);
		await attendance.load();
		attendance.selectedMonth = nextMonth;
		const nextMonthLoad = attendance.load();
		const nextMonthRefresh = attendance.refreshServerClock();
		nextMonthRefreshRequest.resolve(
			Response.json({
				...buildAttendanceSummaryFixture(nextMonth),
				serverTime: `${nextMonth}-15T16:00:00+09:00`
			})
		);
		await nextMonthRefresh;
		nextMonthLoadRequest.resolve(new Response('next month unavailable', { status: 503 }));
		await nextMonthLoad;

		expect(attendance.summary).toBe(null);
		expect(attendance.errorMessage).not.toBe('');
	} finally {
		globalThis.fetch = originalFetch;
		if (originalState === undefined) {
			Reflect.deleteProperty(globalThis, '$state');
		} else {
			Reflect.set(globalThis, '$state', originalState);
		}
	}
});

test('surfaces a current-month load failure after a past-month auxiliary snapshot', async () => {
	const originalState = Reflect.get(globalThis, '$state');
	const originalFetch = globalThis.fetch;
	const currentMonth = currentMonthInTimeZone('Asia/Seoul');
	const previousMonth = currentMonth.endsWith('-01')
		? `${Number(currentMonth.slice(0, 4)) - 1}-12`
		: `${currentMonth.slice(0, 5)}${String(Number(currentMonth.slice(5)) - 1).padStart(2, '0')}`;
	const currentMonthLoadRequest = createDeferredResponse();
	const currentMonthRefreshRequest = createDeferredResponse();
	let requestCount = 0;
	Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
	globalThis.fetch = Object.assign(
		async () => {
			requestCount += 1;
			if (requestCount === 1) {
				return Response.json(buildAttendanceSummaryFixture(previousMonth));
			}
			if (requestCount === 2) {
				return Response.json(buildAttendanceSummaryFixture(currentMonth));
			}
			return requestCount === 3
				? currentMonthLoadRequest.promise
				: currentMonthRefreshRequest.promise;
		},
		{ preconnect: originalFetch.preconnect }
	);

	try {
		const { AttendanceState } = await import('../../../src/routes/attendance/attendance-context.svelte');
		const attendance = new AttendanceState(attendanceText.ko.loadFailed);
		attendance.selectedMonth = previousMonth;
		await attendance.load();
		attendance.selectedMonth = currentMonth;
		const currentMonthLoad = attendance.load();
		const currentMonthRefresh = attendance.refreshServerClock();
		currentMonthRefreshRequest.resolve(
			Response.json({
				...buildAttendanceSummaryFixture(currentMonth),
				serverTime: `${currentMonth}-15T16:00:00+09:00`
			})
		);
		await currentMonthRefresh;
		currentMonthLoadRequest.resolve(new Response('current month unavailable', { status: 503 }));
		await currentMonthLoad;

		expect(attendance.summary).toBe(null);
		expect(attendance.errorMessage).not.toBe('');
	} finally {
		globalThis.fetch = originalFetch;
		if (originalState === undefined) {
			Reflect.deleteProperty(globalThis, '$state');
		} else {
			Reflect.set(globalThis, '$state', originalState);
		}
	}
});

test('keeps the newer current-month summary when an older auxiliary request finishes late', async () => {
	const originalState = Reflect.get(globalThis, '$state');
	const originalFetch = globalThis.fetch;
	const currentMonth = currentMonthInTimeZone('Asia/Seoul');
	const previousMonth = currentMonth.endsWith('-01')
		? `${Number(currentMonth.slice(0, 4)) - 1}-12`
		: `${currentMonth.slice(0, 5)}${String(Number(currentMonth.slice(5)) - 1).padStart(2, '0')}`;
	const auxiliaryRequest = createDeferredResponse();
	const currentMonthRequest = createDeferredResponse();
	let resolveAuxiliaryStarted: (() => void) | undefined;
	const auxiliaryStarted = new Promise<void>((resolve) => {
		resolveAuxiliaryStarted = resolve;
	});
	let requestCount = 0;
	Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
	globalThis.fetch = Object.assign(
		async () => {
			requestCount += 1;
			if (requestCount === 1) {
				return Response.json(buildAttendanceSummaryFixture(previousMonth));
			}
			if (requestCount === 2) {
				resolveAuxiliaryStarted?.();
				return auxiliaryRequest.promise;
			}
			return currentMonthRequest.promise;
		},
		{ preconnect: originalFetch.preconnect }
	);

	try {
		const { AttendanceState } = await import('../../../src/routes/attendance/attendance-context.svelte');
		const attendance = new AttendanceState(attendanceText.ko.loadFailed);
		attendance.selectedMonth = previousMonth;
		const previousMonthLoad = attendance.load();
		await auxiliaryStarted;

		attendance.selectedMonth = currentMonth;
		const currentMonthLoad = attendance.load();
		currentMonthRequest.resolve(
			Response.json({
				...buildAttendanceSummaryFixture(currentMonth),
				todayStatus: 'newer primary'
			})
		);
		await currentMonthLoad;
		auxiliaryRequest.resolve(
			Response.json({
				...buildAttendanceSummaryFixture(currentMonth),
				todayStatus: 'older auxiliary'
			})
		);
		await previousMonthLoad;

		expect(attendance.currentMonthSummary?.todayStatus).toBe('newer primary');
	} finally {
		globalThis.fetch = originalFetch;
		if (originalState === undefined) {
			Reflect.deleteProperty(globalThis, '$state');
		} else {
			Reflect.set(globalThis, '$state', originalState);
		}
	}
});

test('does not restore an older auxiliary summary after a newer current-month load fails', async () => {
	const originalState = Reflect.get(globalThis, '$state');
	const originalFetch = globalThis.fetch;
	const currentMonth = currentMonthInTimeZone('Asia/Seoul');
	const previousMonth = currentMonth.endsWith('-01')
		? `${Number(currentMonth.slice(0, 4)) - 1}-12`
		: `${currentMonth.slice(0, 5)}${String(Number(currentMonth.slice(5)) - 1).padStart(2, '0')}`;
	const auxiliaryRequest = createDeferredResponse();
	let resolveAuxiliaryStarted: (() => void) | undefined;
	const auxiliaryStarted = new Promise<void>((resolve) => {
		resolveAuxiliaryStarted = resolve;
	});
	let requestCount = 0;
	Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
	globalThis.fetch = Object.assign(
		async () => {
			requestCount += 1;
			if (requestCount === 1) {
				return Response.json(buildAttendanceSummaryFixture(previousMonth));
			}
			if (requestCount === 2) {
				resolveAuxiliaryStarted?.();
				return auxiliaryRequest.promise;
			}
			return new Response('current month unavailable', { status: 503 });
		},
		{ preconnect: originalFetch.preconnect }
	);

	try {
		const { AttendanceState } = await import('../../../src/routes/attendance/attendance-context.svelte');
		const attendance = new AttendanceState(attendanceText.ko.loadFailed);
		attendance.selectedMonth = previousMonth;
		const previousMonthLoad = attendance.load();
		await auxiliaryStarted;

		attendance.selectedMonth = currentMonth;
		await attendance.load();
		auxiliaryRequest.resolve(
			Response.json({
				...buildAttendanceSummaryFixture(currentMonth),
				todayStatus: 'older auxiliary'
			})
		);
		await previousMonthLoad;

		expect(attendance.summary).toBe(null);
		expect(attendance.currentMonthSummary).toBe(null);
		expect(attendance.errorMessage).not.toBe('');
	} finally {
		globalThis.fetch = originalFetch;
		if (originalState === undefined) {
			Reflect.deleteProperty(globalThis, '$state');
		} else {
			Reflect.set(globalThis, '$state', originalState);
		}
	}
});
