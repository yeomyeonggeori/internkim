import { describe, expect, test } from 'bun:test';
import { carriesAnEmailedLink } from '../../src/routes/auth/claim/emailed-link';

describe('whether this page load arrived carrying an emailed credential', () => {
	test('sees the implicit flow, which puts the tokens in the fragment', () => {
		const fragment = '#access_token=abc&expires_in=3600&refresh_token=def&type=magiclink';

		expect(carriesAnEmailedLink(fragment, '')).toBe(true);
	});

	test('sees the pkce flow and a verification link, which put theirs in the query', () => {
		expect(carriesAnEmailedLink('', '?code=abc')).toBe(true);
		expect(carriesAnEmailedLink('', '?token_hash=abc&type=magiclink')).toBe(true);
	});

	test('does not see a plain visit, which is a session already open in the browser', () => {
		expect(carriesAnEmailedLink('', '')).toBe(false);
		expect(carriesAnEmailedLink('#', '')).toBe(false);
		expect(carriesAnEmailedLink('#section', '?return=/settings')).toBe(false);
	});

	test('does not take a lookalike name for the field itself', () => {
		expect(carriesAnEmailedLink('', '?access_token_hint=abc')).toBe(false);
		expect(carriesAnEmailedLink('#not_a_code=abc', '')).toBe(false);
	});
});
