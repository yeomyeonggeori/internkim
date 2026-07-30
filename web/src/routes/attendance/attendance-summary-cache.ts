import type { AttendanceSummary } from './attendance-context.svelte';

const storageKeyPrefix = 'attendance.summary.';
const freshMilliseconds = 5 * 60 * 1000;

type CachedAttendanceSummary = {
	summary: AttendanceSummary;
	cachedAt: number;
};

function storageKey(month: string) {
	return `${storageKeyPrefix}${month || 'current'}`;
}

export function readCachedAttendanceSummary(month: string): AttendanceSummary | null {
	if (typeof window === 'undefined') return null;
	try {
		const raw = window.sessionStorage.getItem(storageKey(month));
		if (!raw) return null;
		const cached = JSON.parse(raw) as CachedAttendanceSummary;
		if (!cached.summary || Date.now() - cached.cachedAt > freshMilliseconds) return null;
		return cached.summary;
	} catch {
		return null;
	}
}

export function writeCachedAttendanceSummary(month: string, summary: AttendanceSummary): void {
	if (typeof window === 'undefined') return;
	try {
		const cached: CachedAttendanceSummary = { summary, cachedAt: Date.now() };
		window.sessionStorage.setItem(storageKey(month), JSON.stringify(cached));
		if (month !== summary.month) window.sessionStorage.setItem(storageKey(summary.month), JSON.stringify(cached));
	} catch {
		return;
	}
}

export function clearCachedAttendanceSummaries(): void {
	if (typeof window === 'undefined') return;
	try {
		const storage = window.sessionStorage;
		const keys = Array.from({ length: storage.length }, (_, index) => storage.key(index)).filter(
			(key): key is string => Boolean(key?.startsWith(storageKeyPrefix))
		);
		for (const key of keys) window.sessionStorage.removeItem(key);
	} catch {
		return;
	}
}
