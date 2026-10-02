import type { SupabaseWorkStatusInputs } from '$lib/attendance/supabase-work-status';
import type { SupabaseWorkPolicy } from '$lib/attendance/supabase-work-policy';

const storageKeyPrefix = 'attendance.workStatusRows.';

type StoredWorkStatusInputs = Omit<SupabaseWorkStatusInputs, 'policiesByMember' | 'holidays' | 'now'> & {
	policiesByMember: [string, SupabaseWorkPolicy][];
	holidays: string[];
	now: string;
};

function storageKey(scope: string) {
	return `${storageKeyPrefix}${encodeURIComponent(scope)}`;
}

export function readCachedWorkStatusRows(scope: string): SupabaseWorkStatusInputs | undefined {
	if (typeof window === 'undefined') return undefined;
	try {
		const raw = window.localStorage.getItem(storageKey(scope));
		if (!raw) return undefined;
		const stored: Partial<StoredWorkStatusInputs> | null = JSON.parse(raw);
		if (!stored) return undefined;
		const { timeZone, members, me, attendance, leave, coveredDays, policiesByMember, holidays, now } = stored;
		if (!timeZone || !members || !attendance || !leave || !now) return undefined;
		if (!Array.isArray(coveredDays) || !Array.isArray(policiesByMember) || !Array.isArray(holidays)) return undefined;
		return {
			timeZone,
			members,
			me,
			attendance,
			leave,
			coveredDays,
			policiesByMember: new Map(policiesByMember),
			holidays: new Set(holidays),
			now: new Date(now)
		};
	} catch {
		return undefined;
	}
}

export function writeCachedWorkStatusRows(rows: SupabaseWorkStatusInputs, scope: string): void {
	if (typeof window === 'undefined') return;
	try {
		const stored: StoredWorkStatusInputs = {
			...rows,
			policiesByMember: [...rows.policiesByMember],
			holidays: [...rows.holidays],
			now: rows.now.toISOString()
		};
		window.localStorage.setItem(storageKey(scope), JSON.stringify(stored));
	} catch {
		return;
	}
}

export function clearCachedWorkStatusRows(): void {
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
