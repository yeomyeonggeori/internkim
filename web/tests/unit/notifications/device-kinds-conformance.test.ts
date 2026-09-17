import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { pushDeviceKindOfPlatform, pushDeviceKinds } from '../../../src/lib/notifications/reachability';

const migrationPath = new URL('../../../../supabase/migrations/20260808000002_push_device.sql', import.meta.url);

const senderPath = new URL('../../../../supabase/functions/_shared/push-to-member-devices.ts', import.meta.url);

function kindsTheRecordAccepts(): string[] {
	const migration = readFileSync(migrationPath, 'utf8');
	const constraint = migration.match(/kind text not null check \(kind in \(([^)]*)\)\)/);
	if (!constraint) throw new Error('the push_device migration declares no kind constraint');
	return [...constraint[1].matchAll(/'([^']+)'/g)].map((named) => named[1]);
}

function kindsTheSenderRoutes(): string[] {
	const sender = readFileSync(senderPath, 'utf8');
	return [...sender.matchAll(/device\.kind === '([^']+)'/g)].map((named) => named[1]);
}

describe('every copy of the device kinds says what the record does', () => {
	const recorded = kindsTheRecordAccepts();

	test('the record itself names three kinds', () => {
		expect(recorded).toEqual(['web-push', 'apns', 'fcm']);
	});

	test('the push device endpoint accepts exactly the kinds the record stores', () => {
		expect(pushDeviceKinds.map(String)).toEqual(recorded);
	});

	test('the sender routes every kind the record can hold', () => {
		expect(kindsTheSenderRoutes().sort()).toEqual([...recorded].sort());
	});

	test('each platform a shell runs on claims a kind the record stores', () => {
		expect(recorded).toContain(pushDeviceKindOfPlatform('ios'));
		expect(recorded).toContain(pushDeviceKindOfPlatform('android'));
		expect(pushDeviceKindOfPlatform('web')).toBe('web-push');
	});
});
