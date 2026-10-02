import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { dataRoomLinkInputSchema, dataRoomLinkLifetimeHours, generateDataRoomCode } from '../../../src/lib/data-room/links';

describe('share link contract', () => {
	test('defaults to three days and refuses unsupported or overlong lifetimes', () => {
		expect(dataRoomLinkInputSchema.parse({ label: 'Review', roleCode: 'investor' }).lifetimeHours).toBe(72);
		expect(dataRoomLinkInputSchema.safeParse({ label: 'Review', roleCode: 'investor', lifetimeHours: 169 }).success).toBe(false);
	});
	test('the database supports exactly the same lifetime choices', () => {
		const migration = readFileSync(new URL('../../../../supabase/migrations/20261003000012_data_room_links_and_member_roles.sql', import.meta.url), 'utf8');
		expect(migration).toContain(`lifetime_hours not in (${dataRoomLinkLifetimeHours.join(', ')})`);
	});
	test('generated codes are six digits including leading zeroes', () => {
		for (let count = 0; count < 30; count++) expect(generateDataRoomCode()).toMatch(/^[0-9]{6}$/);
	});
});
