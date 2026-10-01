import { describe, expect, test } from 'bun:test';
import declarations from '../../../../tools/environment.json';
import {
	pagesVariablesFromVault,
	refusalOfPagesVariables,
	requiresPagesRuntimeVariables,
	variablesRequiredOnPages
} from '../../../scripts/pages-variables';

describe('the settings a production deploy needs on the Pages project', () => {
	test('only the static docs project skips company runtime variable checks', () => {
		expect(requiresPagesRuntimeVariables('internkim')).toBe(true);
		expect(requiresPagesRuntimeVariables('internkim-docs')).toBe(false);
		expect(requiresPagesRuntimeVariables('another-pages-project')).toBe(true);
	});

	test('the declaration file names the gateway token among them', () => {
		expect(variablesRequiredOnPages(declarations)).toContain('GATEWAY_ADMIN_TOKEN');
	});

	test('every one of them is read by the web app', () => {
		for (const name of variablesRequiredOnPages(declarations)) {
			const readers = declarations[name as keyof typeof declarations].process.split(', ');
			expect(readers).toContain('web');
		}
	});

	test('nothing is refused when every one is held as a secret', () => {
		expect(refusalOfPagesVariables(['GATEWAY_URL'], { GATEWAY_URL: { type: 'secret_text' } })).toBeNull();
	});

	test('every one of them is in the vault the script ships from', () => {
		for (const name of variablesRequiredOnPages(declarations)) {
			expect(declarations[name as keyof typeof declarations]).toHaveProperty('isInVault', true);
		}
	});

	test('the vault ships each one as a secret, and refuses to ship with one missing', () => {
		const held: Record<string, string> = { GATEWAY_URL: 'https://gateway.example.test' };
		expect(pagesVariablesFromVault(['GATEWAY_URL'], (name) => held[name] ?? '')).toEqual({
			GATEWAY_URL: { type: 'secret_text', value: 'https://gateway.example.test' }
		});
		expect(() => pagesVariablesFromVault(['GATEWAY_URL', 'VAPID_PUBLIC_KEY'], (name) => held[name] ?? '')).toThrow(
			'VAPID_PUBLIC_KEY not set'
		);
	});

	test('a missing setting is refused by name', () => {
		const refusal = refusalOfPagesVariables(['GATEWAY_ADMIN_TOKEN', 'GATEWAY_URL'], {
			GATEWAY_URL: { type: 'secret_text' }
		});
		expect(refusal).toContain('GATEWAY_ADMIN_TOKEN: not set');
		expect(refusal).not.toContain('GATEWAY_URL:');
	});

	test('a plain-text setting is refused, because a deploy drops it', () => {
		expect(refusalOfPagesVariables(['GATEWAY_URL'], { GATEWAY_URL: { type: 'plain_text' } })).toContain(
			'GATEWAY_URL: set as plain text'
		);
	});
});
