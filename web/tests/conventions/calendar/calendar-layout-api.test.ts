import { describe, expect, test } from 'bun:test';
import { readFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';

const calendarRoot = join(import.meta.dir, '..', '..', '..', 'src', 'routes', 'calendar');

function calendarSourceFiles(directory: string): string[] {
	return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
		const path = join(directory, entry.name);
		if (entry.isDirectory()) return calendarSourceFiles(path);
		return entry.name.endsWith('.ts') || entry.name.endsWith('.svelte') ? [path] : [];
	});
}

describe('the calendar screens read the company record and nothing else', () => {
	test('no calendar source names a device endpoint', () => {
		const offenders: string[] = [];
		for (const path of calendarSourceFiles(calendarRoot)) {
			const source = readFileSync(path, 'utf8');
			if (source.includes('/calendar/api')) offenders.push(path);
			if (source.includes('/calendar/dav')) offenders.push(path);
		}

		expect(
			offenders,
			'the calendar is kept by the company, so a screen reaching a device answers for one company only'
		).toEqual([]);
	});

	test('no calendar source branches on whether the company record is configured', () => {
		const offenders: string[] = [];
		for (const path of calendarSourceFiles(calendarRoot)) {
			if (readFileSync(path, 'utf8').includes('isSupabaseConfigured')) offenders.push(path);
		}

		expect(offenders, 'there is one calendar store, so there is nothing to branch on').toEqual([]);
	});
});
