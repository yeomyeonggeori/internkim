import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, test } from 'bun:test';

const copies = [
	['announce-attendance.ts', 'src/lib/server/announce-attendance.ts'],
	['announce-the-day.ts', 'src/lib/server/announce-the-day.ts'],
	['conversation-members.ts', 'src/lib/server/conversation-members.ts'],
	['day-digest-timing.ts', 'src/lib/notifications/day-digest-timing.ts'],
	['notify-member.ts', 'src/lib/server/notify-member.ts'],
	['push-to-member-devices.ts', 'src/lib/server/push-to-member-devices.ts']
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
