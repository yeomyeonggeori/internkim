import { readFileSync } from 'node:fs';
import { describe, expect, test } from 'bun:test';
import { initialModePlaceholder } from '$lib/server/initial-mode-placeholder';

describe('the initial mode placeholder', () => {
	const page = readFileSync('src/app.html', 'utf8');

	test('is in app.html for the server hook to fill', () => {
		expect(page).toContain(initialModePlaceholder);
	});

	test('comes before the launch video picks its colours', () => {
		expect(page.indexOf(initialModePlaceholder)).toBeLessThan(page.indexOf('internkim-launch-video-played'));
	});
});
