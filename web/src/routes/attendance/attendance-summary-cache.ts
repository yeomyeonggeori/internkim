import type { AttendanceSummary } from './attendance-context.svelte';

const storageKeyPrefix = 'attendance.summary.';
const freshMilliseconds = 5 * 60 * 1000;

type CachedAttendanceSummary = {
	summary: AttendanceSummary;
	cachedAt: number;
};

function storageKey(month: string, scope = '') {
	return `${storageKeyPrefix}${scope ? `${encodeURIComponent(scope)}.` : ''}${month || 'current'}`;
}

export function readCachedAttendanceSummary(month: string, scope = ''): AttendanceSummary | null {
	if (typeof window === 'undefined') return null;
	try {
		const raw = window.sessionStorage.getItem(storageKey(month, scope));
		if (!raw) return null;
		const cached = JSON.parse(raw) as CachedAttendanceSummary;
		if (!cached.summary || Date.now() - cached.cachedAt > freshMilliseconds) return null;
		return cached.summary;
	} catch {
		return null;
	}
}

export function writeCachedAttendanceSummary(month: string, summary: AttendanceSummary, scope = ''): void {
	if (typeof window === 'undefined') return;
	try {
		const cached: CachedAttendanceSummary = { summary, cachedAt: Date.now() };
		window.sessionStorage.setItem(storageKey(month, scope), JSON.stringify(cached));
		if (month !== summary.month) window.sessionStorage.setItem(storageKey(summary.month, scope), JSON.stringify(cached));
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
