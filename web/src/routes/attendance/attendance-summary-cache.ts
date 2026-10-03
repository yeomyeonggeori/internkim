import type { AttendanceSummary } from './attendance-context.svelte';
import { invalidateAttendanceCacheGeneration } from './attendance-cache-generation';

const storageKeyPrefix = 'attendance.summary.';

function storageKey(month: string, scope = '') {
	return `${storageKeyPrefix}${scope ? `${encodeURIComponent(scope)}.` : ''}${month || 'current'}`;
}

export function readCachedAttendanceSummary(month: string, scope = ''): AttendanceSummary | null {
	if (typeof window === 'undefined') return null;
	try {
		const raw = window.localStorage.getItem(storageKey(month, scope));
		if (!raw) return null;
		const summary: AttendanceSummary | null = JSON.parse(raw);
		return summary?.month ? summary : null;
	} catch {
		return null;
	}
}

export function writeCachedAttendanceSummary(month: string, summary: AttendanceSummary, scope = ''): void {
	if (typeof window === 'undefined') return;
	try {
		const serialized = JSON.stringify(summary);
		window.localStorage.setItem(storageKey(month, scope), serialized);
		if (month !== summary.month) window.localStorage.setItem(storageKey(summary.month, scope), serialized);
	} catch {
		return;
	}
}

export function clearCachedAttendanceSummaries(): void {
	invalidateAttendanceCacheGeneration();
	if (typeof window === 'undefined') return;
	try {
		const storage = window.localStorage;
		const keys = Array.from({ length: storage.length }, (_, index) => storage.key(index)).filter(
			(key): key is string => Boolean(key?.startsWith(storageKeyPrefix))
		);
		for (const key of keys) storage.removeItem(key);
	} catch {
		return;
	}
}
