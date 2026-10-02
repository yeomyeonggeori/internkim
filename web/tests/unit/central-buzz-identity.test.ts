import { expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';

test('central identity verifies scope, persists only the tab key, and rejects late or revoked claims', async () => {
	const fixture = new URL('./central-buzz-identity.fixture.ts', import.meta.url).pathname;
	const run = Bun.spawn([process.execPath, fixture], { stdout: 'pipe', stderr: 'pipe' });
	const [output, errors, exitCode] = await Promise.all([
		new Response(run.stdout).text(), new Response(run.stderr).text(), run.exited
	]);
	expect(exitCode, errors).toBe(0);
	const result = JSON.parse(output);
	expect(result.initial).toBeNull();
	expect(result.first).toBe('same-derived-secret');
	expect(result.savedScope.accountID).toBe('account-a');
	expect(result.savedScope.companyID).toBe('company-a');
	expect(result.savedScope.sessionKey).toHaveLength(64);
	expect(result.storedToken).toBe(false);
	expect(result.hiddenWhileChecking).toBeNull();
	expect(result.restored).toBe('same-derived-secret');
	expect(result.restoreClaims).toBe(1);
	expect(result.changedCompany).toBe('company-b-key');
	expect(result.late).toBeNull();
	expect(result.afterSignout).toBeNull();
	expect(result.storedAfterSignout).toBe(false);
	expect(result.refused).toBeNull();
	expect(result.finalClaims).toBe(3);
	expect(result.getUserCalls).toBeGreaterThan(6);
	expect(result.memberCalls).toBeGreaterThan(5);
	expect(result.owners[0]).toMatchObject({ accountID: 'account-a', companyID: 'company-a', accessToken: 'sample-token-a' });
});

test('sign-in completes without a host claim while messenger and Buzz connect request identity on demand', () => {
	const read = (path: string) => readFileSync(new URL(`../../src/${path}`, import.meta.url), 'utf8');
	const gate = read('lib/components/web-auth-gate.svelte');
	expect(gate).not.toContain('claimCentralBuzzSecret');
	expect(gate).toContain('location.reload();');
	expect(gate).toContain("await import('$lib/buzz-key-login')");
	expect(read('routes/messenger/messenger.svelte')).toContain('onMount(keepCentralBuzzIdentity);');
	expect(read('lib/components/buzz/buzz-connect-dialog.svelte')).toContain('await ensureCentralBuzzIdentity();');
	expect(read('lib/supabase-session.ts')).toContain('forgetCentralBuzzIdentity();');
});
