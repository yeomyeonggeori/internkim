import { describe, expect, test } from 'bun:test';
import { adminApiFetch } from '../../src/lib/admin-api';
import {
	AdminApiError,
	apiErrorMessage,
	createCompanyHoliday,
	deleteCompanyHoliday,
	fetchAttendanceLeavePolicy,
	fetchCompanyHolidays,
	fetchAttendanceWorkPolicy,
	updateCompanyHoliday,
	updateAttendanceWorkPolicy,
	updateAttendanceLeavePolicy
} from '../../src/routes/admin/admin-api';

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

describe('attendance leave policy API', () => {
	test('uses the attendance leave policy GET endpoint', async () => {
		const policy = { version: 2, balanceTrackingMode: 'managed', fiscalYearStartMonth: 1, fiscalYearStartDay: 1, leaveTypes: [], updatedAt: '' };
		globalThis.fetch = (async (input) => { expect(input).toBe('/admin/api/attendance-leave-policy'); return new Response(JSON.stringify(policy)); }) as typeof fetch;
		expect(await fetchAttendanceLeavePolicy('/admin/api', 'load failed')).toEqual(policy);
		restoreGlobals();
	});

	test('sends the complete policy to the attendance leave policy PUT endpoint', async () => {
		const policy = { version: 2 as const, balanceTrackingMode: 'managed' as const, fiscalYearStartMonth: 1, fiscalYearStartDay: 1, leaveTypes: [], updatedAt: '' };
		globalThis.fetch = (async (input, init) => { expect(input).toBe('/admin/api/attendance-leave-policy'); expect(init?.method).toBe('PUT'); expect(init?.body).toBe(JSON.stringify(policy)); return new Response(JSON.stringify(policy)); }) as typeof fetch;
		expect(await updateAttendanceLeavePolicy('/admin/api', policy, 'save failed')).toEqual(policy);
		restoreGlobals();
	});
});

describe('company holiday API', () => {
	const holiday = {
		id: 'company-holiday-1',
		title: '창립기념일',
		date: '2026-09-18',
		recursAnnually: true,
		createdAt: '2026-07-31T00:00:00Z',
		updatedAt: '2026-07-31T00:00:00Z'
	};

	test('lists company holidays', async () => {
		globalThis.fetch = (async (input, init) => {
			expect(input).toBe('/admin/api/company-holidays');
			expect(init?.credentials).toBe('include');
			return new Response(JSON.stringify({ holidays: [holiday] }));
		}) as typeof fetch;

		expect(await fetchCompanyHolidays('/admin/api', 'load failed')).toEqual({
			holidays: [holiday]
		});
		restoreGlobals();
	});

	test('creates and updates company holidays', async () => {
		const input = { title: holiday.title, date: holiday.date, recursAnnually: true };
		let requestIndex = 0;
		globalThis.fetch = (async (url, init) => {
			if (requestIndex === 0) {
				expect(url).toBe('/admin/api/company-holidays');
				expect(init?.method).toBe('POST');
			} else {
				expect(url).toBe('/admin/api/company-holidays/company-holiday-1');
				expect(init?.method).toBe('PUT');
			}
			expect(init?.body).toBe(JSON.stringify(input));
			requestIndex += 1;
			return new Response(JSON.stringify(holiday));
		}) as typeof fetch;

		expect(await createCompanyHoliday('/admin/api', input, 'save failed')).toEqual(holiday);
		expect(
			await updateCompanyHoliday('/admin/api', holiday.id, input, 'save failed')
		).toEqual(holiday);
		restoreGlobals();
	});

	test('deletes company holidays', async () => {
		globalThis.fetch = (async (input, init) => {
			expect(input).toBe('/admin/api/company-holidays/company-holiday-1');
			expect(init?.method).toBe('DELETE');
			return new Response(null, { status: 204 });
		}) as typeof fetch;

		await deleteCompanyHoliday('/admin/api', holiday.id, 'delete failed');
		restoreGlobals();
	});
});

describe('attendance work policy API', () => {
	test('loads the work policy revisions', async () => {
		const policy = { version: 1, updatedAt: '', revisions: [] };
		const response = {
			policy,
			currentMonth: '2026-08',
			holidayDates: ['2026-08-17'],
			timeZone: 'Asia/Seoul'
		};
		globalThis.fetch = (async (input) => {
			expect(input).toBe('/admin/api/attendance-work-policy');
			return new Response(JSON.stringify(response));
		}) as typeof fetch;
		expect(await fetchAttendanceWorkPolicy('/admin/api', 'load failed')).toEqual(response);
		restoreGlobals();
	});

	test('saves only the current work policy revision', async () => {
		const revision = {
			effectiveDate: '',
			workMode: 'flexible' as const,
			workingWeekdays: [1, 2, 3, 4, 5],
			dailyTargetMinutes: 480,
			weeklyTargetMinutes: 2400,
			referenceStartTime: '09:00',
			fixedStartTime: '',
			fixedEndTime: '',
			coreTimeEnabled: true,
			coreStartTime: '11:00',
			coreEndTime: '16:00',
			breakPeriods: [{ startTime: '12:00', endTime: '13:00' }],
			nightStartTime: '22:00',
			nightEndTime: '06:00'
		};
		const policy = { version: 1, updatedAt: '', revisions: [revision] };
		const response = {
			policy,
			currentMonth: '2026-08',
			holidayDates: [],
			timeZone: 'Asia/Seoul'
		};
		globalThis.fetch = (async (input, init) => {
			expect(input).toBe('/admin/api/attendance-work-policy');
			expect(init?.method).toBe('PUT');
			expect(init?.body).toBe(JSON.stringify(revision));
			return new Response(JSON.stringify(response));
		}) as typeof fetch;
		expect(await updateAttendanceWorkPolicy('/admin/api', revision, 'save failed')).toEqual(response);
		restoreGlobals();
	});
});
