import { describe, expect, test } from 'bun:test';
import declarations from '../../../../tools/environment.json';
import { refusalOfPagesVariables, variablesRequiredOnPages } from '../../../scripts/pages-variables';

describe('the settings a production deploy needs on the Pages project', () => {
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
