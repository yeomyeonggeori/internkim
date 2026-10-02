import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { dataRoomAccessCodePattern, dataRoomLinkInputSchema, dataRoomLinkLifetimeHours, dataRoomLinkSessionSchema, generateDataRoomCode } from '../../../src/lib/data-room/links';

describe('share link contract', () => {
	test('accepts existing PostgreSQL company identifiers when a guest session opens', () => {
		const session = {
			sessionID: '25c20378-a3a6-4fe2-a12f-1c2e83c3b430',
			companyID: '000000cc-0000-0000-0000-000000000001',
			expiresAt: '2026-10-02T06:00:00.123456+00:00'
		};
		expect(dataRoomLinkSessionSchema.parse(session)).toEqual(session);
		expect(dataRoomLinkSessionSchema.safeParse({ ...session, companyID: 'company' }).success).toBe(false);
	});
	test('defaults to three days and refuses unsupported or overlong lifetimes', () => {
		expect(dataRoomLinkInputSchema.parse({ label: 'Review', roleCode: 'investor' }).lifetimeHours).toBe(72);
		expect(dataRoomLinkInputSchema.safeParse({ label: 'Review', roleCode: 'investor', lifetimeHours: 169 }).success).toBe(false);
	});
	test('the database supports exactly the same lifetime choices', () => {
		const migration = readFileSync(new URL('../../../../supabase/migrations/20261003000012_data_room_links_and_member_roles.sql', import.meta.url), 'utf8');
		expect(migration).toContain(`lifetime_hours not in (${dataRoomLinkLifetimeHours.join(', ')})`);
		expect(migration).toContain(`access_code !~ '^${dataRoomAccessCodePattern}$'`);
	});
	test('generated codes are six digits including leading zeroes', () => {
		for (let count = 0; count < 30; count++) expect(generateDataRoomCode()).toMatch(/^[0-9]{6}$/);
	});
});
