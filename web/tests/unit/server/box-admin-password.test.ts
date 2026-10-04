import { describe, expect, test } from 'bun:test';
import type { BoxRow } from '$lib/server/box';
import { adminPasswordOutcomeReportSchema, adminPasswordRequestSchema, pendingAdminPasswordOf } from '$lib/server/box-admin-password';

const sealed = {
	version: 1 as const,
	recipient: 'e06Qm75__kTEZaIgA31gjuNYl9Me-XLwf3SJLLD3PxM',
	enc: 'e06Qm75__kTEZaIgA31gjuNYl9Me-XLwf3SJLLD3PxM',
	ciphertext: 'c2VhbGVk'
};

const settingID = '00000000-0000-4000-8000-0000000000a1';

function aBox(adminPassword?: { settingID: string; sealed: typeof sealed; requestedAt: string }): BoxRow {
	return {
		companyID: 'company-a',
		publicKey: sealed.recipient,
		settings: { encryptionKey: sealed.recipient, hasAdminAccount: true, ...(adminPassword ? { adminPassword } : {}) }
	};
}

describe('the admin password a browser sends', () => {
	test('is accepted with a setting id and a sealed password', () => {
		expect(adminPasswordRequestSchema.safeParse({ settingID, sealed }).success).toBe(true);
	});

	test('is refused without a setting id, with an invalid one, or with anything extra', () => {
		expect(adminPasswordRequestSchema.safeParse({ sealed }).success).toBe(false);
		expect(adminPasswordRequestSchema.safeParse({ settingID: 'not-a-uuid', sealed }).success).toBe(false);
		expect(adminPasswordRequestSchema.safeParse({ settingID, sealed, password: 'plain' }).success).toBe(false);
	});
});

describe('the admin password outcome a box reports', () => {
	test('is accepted as applied or failed', () => {
		expect(adminPasswordOutcomeReportSchema.safeParse({ settingID, result: 'applied' }).success).toBe(true);
		expect(adminPasswordOutcomeReportSchema.safeParse({ settingID, result: 'failed' }).success).toBe(true);
	});

	test('is refused for a result the runtime does not know', () => {
		expect(adminPasswordOutcomeReportSchema.safeParse({ settingID, result: 'joined' }).success).toBe(false);
	});
});

describe('the admin password pending for a box', () => {
	test('is named when the box holds one', () => {
		expect(pendingAdminPasswordOf(aBox({ settingID, sealed, requestedAt: '2026-10-04T09:00:00.000Z' }))).toEqual({ settingID, sealed });
	});

	test('is null when the box holds none', () => {
		expect(pendingAdminPasswordOf(aBox())).toBeNull();
	});
});
