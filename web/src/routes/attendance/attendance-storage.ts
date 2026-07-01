import type { ChartMode } from './attendance-context.svelte';

const STORAGE_KEY = 'attendance.filters';

export type PersistedAttendanceFilters = {
	selectedMonth?: string;
	chartMode?: ChartMode;
};

export function readPersistedAttendanceFilters(): PersistedAttendanceFilters {
	if (typeof window === 'undefined') return {};
	try {
		const raw = window.localStorage.getItem(STORAGE_KEY);
		const parsed = raw ? (JSON.parse(raw) as PersistedAttendanceFilters) : {};
		return { chartMode: parsed.chartMode };
	} catch {
		return {};
	}
}

export function writePersistedAttendanceFilters(value: PersistedAttendanceFilters): void {
	if (typeof window === 'undefined') return;
	try {
		window.localStorage.setItem(STORAGE_KEY, JSON.stringify(value));
	} catch {
		return;
	}
}
