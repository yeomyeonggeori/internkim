import type { AttendanceSummary } from '../../routes/attendance/attendance-context.svelte';

const storageKeyPrefix = 'attendance.today.';
const maximumAgeMilliseconds = 24 * 60 * 60 * 1000;

type StoredAttendanceToday = { savedAt: number; summary: AttendanceSummary };

function storageKey(scope: string): string {
	return `${storageKeyPrefix}${encodeURIComponent(scope)}`;
}

export function readStoredAttendanceToday(scope: string, now = Date.now()): StoredAttendanceToday | null {
	if (!scope || typeof window === 'undefined') return null;
	try {
		const raw = window.localStorage.getItem(storageKey(scope));
		if (!raw) return null;
		const stored: StoredAttendanceToday = JSON.parse(raw);
		const age = now - stored.savedAt;
		if (!Number.isFinite(age) || age < 0 || age > maximumAgeMilliseconds) return null;
		return stored;
	} catch {
		return null;
	}
}

export function storeAttendanceToday(scope: string, summary: AttendanceSummary, savedAt = Date.now()): void {
	if (!scope || typeof window === 'undefined') return;
	try {
		window.localStorage.setItem(storageKey(scope), JSON.stringify({ savedAt, summary }));
	} catch {
		return;
	}
}

export function forgetStoredAttendanceToday(): void {
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
