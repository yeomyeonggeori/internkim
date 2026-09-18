import { describe, expect, test } from 'bun:test';
import { readdirSync, readFileSync } from 'node:fs';
import { pushDeviceKindOfPlatform, pushDeviceKinds } from '../../../src/lib/notifications/reachability';

const migrationsPath = new URL('../../../../supabase/migrations/', import.meta.url);

const notificationSenderPath = new URL('../../../../supabase/functions/_shared/push-to-member-devices.ts', import.meta.url);

const activitySenderPath = new URL('../../../../supabase/functions/_shared/attendance-live-activity.ts', import.meta.url);

function kindsTheRecordAccepts(): string[] {
	const declarations = readdirSync(migrationsPath)
		.filter((name) => name.endsWith('.sql'))
		.sort()
		.map((name) => readFileSync(new URL(name, migrationsPath), 'utf8'))
		.filter((migration) => /push_device/.test(migration))
		.map((migration) => migration.match(/check \(kind in \(([^)]*)\)\)/))
		.filter((constraint): constraint is RegExpMatchArray => constraint !== null);
	const newest = declarations.at(-1);
	if (!newest) throw new Error('no migration declares the push_device kind constraint');
	return [...newest[1].matchAll(/'([^']+)'/g)].map((named) => named[1]);
}

function kindsTheSendersRoute(): string[] {
	const notifications = readFileSync(notificationSenderPath, 'utf8');
	const activities = readFileSync(activitySenderPath, 'utf8');
	return [
		...[...notifications.matchAll(/device\.kind === '([^']+)'/g)].map((named) => named[1]),
		...[...activities.matchAll(/export const activity\w*Kind = '([^']+)'/g)].map((named) => named[1])
	];
}

describe('every copy of the device kinds says what the record does', () => {
	const recorded = kindsTheRecordAccepts();

	test('the record itself names five kinds', () => {
		expect(recorded).toEqual(['web-push', 'apns', 'fcm', 'apns-activity-start', 'apns-activity']);
	});

	test('the push device endpoint accepts exactly the kinds the record stores', () => {
		expect(pushDeviceKinds.map(String)).toEqual(recorded);
	});

	test('some sender routes every kind the record can hold', () => {
		expect(kindsTheSendersRoute().sort()).toEqual([...recorded].sort());
	});

	test('each platform a shell runs on claims a kind the record stores', () => {
		expect(recorded).toContain(pushDeviceKindOfPlatform('ios'));
		expect(recorded).toContain(pushDeviceKindOfPlatform('android'));
		expect(pushDeviceKindOfPlatform('web')).toBe('web-push');
	});
});
