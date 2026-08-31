import { describe, expect, test } from 'bun:test';

import { normalizeMailActorEmail, resolveMailActorEmail } from '../../../src/routes/mail/mail-request-actor';

describe('who a mail page is looking at the mailbox as', () => {
	test('an address is compared in one case, with no surrounding space', () => {
		expect(normalizeMailActorEmail(' Member@Example.COM ')).toBe('member@example.com');
	});

	test('the connected account answers first, then the address being typed into the draft', () => {
		expect(resolveMailActorEmail(' Account@Example.COM ', 'draft@example.com', '')).toBe('account@example.com');
		expect(resolveMailActorEmail('', ' Draft@Example.COM ', '')).toBe('draft@example.com');
	});

	test('a development address stands in when no account is connected and nothing is typed', () => {
		expect(resolveMailActorEmail('', '', ' Dev@Example.COM ')).toBe('dev@example.com');
		expect(resolveMailActorEmail('', '', '')).toBe('');
	});
});
