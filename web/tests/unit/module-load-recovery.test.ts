import { describe, expect, test } from 'bun:test';

import { claimModuleLoadRetry, isModuleLoadError, scheduleModuleLoadRecovery } from '../../src/lib/module-load-recovery';

class TestStorage {
	private readonly values = new Map<string, string>();

	getItem(key: string) {
		return this.values.get(key) ?? null;
	}

	setItem(key: string, value: string) {
		this.values.set(key, value);
	}
}

describe('module load recovery', () => {
	test('detects browser module import failures', () => {
		expect(isModuleLoadError(new TypeError('Importing a module script failed.'))).toBe(true);
		expect(isModuleLoadError(new TypeError('Failed to fetch dynamically imported module: /_app/immutable/nodes/12.js'))).toBe(true);
		expect(isModuleLoadError(new Error('Unable to preload CSS for /_app/immutable/assets/12.css'))).toBe(true);
		expect(isModuleLoadError(new Error('메일 계정을 불러오지 못했습니다'))).toBe(false);
	});

	test('claims a single retry for the same path inside the retry window', () => {
		const storage = new TestStorage();

		expect(claimModuleLoadRetry('/mail', storage, 1_000)).toBe(true);
		expect(claimModuleLoadRetry('/mail', storage, 30_000)).toBe(false);
		expect(claimModuleLoadRetry('/attendance', storage, 30_000)).toBe(true);
		expect(claimModuleLoadRetry('/attendance', storage, 91_000)).toBe(true);
	});

	test('schedules a reload for recoverable module import failures', () => {
		const storage = new TestStorage();
		let reloadCount = 0;
		let scheduledCount = 0;
		const environment = {
			currentPath: '/attendance',
			now: () => 10,
			reload: () => {
				reloadCount += 1;
			},
			schedule: (callback: () => void) => {
				scheduledCount += 1;
				callback();
			},
			storage
		};

		expect(scheduleModuleLoadRecovery(new TypeError('Importing a module script failed.'), '', environment)).toBe(true);
		expect(scheduleModuleLoadRecovery(new TypeError('Importing a module script failed.'), '', environment)).toBe(false);
		expect(scheduleModuleLoadRecovery(new Error('ordinary failure'), '', environment)).toBe(false);
		expect(reloadCount).toBe(1);
		expect(scheduledCount).toBe(1);
	});
});
