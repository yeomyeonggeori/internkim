import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { claimCodeLength } from '../../src/routes/auth/claim/claim-code';

function codeLengthInConfiguration(): number {
	const configuration = readFileSync(new URL('../../../supabase/config.toml', import.meta.url), 'utf8');
	const emailSection = configuration.split(/^\[auth\.email\]$/m)[1]?.split(/^\[/m)[0] ?? '';
	const declared = emailSection.match(/^otp_length\s*=\s*(\d+)$/m);
	if (!declared) throw new Error('supabase/config.toml declares no otp_length under [auth.email]');
	return Number(declared[1]);
}

describe('the emailed claim code', () => {
	test('gets a field as long as the config the mail is generated from says it is', () => {
		expect(claimCodeLength).toBe(codeLengthInConfiguration());
	});
});
