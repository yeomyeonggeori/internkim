import { describe, expect, test } from 'bun:test';
import { createScopedBuzzIdentity, type BuzzIdentityScope } from '../../src/lib/scoped-buzz-identity';

const first = { projectURL: 'https://central.example.com', accountID: 'account-a', companyID: 'company-a', sessionKey: 'session-a' };

function scenario() {
	let scope: BuzzIdentityScope | null = first;
	let restored: string | null = null;
	let claims = 0;
	let claim: () => Promise<string | null> = async () => 'same-derived-secret';
	let visible: string | null = null;
	const published: string[] = [];
	const identity = createScopedBuzzIdentity({
		readScope: async () => scope,
		restore: () => restored,
		claim: async () => { claims += 1; return claim(); },
		publish: (_scope, secret) => { visible = secret; published.push(secret); },
		hide: () => { visible = null; }
	});
	return {
		identity, published,
		setScope: (next: BuzzIdentityScope | null) => { scope = next; },
		restore: (secret: string) => { restored = secret; },
		claimWith: (next: () => Promise<string | null>) => { claim = next; },
		claims: () => claims,
		visible: () => visible
	};
}

function deferred<Value>() {
	let resolve: (value: Value) => void = () => {};
	const promise = new Promise<Value>((complete) => { resolve = complete; });
	return { promise, resolve };
}

describe('deferred central Buzz identity', () => {
	test('does no host work until requested and shares concurrent requests', async () => {
		const work = scenario();
		expect(work.claims()).toBe(0);
		const one = work.identity.ensure();
		expect(work.identity.ensure()).toBe(one);
		expect(await one).toBe('same-derived-secret');
		expect(work.claims()).toBe(1);
		expect(work.published).toEqual(['same-derived-secret']);
	});

	test('restores a scope-verified tab key without contacting the host', async () => {
		const work = scenario();
		work.restore('existing-derived-secret');
		expect(await work.identity.ensure()).toBe('existing-derived-secret');
		expect(work.claims()).toBe(0);
	});

	for (const changed of [
		{ ...first, projectURL: 'https://other.example.com' },
		{ ...first, accountID: 'account-b' },
		{ ...first, companyID: 'company-b' },
		{ ...first, sessionKey: 'session-b' },
		null
	]) {
		test(`discards a late key after scope changes to ${JSON.stringify(changed)}`, async () => {
			const work = scenario();
			const answer = deferred<string | null>();
			work.claimWith(() => answer.promise);
			const pending = work.identity.ensure();
			await Promise.resolve();
			work.setScope(changed);
			answer.resolve('old-key');
			expect(await pending).toBeNull();
			expect(work.published).toEqual([]);
		});
	}

	test('invalidates an old generation even if the same account signs in again', async () => {
		const work = scenario();
		const answer = deferred<string | null>();
		work.claimWith(() => answer.promise);
		const old = work.identity.ensure();
		await Promise.resolve();
		work.identity.invalidate();
		work.claimWith(async () => 'current-key');
		expect(await work.identity.ensure()).toBe('current-key');
		answer.resolve('old-key');
		expect(await old).toBeNull();
		expect(work.visible()).toBe('current-key');
	});

	test('a refused or failed host claim leaves the rest of the session usable and can retry', async () => {
		const work = scenario();
		work.claimWith(async () => { throw new Error('host offline'); });
		expect(await work.identity.ensure()).toBeNull();
		work.claimWith(async () => 'same-derived-secret');
		expect(await work.identity.ensure()).toBe('same-derived-secret');
	});

	test('hides an active key before rechecking membership', async () => {
		const work = scenario();
		await work.identity.ensure();
		expect(work.visible()).toBe('same-derived-secret');
		work.setScope(null);
		const recheck = work.identity.ensure();
		expect(work.visible()).toBeNull();
		expect(await recheck).toBeNull();
	});
});
