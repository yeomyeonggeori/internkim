import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, test } from 'bun:test';

const copies = [
	['announce-attendance.ts', 'src/lib/server/announce-attendance.ts'],
	['announce-the-day.ts', 'src/lib/server/announce-the-day.ts'],
	['day-digest-timing.ts', 'src/lib/notifications/day-digest-timing.ts'],
	['notify-member.ts', 'src/lib/server/notify-member.ts'],
	['push-to-member-devices.ts', 'src/lib/server/push-to-member-devices.ts'],
	['base64url.ts', 'src/lib/notifications/base64url.ts'],
	['web-push.ts', 'src/lib/server/web-push.ts'],
	['web-push-encrypt.ts', 'src/lib/server/web-push-encrypt.ts'],
	['web-push-vapid.ts', 'src/lib/server/web-push-vapid.ts'],
	['who-answers.ts', 'src/lib/server/who-answers.ts']
] as const;

function bodyOf(path: string): string {
	return readFileSync(path, 'utf8').replace(/^(?:import[^\n]*\n)+/, '');
}

function sharedBody(name: string): string {
	return bodyOf(join(import.meta.dir, '../../../../supabase/functions/_shared', name));
}

function webBody(path: string): string {
	return bodyOf(join(import.meta.dir, '../../..', path));
}

describe('the functions copies stay word for word the web ones', () => {
	for (const [name, path] of copies) {
		test(`${name} says what ${path} says`, () => {
			expect(sharedBody(name)).toContain('export');
			expect(sharedBody(name)).toBe(webBody(path));
		});
	}
});
