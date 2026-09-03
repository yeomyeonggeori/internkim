import { describe, expect, test } from 'bun:test';
import { adminApiFetch } from '../../src/lib/admin-api';
import {
	AdminApiError,
	apiErrorMessage,
	fetchCalendarHolidayStatus,
	refreshCalendarHolidayStatus
} from '../../src/routes/admin/admin-api';
import type { CalendarHolidayStatus } from '../../src/routes/admin/admin-types';

const originalFetch = globalThis.fetch;
const originalWindow = (globalThis as { window?: unknown }).window;

function restoreGlobals() {
	globalThis.fetch = originalFetch;
	(globalThis as { window?: unknown }).window = originalWindow;
}

function installWindow() {
	const store = new Map<string, string>();
	let reloadCount = 0;
	(globalThis as { window?: unknown }).window = {
		sessionStorage: {
			getItem: (key: string) => store.get(key) ?? null,
			setItem: (key: string, value: string) => void store.set(key, value)
		},
		location: { reload: () => void (reloadCount += 1) }
	};
	return () => reloadCount;
}

function stubFetch(response: unknown) {
	globalThis.fetch = (async () => response) as unknown as typeof fetch;
}

describe('adminApiFetch', () => {
	test('returns a normal response unchanged', async () => {
		try {
			const expected = { type: 'basic', ok: true } as unknown as Response;
			stubFetch(expected);
			const response = await adminApiFetch('/admin/api/session');
			expect(response).toBe(expected);
		} finally {
			restoreGlobals();
		}
	});

	test('triggers a reauthentication reload on a Cloudflare Access redirect', async () => {
		try {
			const reloadCount = installWindow();
			stubFetch({ type: 'opaqueredirect' });
			await expect(adminApiFetch('/admin/api/locale')).rejects.toThrow();
			expect(reloadCount()).toBe(1);
		} finally {
			restoreGlobals();
		}
	});

	test('does not reload twice within the cooldown window', async () => {
		try {
			const reloadCount = installWindow();
			stubFetch({ type: 'opaqueredirect' });
			await expect(adminApiFetch('/admin/api/locale')).rejects.toThrow();
			await expect(adminApiFetch('/admin/api/session')).rejects.toThrow();
			expect(reloadCount()).toBe(1);
		} finally {
			restoreGlobals();
		}
	});
});

describe('apiErrorMessage', () => {
	test('falls back instead of showing raw organization supervisor validation', () => {
		const error = new AdminApiError('supervisor hierarchy cannot contain cycles', 400);

		expect(apiErrorMessage(error, '사용자 저장에 실패했습니다.')).toBe('사용자 저장에 실패했습니다.');
	});

	test('falls back instead of showing raw network fetch failures', () => {
		expect(apiErrorMessage(new TypeError('Failed to fetch'), '사용자를 불러오지 못했습니다.')).toBe('사용자를 불러오지 못했습니다.');
		expect(apiErrorMessage(new TypeError('Load failed'), '사용자를 불러오지 못했습니다.')).toBe('사용자를 불러오지 못했습니다.');
	});
});

describe('calendar holiday status API', () => {
	test('loads status and posts a manual refresh', async () => {
		const status = { status: 'degraded', countryCode: 'KR', provider: 'nager', years: [] } satisfies CalendarHolidayStatus;
		const requests: { input: string; method?: string }[] = [];
		globalThis.fetch = (async (input, init) => {
			requests.push({ input: String(input), method: init?.method });
			return new Response(JSON.stringify(status));
		}) as typeof fetch;

		try {
			expect(await fetchCalendarHolidayStatus('/admin/api', 'load failed')).toEqual(status);
			expect(await refreshCalendarHolidayStatus('/admin/api', 'refresh failed')).toEqual(status);
			expect(requests).toEqual([
				{ input: '/admin/api/calendar-holidays/status', method: undefined },
				{ input: '/admin/api/calendar-holidays/refresh', method: 'POST' }
			]);
		} finally {
			restoreGlobals();
		}
	});
});
