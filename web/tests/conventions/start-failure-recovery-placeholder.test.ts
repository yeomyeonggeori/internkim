import { readFileSync } from 'node:fs';
import { describe, expect, test } from 'bun:test';
import { startFailureRecoveryPlaceholder } from '$lib/server/start-failure-recovery-script';

describe('the start failure recovery placeholder', () => {
	const page = readFileSync('src/app.html', 'utf8');

	test('is in app.html for the server hook to fill', () => {
		expect(page).toContain(startFailureRecoveryPlaceholder);
	});

	test('comes before the scripts that start the app', () => {
		expect(page.indexOf(startFailureRecoveryPlaceholder)).toBeLessThan(page.indexOf('%sveltekit.body%'));
	});
});
