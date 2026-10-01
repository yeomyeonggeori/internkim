import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { declaredZone } from './fleet-domain-declaration';

// supabase/config.toml is Supabase's own file, read by the Supabase CLI, so it
// cannot import internal/fleetdomain's declaration. This test is the check
// that keeps its zone lines from drifting away from the one declaration.

function configTOML(): string {
	return readFileSync(new URL('../../../supabase/config.toml', import.meta.url), 'utf8');
}

describe('supabase/config.toml carries the same zone fleetdomain declares', () => {
	const zone = declaredZone();
	const config = configTOML();

	test('auth redirects to the declared zone', () => {
		expect(config).toContain(`site_url = "https://${zone}"`);
		expect(config).toContain(`additional_redirect_urls = ["https://${zone}/auth/claim", "https://${zone}/share/**", "http://localhost:*/share/**", "http://127.0.0.1:*/share/**"]`);
	});

	test('the webauthn relying party is the declared zone', () => {
		expect(config).toContain(`rp_id = "${zone}"`);
		expect(config).toContain(`rp_origins = ["https://${zone}"]`);
	});
});
