import { describe, expect, test } from 'bun:test';
import { CredentialCache } from './credential-cache';

function cacheReadingFrom(answers: Record<string, { kind: string; secret: string } | null>, clock: { now: number }) {
	const reads: string[] = [];
	const cache = new CredentialCache(async (memberID) => {
		reads.push(memberID);
		return answers[memberID] ?? null;
	}, () => clock.now);
	return { cache, reads };
}

describe('CredentialCache', () => {
	test('reads a member once and answers from what it kept', async () => {
		const clock = { now: 0 };
		const { cache, reads } = cacheReadingFrom({ 'member-1': { kind: 'buzz-secret', secret: 's1' } }, clock);

		expect(await cache.credentialOf('member-1')).toEqual({ kind: 'buzz-secret', secret: 's1' });
		clock.now = 5 * 60_000;
		expect(await cache.credentialOf('member-1')).toEqual({ kind: 'buzz-secret', secret: 's1' });
		expect(reads).toEqual(['member-1']);
	});

	test('reads that overlap share one trip', async () => {
		const clock = { now: 0 };
		const { cache, reads } = cacheReadingFrom({ 'member-1': { kind: 'buzz-secret', secret: 's1' } }, clock);

		await Promise.all([cache.credentialOf('member-1'), cache.credentialOf('member-1'), cache.credentialOf('member-1')]);
		expect(reads).toEqual(['member-1']);
	});

	test('reads again once what it kept has aged out', async () => {
		const clock = { now: 0 };
		const { cache, reads } = cacheReadingFrom({ 'member-1': { kind: 'buzz-secret', secret: 's1' } }, clock);

		await cache.credentialOf('member-1');
		clock.now = 10 * 60_000;
		await cache.credentialOf('member-1');
		expect(reads).toEqual(['member-1', 'member-1']);
	});

	test('keeps a missing credential only briefly, so a member who connects is seen soon', async () => {
		const clock = { now: 0 };
		const { cache, reads } = cacheReadingFrom({}, clock);

		expect(await cache.credentialOf('member-2')).toBeNull();
		clock.now = 10_000;
		await cache.credentialOf('member-2');
		clock.now = 31_000;
		await cache.credentialOf('member-2');
		expect(reads).toEqual(['member-2', 'member-2']);
	});

	test('a member acting is read again past a kept absence, so a credential issued moments ago is used', async () => {
		const clock = { now: 0 };
		const answers: Record<string, { kind: string; secret: string } | null> = {};
		const { cache, reads } = cacheReadingFrom(answers, clock);

		expect(await cache.credentialOf('member-2')).toBeNull();
		answers['member-2'] = { kind: 'buzz-secret', secret: 's2' };
		clock.now = 9_000;
		expect(await cache.credentialOf('member-2')).toBeNull();
		expect(await cache.credentialForAction('member-2')).toEqual({ kind: 'buzz-secret', secret: 's2' });
		expect(await cache.credentialOf('member-2')).toEqual({ kind: 'buzz-secret', secret: 's2' });
		expect(await cache.credentialForAction('member-2')).toEqual({ kind: 'buzz-secret', secret: 's2' });
		expect(reads).toEqual(['member-2', 'member-2']);
	});

	test('forgets one member or everyone on request', async () => {
		const clock = { now: 0 };
		const { cache, reads } = cacheReadingFrom(
			{ 'member-1': { kind: 'buzz-secret', secret: 's1' }, 'member-2': { kind: 'buzz-secret', secret: 's2' } },
			clock
		);

		await cache.credentialOf('member-1');
		await cache.credentialOf('member-2');
		cache.forget('member-1');
		await cache.credentialOf('member-1');
		await cache.credentialOf('member-2');
		cache.forgetEveryone();
		await cache.credentialOf('member-2');
		expect(reads).toEqual(['member-1', 'member-2', 'member-1', 'member-2']);
	});

	test('a read that fails is not kept', async () => {
		let attempts = 0;
		const cache = new CredentialCache(async () => {
			attempts += 1;
			if (attempts === 1) throw new Error('the central plane answered 502');
			return { kind: 'buzz-secret', secret: 's1' };
		});

		await expect(cache.credentialOf('member-1')).rejects.toThrow('502');
		expect(await cache.credentialOf('member-1')).toEqual({ kind: 'buzz-secret', secret: 's1' });
		expect(attempts).toBe(2);
	});
});
