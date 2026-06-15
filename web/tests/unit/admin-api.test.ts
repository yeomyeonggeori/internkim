import { describe, expect, test } from 'bun:test';
import { adminApiFetch } from '../../src/lib/admin-api';

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
