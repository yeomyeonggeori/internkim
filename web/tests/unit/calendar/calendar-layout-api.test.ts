import { describe, expect, test } from 'bun:test';
import { readFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';

const calendarRoot = join(import.meta.dir, '..', '..', '..', 'src', 'routes', 'calendar');

const retiredCalendarEndpoints = [
	'/calendar/api/remote-sync',
	'/calendar/api/account-status',
	'/calendar/api/google-calendars',
	'/calendar/api/google-oauth-client',
	'/calendar/api/conflicts',
	'/calendar/oauth/google'
];

function calendarSourceFiles(directory: string): string[] {
	return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
		const path = join(directory, entry.name);
		if (entry.isDirectory()) return calendarSourceFiles(path);
		return entry.name.endsWith('.ts') || entry.name.endsWith('.svelte') ? [path] : [];
	});
}

describe('the calendar route asks only for what admind still answers', () => {
	test('no calendar source names an endpoint the plane retired with Google Workspace', () => {
		const offenders: string[] = [];
		for (const path of calendarSourceFiles(calendarRoot)) {
			const source = readFileSync(path, 'utf8');
			for (const endpoint of retiredCalendarEndpoints) {
				if (source.includes(endpoint)) offenders.push(`${path} names ${endpoint}`);
			}
		}

		expect(
			offenders,
			`admind answers none of these, so a screen that calls one shows a permanent error and no other test sees it`
		).toEqual([]);
	});

	test('the subscription sheet is still fed by the endpoints that survive', () => {
		const source = readFileSync(join(calendarRoot, 'calendar-layout-api.ts'), 'utf8');

		expect(source).toContain('/calendar/api/sync');
		expect(source).toContain('/calendar/api/ics-token');
	});
});
