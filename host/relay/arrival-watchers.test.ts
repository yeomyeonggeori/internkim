import { describe, expect, test } from 'bun:test';
import {
	ArrivalWatchers,
	arrivalsRenewalMilliseconds,
	arrivalsWatchCapability,
	keepWatchingArrivals,
	type ArrivalWatchersDependencies
} from './arrival-watchers';

const arrivalsURL = 'http://127.0.0.1:18091/arrived';
const typingURL = 'http://127.0.0.1:18091/typing';

type Asked = { capability: string; body: Record<string, unknown> };

function credentialOf(memberID: string) {
	return { kind: 'buzz-secret', secret: `secret-of-${memberID}` };
}

function dependencies(overrides: Partial<ArrivalWatchersDependencies> = {}): ArrivalWatchersDependencies {
	return {
		activeMemberIDs: async () => ['member-1'],
		credentialOf: async (memberID) => credentialOf(memberID),
		askChatd: async () => ({ status: 200, body: {} }),
		arrivalsURL,
		typingURL,
		report: () => {},
		now: () => 0,
		...overrides
	};
}

function watchedSecrets(asked: Asked[]): string[] {
	return asked
		.filter((one) => one.capability === arrivalsWatchCapability)
		.map((one) => (one.body.actor as { secret: string }).secret);
}

function chatdThatIsUpWhen(isUp: () => boolean, asked: Asked[] = []): ArrivalWatchersDependencies['askChatd'] {
	return async (capability, body) => {
		if (!isUp()) throw new Error('Unable to connect. Is the computer able to access the url?');
		asked.push({ capability, body });
		return { status: 200, body: {} };
	};
}

describe('renewing arrival watches', () => {
	test('asks chatd to watch every active member who holds a credential, telling it where to post', async () => {
		const asked: Asked[] = [];
		const watchers = new ArrivalWatchers(
			dependencies({
				activeMemberIDs: async () => ['member-1', 'member-2', 'member-3'],
				credentialOf: async (memberID) => (memberID === 'member-2' ? null : credentialOf(memberID)),
				askChatd: async (capability, body) => {
					asked.push({ capability, body });
					const actor = body.actor as { secret: string };
					return actor.secret === 'secret-of-member-3'
						? { status: 502, body: { error: 'the relay refused' } }
						: { status: 200, body: {} };
				}
			})
		);

		const renewal = await watchers.renewEveryone();

		expect(asked).toEqual([
			{ capability: arrivalsWatchCapability, body: { actor: credentialOf('member-1'), arrivalsURL, typingURL } },
			{ capability: arrivalsWatchCapability, body: { actor: credentialOf('member-3'), arrivalsURL, typingURL } }
		]);
		expect(renewal).toEqual({
			watched: 1,
			withoutCredential: 1,
			refusals: ['member member-3: chatd answered 502: {"error":"the relay refused"}']
		});
	});
});

describe('a member is watched from the moment the relay acts for them', () => {
	test('a member a renewal could not watch is watched at once, not at the next renewal', async () => {
		const asked: Asked[] = [];
		let isChatdUp = false;
		const watchers = new ArrivalWatchers(dependencies({ askChatd: chatdThatIsUpWhen(() => isChatdUp, asked) }));
		await expect(watchers.renewEveryone()).rejects.toThrow();

		isChatdUp = true;
		await watchers.watchOnceTheyAct('member-1', credentialOf('member-1'));

		expect(watchedSecrets(asked)).toEqual(['secret-of-member-1']);
	});

	test('a member watched within this renewal period is not asked for again', async () => {
		const asked: Asked[] = [];
		let now = 0;
		const watchers = new ArrivalWatchers(dependencies({ askChatd: chatdThatIsUpWhen(() => true, asked), now: () => now }));
		await watchers.renewEveryone();

		now += arrivalsRenewalMilliseconds - 1;
		await watchers.watchOnceTheyAct('member-1', credentialOf('member-1'));
		expect(watchedSecrets(asked)).toEqual(['secret-of-member-1']);

		now += 1;
		await watchers.watchOnceTheyAct('member-1', credentialOf('member-1'));
		expect(watchedSecrets(asked)).toEqual(['secret-of-member-1', 'secret-of-member-1']);
	});
});

describe('keeping arrival watches', () => {
	test('a renewal that could not reach chatd is tried again within a second, backing off to the renewal period', async () => {
		let isChatdUp = false;
		const watchers = new ArrivalWatchers(dependencies({ askChatd: chatdThatIsUpWhen(() => isChatdUp) }));
		const scheduled: { milliseconds: number; renew: () => Promise<void> }[] = [];

		await keepWatchingArrivals(watchers, (milliseconds, renew) => scheduled.push({ milliseconds, renew }));
		await scheduled[0].renew();
		isChatdUp = true;
		await scheduled[1].renew();

		expect(scheduled.map((next) => next.milliseconds)).toEqual([1_000, 2_000, arrivalsRenewalMilliseconds]);
	});
});
