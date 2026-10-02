import { afterEach, beforeEach, describe, expect, mock, spyOn, test } from 'bun:test';
import * as leaveAPI from '../../../src/routes/attendance/leave/employee-leave-api';
import type { EmployeeLeavePayload } from '../../../src/routes/attendance/leave/employee-leave-types';
import { attendanceText } from '../../../src/routes/attendance/text';
import { EmployeeLeaveState } from '../../../src/routes/attendance/leave/employee-leave-state.svelte';

const payload: EmployeeLeavePayload = {
	balanceTrackingMode: 'managed', leaveTypes: [], requests: [],
	summary: { usedMilliDays: 0, reservedMilliDays: 0, availableMilliDays: 15000 }
};

function deferredPayload() {
	let resolve: ((value: EmployeeLeavePayload) => void) | undefined;
	const promise = new Promise<EmployeeLeavePayload>((done) => { resolve = done; });
	if (!resolve) throw new Error('payload gate was not initialized');
	return { promise, resolve };
}

describe('employee leave consumer loading', () => {
	let originalState: unknown;
	beforeEach(() => {
		originalState = Reflect.get(globalThis, '$state');
		Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
	});
	afterEach(() => {
		mock.restore();
		if (originalState === undefined) Reflect.deleteProperty(globalThis, '$state');
		else Reflect.set(globalThis, '$state', originalState);
	});

	test('concurrent visible consumers share one initial load and reuse its result', async () => {
		const pending = deferredPayload();
		const fetch = spyOn(leaveAPI, 'fetchEmployeeLeave').mockImplementation(() => pending.promise);
		const state = new EmployeeLeaveState(attendanceText.en.leave);
		expect(fetch).toHaveBeenCalledTimes(0);
		const first = state.ensureLoaded();
		const second = state.ensureLoaded();
		expect(first).toBe(second);
		expect(fetch).toHaveBeenCalledTimes(1);
		expect(state.isLoading).toBe(true);
		pending.resolve(payload);
		await first;
		await state.ensureLoaded();
		expect(fetch).toHaveBeenCalledTimes(1);
		expect(state.payload).toEqual(payload);
		expect(state.isLoading).toBe(false);
	});

	test('a failed initial load can be retried by a later consumer', async () => {
		const fetch = spyOn(leaveAPI, 'fetchEmployeeLeave').mockRejectedValueOnce(new Error('unavailable'));
		const state = new EmployeeLeaveState(attendanceText.en.leave);
		await state.ensureLoaded();
		expect(state.errorMessage).toBe(attendanceText.en.leave.loadFailed);
		fetch.mockResolvedValueOnce(payload);
		await state.ensureLoaded();
		expect(fetch).toHaveBeenCalledTimes(2);
		expect(state.errorMessage).toBe('');
		expect(state.payload).toEqual(payload);
	});

	test('an explicit refresh replaces cached balances and wins over an older initial request', async () => {
		const pending = deferredPayload();
		const refreshed = { ...payload, summary: { ...payload.summary, availableMilliDays: 14000 } };
		const fetch = spyOn(leaveAPI, 'fetchEmployeeLeave')
			.mockImplementationOnce(() => pending.promise)
			.mockResolvedValueOnce(refreshed);
		const state = new EmployeeLeaveState(attendanceText.en.leave);
		const initial = state.ensureLoaded();
		await state.load();
		pending.resolve(payload);
		await initial;
		expect(fetch).toHaveBeenCalledTimes(2);
		expect(state.payload).toEqual(refreshed);
	});
});
